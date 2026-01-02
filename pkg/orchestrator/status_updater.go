// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// SourceState represents the state of a source
type SourceState string

const (
	SourceStateRunning SourceState = "Running"
	SourceStateStopped SourceState = "Stopped"
	SourceStateError   SourceState = "Error"
)

// SourceStatusTracker tracks the status of sources for an Ingester
type SourceStatusTracker struct {
	mu       sync.RWMutex
	sources  map[string]*SourceStatusInfo // key: source name
	ingester types.NamespacedName
}

// SourceStatusInfo holds status information for a source
type SourceStatusInfo struct {
	Name      string
	Type      string
	State     SourceState
	LastError string
	LastSeen  *metav1.Time
}

// NewSourceStatusTracker creates a new source status tracker
func NewSourceStatusTracker(namespace, name string) *SourceStatusTracker {
	return &SourceStatusTracker{
		sources:  make(map[string]*SourceStatusInfo),
		ingester: types.NamespacedName{Namespace: namespace, Name: name},
	}
}

// UpdateSourceState updates the state of a source
func (sst *SourceStatusTracker) UpdateSourceState(sourceName, sourceType string, state SourceState, err error) {
	sst.mu.Lock()
	defer sst.mu.Unlock()

	info, exists := sst.sources[sourceName]
	if !exists {
		info = &SourceStatusInfo{
			Name: sourceName,
			Type: sourceType,
		}
		sst.sources[sourceName] = info
	}

	info.State = state
	info.Type = sourceType
	if err != nil {
		info.LastError = err.Error()
	} else {
		info.LastError = ""
	}
}

// UpdateSourceLastSeen updates the last seen timestamp for a source
func (sst *SourceStatusTracker) UpdateSourceLastSeen(sourceName string) {
	sst.mu.Lock()
	defer sst.mu.Unlock()

	info, exists := sst.sources[sourceName]
	if !exists {
		return
	}

	now := metav1.Now()
	info.LastSeen = &now
}

// GetStatus returns the current status for all sources
func (sst *SourceStatusTracker) GetStatus() []SourceStatusInfo {
	sst.mu.RLock()
	defer sst.mu.RUnlock()

	result := make([]SourceStatusInfo, 0, len(sst.sources))
	for _, info := range sst.sources {
		result = append(result, *info)
	}
	return result
}

// IngesterStatusUpdater updates Ingester CRD status with source information
type IngesterStatusUpdater struct {
	dynClient dynamic.Interface
	trackers  map[types.NamespacedName]*SourceStatusTracker
	mu        sync.RWMutex
}

// NewIngesterStatusUpdater creates a new status updater
func NewIngesterStatusUpdater(dynClient dynamic.Interface) *IngesterStatusUpdater {
	return &IngesterStatusUpdater{
		dynClient: dynClient,
		trackers:  make(map[types.NamespacedName]*SourceStatusTracker),
	}
}

// GetOrCreateTracker gets or creates a status tracker for an Ingester
func (isu *IngesterStatusUpdater) GetOrCreateTracker(namespace, name string) *SourceStatusTracker {
	isu.mu.Lock()
	defer isu.mu.Unlock()

	nn := types.NamespacedName{Namespace: namespace, Name: name}
	tracker, exists := isu.trackers[nn]
	if !exists {
		tracker = NewSourceStatusTracker(namespace, name)
		isu.trackers[nn] = tracker
	}
	return tracker
}

// RemoveTracker removes a status tracker
func (isu *IngesterStatusUpdater) RemoveTracker(namespace, name string) {
	isu.mu.Lock()
	defer isu.mu.Unlock()

	nn := types.NamespacedName{Namespace: namespace, Name: name}
	delete(isu.trackers, nn)
}

// UpdateStatus updates the Ingester CRD status
func (isu *IngesterStatusUpdater) UpdateStatus(ctx context.Context, namespace, name string) error {
	isu.mu.RLock()
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	tracker, exists := isu.trackers[nn]
	isu.mu.RUnlock()

	if !exists {
		return nil // No tracker, nothing to update
	}

	// Get current Ingester CRD
	ingesterResource := isu.dynClient.Resource(config.IngesterGVR).Namespace(namespace)
	current, err := ingesterResource.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Ingester CRD: %w", err)
	}

	// Build status
	sourceStatuses := tracker.GetStatus()
	statusSources := make([]interface{}, 0, len(sourceStatuses))
	for _, ss := range sourceStatuses {
		sourceMap := map[string]interface{}{
			"name":  ss.Name,
			"type":  ss.Type,
			"state": string(ss.State),
		}
		if ss.LastError != "" {
			sourceMap["lastError"] = ss.LastError
		}
		if ss.LastSeen != nil {
			sourceMap["lastSeen"] = ss.LastSeen.Format(time.RFC3339)
		}
		statusSources = append(statusSources, sourceMap)
	}

	// Determine overall Ready condition
	ready := true
	readyReason := "AllSourcesRunning"
	readyMessage := "All sources are running"
	for _, ss := range sourceStatuses {
		if ss.State == SourceStateError {
			ready = false
			readyReason = "SourceError"
			readyMessage = fmt.Sprintf("Source %s has error: %s", ss.Name, ss.LastError)
			break
		} else if ss.State == SourceStateStopped {
			ready = false
			readyReason = "SourceStopped"
			readyMessage = fmt.Sprintf("Source %s is stopped", ss.Name)
		}
	}

	// Build conditions
	conditions := []interface{}{
		map[string]interface{}{
			"type":               "Ready",
			"status":             mapConditionStatus(ready),
			"reason":              readyReason,
			"message":            readyMessage,
			"lastTransitionTime": metav1.Now().Format(time.RFC3339),
		},
	}

	// Update status - use status subresource only
	status := map[string]interface{}{
		"sources":    statusSources,
		"conditions": conditions,
	}

	// Set only the status field (preserve metadata for UpdateStatus)
	current.Object["status"] = status

	// Use UpdateStatus to update via status subresource only (D027: status integrity)
	_, err = ingesterResource.UpdateStatus(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update Ingester status: %w", err)
	}

	orchestratorLogger.Debug("Updated Ingester status",
		sdklog.Operation("update_status"),
		sdklog.String("namespace", namespace),
		sdklog.String("name", name),
		sdklog.Int("source_count", len(sourceStatuses)),
		sdklog.Bool("ready", ready),
		sdklog.String("ready_reason", readyReason))

	return nil
}

// mapConditionStatus maps boolean to condition status string
func mapConditionStatus(ready bool) string {
	if ready {
		return "True"
	}
	return "False"
}

