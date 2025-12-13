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

package processor

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

// setupTestProcessor creates a processor with test dependencies
func setupTestProcessor(t *testing.T) (*Processor, dynamic.Interface) {
	t.Helper()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)

	filterConfig := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}
	f := filter.NewFilter(filterConfig)

	deduper := dedup.NewDeduper(60, 10000)
	defer deduper.Stop()

	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil, nil,
	)

	proc := NewProcessor(f, deduper, creator)
	return proc, dynamicClient
}

// TestPipelineStrategy_FingerprintDefault tests that default (fingerprint) strategy works
func TestPipelineStrategy_FingerprintDefault(t *testing.T) {
	proc, _ := setupTestProcessor(t)

	ctx := context.Background()
	rawEvent := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"resource": map[string]interface{}{
				"kind":      "Pod",
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	// Config without strategy should default to fingerprint
	config := &generic.SourceConfig{
		Source: "test-source",
		Dedup: &generic.DedupConfig{
			Enabled:  true,
			Window:   "60s",
			Strategy: "", // Empty should default to fingerprint
		},
	}

	// First event should process successfully
	err := proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("First event should process successfully: %v", err)
	}

	// Duplicate event should be deduplicated
	err = proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("Duplicate event should be deduplicated (no error): %v", err)
	}
}

// TestPipelineStrategy_FingerprintExplicit tests explicit fingerprint strategy
func TestPipelineStrategy_FingerprintExplicit(t *testing.T) {
	proc, _ := setupTestProcessor(t)

	ctx := context.Background()
	rawEvent := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"resource": map[string]interface{}{
				"kind":      "Pod",
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	config := &generic.SourceConfig{
		Source: "test-source",
		Dedup: &generic.DedupConfig{
			Enabled:  true,
			Window:   "60s",
			Strategy: "fingerprint",
		},
	}

	// First event should process successfully
	err := proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("First event should process successfully: %v", err)
	}

	// Duplicate event should be deduplicated
	err = proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("Duplicate event should be deduplicated (no error): %v", err)
	}
}

// TestPipelineStrategy_EventStream tests event-stream strategy
func TestPipelineStrategy_EventStream(t *testing.T) {
	proc, _ := setupTestProcessor(t)

	ctx := context.Background()
	rawEvent := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"resource": map[string]interface{}{
				"kind":      "Pod",
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	config := &generic.SourceConfig{
		Source: "test-source",
		Dedup: &generic.DedupConfig{
			Enabled:            true,
			Window:             "5s", // Shorter window for event-stream
			Strategy:           "event-stream",
			MaxEventsPerWindow: 10,
		},
	}

	// First event should process successfully
	err := proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("First event should process successfully: %v", err)
	}

	// Duplicate event should be deduplicated
	err = proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("Duplicate event should be deduplicated (no error): %v", err)
	}

	// Wait for window to expire
	time.Sleep(6 * time.Second)

	// After window expires, should create again
	err = proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("After window expires, should create again: %v", err)
	}
}

// TestPipelineStrategy_NoStrategyField tests backward compatibility (no strategy field)
func TestPipelineStrategy_NoStrategyField(t *testing.T) {
	proc, _ := setupTestProcessor(t)

	ctx := context.Background()
	rawEvent := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"resource": map[string]interface{}{
				"kind":      "Pod",
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	// Config without dedup config should still work (backward compatibility)
	config := &generic.SourceConfig{
		Source: "test-source",
		// No Dedup config - should default to fingerprint behavior
	}

	// First event should process successfully
	err := proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("First event should process successfully: %v", err)
	}
}

// TestPipelineStrategy_KeyBased tests key-based strategy
func TestPipelineStrategy_KeyBased(t *testing.T) {
	proc, _ := setupTestProcessor(t)

	ctx := context.Background()
	rawEvent := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"resource": map[string]interface{}{
				"kind":      "Pod",
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	config := &generic.SourceConfig{
		Source: "test-source",
		Dedup: &generic.DedupConfig{
			Enabled:  true,
			Window:   "60s",
			Strategy: "key",
			Fields:   []string{"source", "kind", "name"},
		},
	}

	// First event should process successfully
	err := proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("First event should process successfully: %v", err)
	}

	// Duplicate event should be deduplicated
	err = proc.ProcessEvent(ctx, rawEvent, config)
	if err != nil {
		t.Errorf("Duplicate event should be deduplicated (no error): %v", err)
	}
}
