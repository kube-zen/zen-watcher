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

package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	sdkdedup "github.com/kube-zen/zen-sdk/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// setupTestEnv is defined in pipeline_test.go - using it here
var _ = setupTestEnv

// TestPipelineIntegration_FullFlow_FilterFirst tests complete pipeline with filter_first strategy
func TestPipelineIntegration_FullFlow_FilterFirst(t *testing.T) {
	dynamicClient := setupTestEnv(t)

	ctx := context.Background()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	// Configure filter to allow HIGH severity only
	filterConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"test-source": {
				MinSeverity: "HIGH", // Only HIGH and above
			},
		},
	}
	f := filter.NewFilter(filterConfig)
	deduper := sdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000
	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, // eventsTotal
		nil, // observationsCreated
		nil, // observationsFiltered
		nil, // observationsDeduped
		nil, // observationsCreateErrors
		f,   // filter
	)
	proc := processor.NewProcessor(f, deduper, creator)

	// Test 1: HIGH severity event should pass filter and create observation
	rawEvent1 := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"id":       "event-1",
		},
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "filter_first",
		},
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "test-event",
			Priority: map[string]float64{
				"HIGH": 0.8,
				"LOW":  0.3,
			},
		},
	}

	err := proc.ProcessEvent(ctx, rawEvent1, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err := dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	if len(list.Items) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(list.Items))
	}

	// Test 2: LOW severity event should be filtered out
	rawEvent2 := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "LOW",
			"id":       "event-2",
		},
	}

	err = proc.ProcessEvent(ctx, rawEvent2, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err = dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	// Should still be 1 (LOW event filtered out)
	if len(list.Items) != 1 {
		t.Errorf("Expected 1 observation after filtering LOW event, got %d", len(list.Items))
	}
}

// TestPipelineIntegration_FullFlow_DedupFirst tests complete pipeline with dedup_first strategy
func TestPipelineIntegration_FullFlow_DedupFirst(t *testing.T) {
	dynamicClient := setupTestEnv(t)

	ctx := context.Background()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	deduper := sdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000
	filterConfig := &filter.FilterConfig{Sources: make(map[string]filter.SourceFilter)}
	f := filter.NewFilter(filterConfig)
	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, // eventsTotal
		nil, // observationsCreated
		nil, // observationsFiltered
		nil, // observationsDeduped
		nil, // observationsCreateErrors
		f,   // filter
	)
	proc := processor.NewProcessor(f, deduper, creator)

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "dedup_first",
		},
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "test-event",
			Priority: map[string]float64{
				"HIGH": 0.8,
			},
		},
	}

	// Test 1: First event should create observation
	rawEvent1 := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"id":       "duplicate-id",
		},
	}

	err := proc.ProcessEvent(ctx, rawEvent1, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err := dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	if len(list.Items) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(list.Items))
	}

	// Test 2: Duplicate event should be deduplicated
	rawEvent2 := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now().Add(1 * time.Second),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"id":       "duplicate-id", // Same ID = duplicate
		},
	}

	err = proc.ProcessEvent(ctx, rawEvent2, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err = dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	// Should still be 1 (duplicate deduplicated)
	if len(list.Items) != 1 {
		t.Errorf("Expected 1 observation after deduplication, got %d", len(list.Items))
	}
}

// TestPipelineIntegration_FullFlow_FilterAndDedup tests pipeline with both filter and dedup
func TestPipelineIntegration_FullFlow_FilterAndDedup(t *testing.T) {
	dynamicClient := setupTestEnv(t)

	// Configure filter to allow HIGH severity only
	filterConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"test-source": {
				MinSeverity: "HIGH", // Only HIGH and above
			},
		},
	}
	f := filter.NewFilter(filterConfig)
	deduper := sdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000

	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}
	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, // eventsTotal
		nil, // observationsCreated
		nil, // observationsFiltered
		nil, // observationsDeduped
		nil, // observationsCreateErrors
		f,   // filter
	)
	proc := processor.NewProcessor(f, deduper, creator)

	ctx := context.Background()

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "filter_first",
		},
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "test-event",
			Priority: map[string]float64{
				"HIGH": 0.8,
				"LOW":  0.3,
			},
		},
	}

	// Test 1: HIGH severity event should pass filter and create observation
	rawEvent1 := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"id":       "unique-1",
		},
	}

	err := proc.ProcessEvent(ctx, rawEvent1, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err := dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	if len(list.Items) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(list.Items))
	}

	// Test 2: Duplicate HIGH event should be deduplicated
	rawEvent2 := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now().Add(1 * time.Second),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"id":       "unique-1", // Same ID = duplicate
		},
	}

	err = proc.ProcessEvent(ctx, rawEvent2, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err = dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	// Should still be 1 (duplicate deduplicated)
	if len(list.Items) != 1 {
		t.Errorf("Expected 1 observation after deduplication, got %d", len(list.Items))
	}

	// Test 3: LOW severity duplicate should be filtered (never reaches dedup)
	rawEvent3 := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now().Add(2 * time.Second),
		RawData: map[string]interface{}{
			"severity": "LOW",
			"id":       "unique-1", // Same ID but filtered before dedup
		},
	}

	err = proc.ProcessEvent(ctx, rawEvent3, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err = dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	// Should still be 1 (LOW event filtered out)
	if len(list.Items) != 1 {
		t.Errorf("Expected 1 observation after filtering LOW event, got %d", len(list.Items))
	}
}

// TestPipelineIntegration_Normalization tests that normalization happens after filter/dedup
func TestPipelineIntegration_Normalization(t *testing.T) {
	dynamicClient := setupTestEnv(t)
	proc := setupPipeline(t, dynamicClient)

	ctx := context.Background()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "vulnerability",
			Priority: map[string]float64{
				"HIGH":   0.8,
				"MEDIUM": 0.5,
			},
			FieldMapping: []generic.FieldMapping{
				{
					From: "vuln_id",
					To:   "cve_id",
				},
			},
		},
	}

	rawEvent := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"vuln_id":  "CVE-2024-001",
			"resource": map[string]interface{}{
				"kind":      "Pod",
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	err := proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	list, err := dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	if len(list.Items) == 0 {
		t.Fatal("Expected observation to be created")
	}

	obs := list.Items[0]
	spec, ok := obs.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("Observation spec is not a map")
	}

	// Verify normalization was applied
	if spec["category"] != "security" {
		t.Errorf("category = %v, want security", spec["category"])
	}

	if spec["eventType"] != "vulnerability" {
		t.Errorf("eventType = %v, want vulnerability", spec["eventType"])
	}

	// Verify field mapping was applied
	details, ok := spec["details"].(map[string]interface{})
	if !ok {
		t.Fatal("Observation details is not a map")
	}

	if details["cve_id"] != "CVE-2024-001" {
		t.Errorf("cve_id = %v, want CVE-2024-001", details["cve_id"])
	}
}

// TestPipelineIntegration_ErrorHandling tests error handling in pipeline
func TestPipelineIntegration_ErrorHandling(t *testing.T) {
	dynamicClient := setupTestEnv(t)
	proc := setupPipeline(t, dynamicClient)

	ctx := context.Background()

	// Test with nil source config (should handle gracefully)
	rawEvent := &generic.RawEvent{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
		},
	}

	// Process with nil config - should not crash
	err := proc.ProcessEvent(ctx, rawEvent, nil)
	// May return error or handle gracefully
	if err != nil {
		t.Logf("ProcessEvent() with nil config returned error (acceptable): %v", err)
	}

	// Test with empty source
	rawEvent2 := &generic.RawEvent{
		Source:    "",
		Timestamp: time.Now(),
		RawData:   map[string]interface{}{},
	}

	sourceConfig := &generic.SourceConfig{
		Source: "",
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "test",
		},
	}

	err = proc.ProcessEvent(ctx, rawEvent2, sourceConfig)
	// Should handle gracefully
	if err != nil {
		t.Logf("ProcessEvent() with empty source returned error (acceptable): %v", err)
	}
}
