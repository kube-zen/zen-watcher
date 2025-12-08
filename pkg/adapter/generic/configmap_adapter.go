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
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapAdapter handles ALL ConfigMap-based sources (kube-bench, checkov, etc.)
type ConfigMapAdapter struct {
	clientSet kubernetes.Interface
	events    chan RawEvent
	lastSeen  map[string]string // ConfigMap key -> last seen data hash
	mu        sync.RWMutex
}

// NewConfigMapAdapter creates a new generic ConfigMap adapter
func NewConfigMapAdapter(clientSet kubernetes.Interface) *ConfigMapAdapter {
	return &ConfigMapAdapter{
		clientSet: clientSet,
		events:    make(chan RawEvent, 100),
		lastSeen:  make(map[string]string),
	}
}

// Type returns the adapter type
func (a *ConfigMapAdapter) Type() string {
	return "configmap"
}

// Validate validates the configmap configuration
func (a *ConfigMapAdapter) Validate(config *SourceConfig) error {
	if config.ConfigMap == nil {
		return fmt.Errorf("configmap config is required for configmap adapter")
	}
	if config.ConfigMap.LabelSelector == "" {
		return fmt.Errorf("configmap.labelSelector is required")
	}
	return nil
}

// Start starts the ConfigMap adapter
func (a *ConfigMapAdapter) Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}

	// Parse poll interval
	pollInterval, err := time.ParseDuration(config.ConfigMap.PollInterval)
	if err != nil {
		pollInterval = 5 * time.Minute
	}

	// Start polling
	go a.pollConfigMaps(ctx, config, pollInterval)

	logger.Info("ConfigMap adapter started",
		logger.Fields{
			Component: "adapter",
			Operation: "configmap_start",
			Source:    config.Source,
			Additional: map[string]interface{}{
				"namespace":     config.ConfigMap.Namespace,
				"labelSelector": config.ConfigMap.LabelSelector,
				"pollInterval":  pollInterval.String(),
			},
		})

	return a.events, nil
}

// pollConfigMaps periodically polls ConfigMaps and emits events on changes
func (a *ConfigMapAdapter) pollConfigMaps(ctx context.Context, config *SourceConfig, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Initial poll
	a.doPoll(ctx, config)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.doPoll(ctx, config)
		}
	}
}

// doPoll performs a single poll of ConfigMaps
func (a *ConfigMapAdapter) doPoll(ctx context.Context, config *SourceConfig) {
	listOpts := metav1.ListOptions{
		LabelSelector: config.ConfigMap.LabelSelector,
	}

	var configMaps *corev1.ConfigMapList
	var err error
	if config.ConfigMap.Namespace != "" {
		configMaps, err = a.clientSet.CoreV1().ConfigMaps(config.ConfigMap.Namespace).List(ctx, listOpts)
	} else {
		configMaps, err = a.clientSet.CoreV1().ConfigMaps("").List(ctx, listOpts)
	}

	if err != nil {
		logger.Debug("Failed to list ConfigMaps",
			logger.Fields{
				Component: "adapter",
				Operation: "configmap_list",
				Source:    config.Source,
				Error:     err,
			})
		return
	}

	for _, cm := range configMaps.Items {
		cmKey := fmt.Sprintf("%s/%s", cm.Namespace, cm.Name)

		// Extract data
		var data interface{} = cm.Data
		if config.ConfigMap.JSONPath != "" {
			// Apply JSONPath if specified
			// Simplified - would use jsonpath library in production
			data = cm.Data
		}

		// Serialize to JSON for comparison
		dataJSON, err := json.Marshal(data)
		if err != nil {
			continue
		}
		hash := sha256.Sum256(dataJSON)
		dataHash := fmt.Sprintf("%x", hash)

		// Check if changed
		a.mu.RLock()
		lastHash, exists := a.lastSeen[cmKey]
		a.mu.RUnlock()

		if !exists || lastHash != dataHash {
			// ConfigMap changed or first time seeing it
			a.mu.Lock()
			a.lastSeen[cmKey] = dataHash
			a.mu.Unlock()

			// Create raw event
			event := RawEvent{
				Source:    config.Source,
				Timestamp: time.Now(),
				RawData: map[string]interface{}{
					"configMap": map[string]interface{}{
						"name":       cm.Name,
						"namespace":  cm.Namespace,
						"data":       cm.Data,
						"binaryData": cm.BinaryData,
					},
					"extracted": data,
				},
				Metadata: map[string]interface{}{
					"event": "change",
					"first": !exists,
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

// Stop stops the ConfigMap adapter
func (a *ConfigMapAdapter) Stop() {
	close(a.events)
}
