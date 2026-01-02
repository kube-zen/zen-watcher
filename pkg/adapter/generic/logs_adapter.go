// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may Obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generic

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LogsAdapter handles ALL log-based sources (sealed-secrets, falco, etc.)
type LogsAdapter struct {
	clientSet kubernetes.Interface
	events    chan RawEvent
	watchers  map[string]context.CancelFunc // pod key -> cancel func
	mu        sync.RWMutex
}

// CompiledPattern represents a compiled regex pattern
type CompiledPattern struct {
	Regex    *regexp.Regexp
	Type     string
	Priority float64
}

// NewLogsAdapter creates a new generic logs adapter
func NewLogsAdapter(clientSet kubernetes.Interface) *LogsAdapter {
	return &LogsAdapter{
		clientSet: clientSet,
		events:    make(chan RawEvent, 100),
		watchers:  make(map[string]context.CancelFunc),
	}
}

// Type returns the adapter type
func (a *LogsAdapter) Type() string {
	return "logs"
}

// Validate validates the logs configuration
func (a *LogsAdapter) Validate(config *SourceConfig) error {
	if config.Logs == nil {
		return fmt.Errorf("logs config is required for logs adapter")
	}
	if config.Logs.PodSelector == "" {
		return fmt.Errorf("logs.podSelector is required")
	}
	if len(config.Logs.Patterns) == 0 {
		return fmt.Errorf("logs.patterns must have at least one pattern")
	}
	return nil
}

// Start starts the logs adapter
func (a *LogsAdapter) Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}

	// Compile patterns
	compiledPatterns := make([]CompiledPattern, 0, len(config.Logs.Patterns))
	for _, pattern := range config.Logs.Patterns {
		re, err := regexp.Compile(pattern.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern.Regex, err)
		}
		compiledPatterns = append(compiledPatterns, CompiledPattern{
			Regex:    re,
			Type:     pattern.Type,
			Priority: pattern.Priority,
		})
	}

	// Parse poll interval
	pollInterval, err := time.ParseDuration(config.Logs.PollInterval)
	if err != nil {
		pollInterval = 1 * time.Second
	}

	// Start pod watcher
	go a.watchPods(ctx, config, compiledPatterns, pollInterval)

	logger := sdklog.NewLogger("zen-watcher-adapter")
	logger.Info("Logs adapter started",
		sdklog.Operation("logs_start"),
		sdklog.String("source", config.Source),
		sdklog.String("pod_selector", config.Logs.PodSelector),
		sdklog.Int("patterns", len(compiledPatterns)),
		sdklog.Duration("poll_interval", pollInterval))

	return a.events, nil
}

// watchPods watches for pods matching the selector and streams their logs
func (a *LogsAdapter) watchPods(ctx context.Context, config *SourceConfig, patterns []CompiledPattern, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// List pods with selector
			pods, err := a.clientSet.CoreV1().Pods("").List(ctx, metav1.ListOptions{
				LabelSelector: config.Logs.PodSelector,
			})
			if err != nil {
				logger := sdklog.NewLogger("zen-watcher-adapter")
				logger.Debug("Failed to list pods",
					sdklog.Operation("logs_list_pods"),
					sdklog.String("source", config.Source),
					sdklog.Error(err))
				continue
			}

			// Track active pods
			activePods := make(map[string]bool)
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					continue
				}
				podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
				activePods[podKey] = true

				// Start log stream if not already running
				a.mu.RLock()
				_, exists := a.watchers[podKey]
				a.mu.RUnlock()

				if !exists {
					streamCtx, cancel := context.WithCancel(ctx)
					a.mu.Lock()
					a.watchers[podKey] = cancel
					a.mu.Unlock()

					go a.streamPodLogs(streamCtx, pod, config, patterns)
				}
			}

			// Stop watchers for pods that no longer exist
			a.mu.Lock()
			for podKey, cancel := range a.watchers {
				if !activePods[podKey] {
					cancel()
					delete(a.watchers, podKey)
				}
			}
			a.mu.Unlock()
		}
	}
}

// streamPodLogs streams logs from a pod and matches patterns
func (a *LogsAdapter) streamPodLogs(ctx context.Context, pod corev1.Pod, config *SourceConfig, patterns []CompiledPattern) {
	container := config.Logs.Container
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}

	opts := &corev1.PodLogOptions{
		Container:    container,
		Follow:       true,
		SinceSeconds: &[]int64{int64(config.Logs.SinceSeconds)}[0],
	}

	req := a.clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		logger := sdklog.NewLogger("zen-watcher-adapter")
		logger.Debug("Failed to stream pod logs",
			sdklog.Operation("logs_stream"),
			sdklog.String("source", config.Source),
			sdklog.String("namespace", pod.Namespace),
			sdklog.String("pod", pod.Name),
			sdklog.String("container", container),
			sdklog.Error(err))
		return
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		// Match against all patterns
		for _, pattern := range patterns {
			matches := pattern.Regex.FindStringSubmatch(line)
			if matches != nil {
				// Extract named groups
				// Optimized: pre-allocate map with estimated capacity (typically 2-5 groups)
				namedGroups := make(map[string]string, 4)
				for i, name := range pattern.Regex.SubexpNames() {
					if i > 0 && name != "" && i < len(matches) {
						namedGroups[name] = matches[i]
					}
				}

				// Create raw event
				event := RawEvent{
					Source:    config.Source,
					Timestamp: time.Now(),
					RawData: map[string]interface{}{
						"logLine":   line,
						"matches":   namedGroups,
						"pod":       pod.Name,
						"namespace": pod.Namespace,
						"container": container,
					},
					Metadata: map[string]interface{}{
						"type":     pattern.Type,
						"priority": pattern.Priority,
					},
				}

				select {
				case a.events <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logger := sdklog.NewLogger("zen-watcher-adapter")
		logger.Debug("Log scanner error",
			sdklog.Operation("logs_scanner"),
			sdklog.String("source", config.Source),
			sdklog.Error(err))
	}
}

// Stop stops the logs adapter
func (a *LogsAdapter) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, cancel := range a.watchers {
		cancel()
	}
	a.watchers = make(map[string]context.CancelFunc)
	close(a.events)
}
