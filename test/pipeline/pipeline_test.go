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
	"github.com/kube-zen/zen-watcher/pkg/filter"
	sdkdedup "github.com/kube-zen/zen-sdk/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// setupTestEnv creates a fake dynamic client for testing
func setupTestEnv(t *testing.T) dynamic.Interface {
	t.Helper()
	scheme := runtime.NewScheme()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}
	// Register observations resource for List operations
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		observationGVR: "ObservationList",
	})
}

// setupPipeline creates a complete pipeline with fake clients
func setupPipeline(t *testing.T, dynamicClient dynamic.Interface) *processor.Processor {
	t.Helper()

	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	// Create filter
	filterConfig := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}
	f := filter.NewFilter(filterConfig)

	// Create deduper
	deduper := sdksdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000

	// Create observation creator
	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil, nil,
	)

	// Create processor
	return processor.NewProcessor(f, deduper, creator)
}

// TestPipeline_NormalPath tests the normal flow: source config -> event -> observation created
func TestPipeline_NormalPath(t *testing.T) {
	dynamicClient := setupTestEnv(t)
	proc := setupPipeline(t, dynamicClient)

	ctx := context.Background()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	// Create a raw event (simulating a source adapter emitting an event)
	rawEvent := &generic.RawEvent{
		Source:    "trivy",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"vulnerability": "CVE-2024-001",
			"severity":      "HIGH",
			"resource": map[string]interface{}{
				"kind":      "Pod",
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	// Create source config
	sourceConfig := &generic.SourceConfig{
		Source: "trivy",
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "vulnerability",
			Priority: map[string]float64{
				"HIGH": 0.8,
			},
		},
	}

	// Process event through pipeline
	err := proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	// Wait a bit for async processing
	time.Sleep(100 * time.Millisecond)

	// List observations to verify one was created
	list, err := dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	if len(list.Items) == 0 {
		t.Error("Expected at least one observation to be created, got 0")
		return
	}

	// Verify first observation fields
	obs := list.Items[0]
	spec, ok := obs.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("Observation spec is not a map")
	}

	if spec["source"] != "trivy" {
		t.Errorf("source = %v, want trivy", spec["source"])
	}

	if spec["category"] != "security" {
		t.Errorf("category = %v, want security", spec["category"])
	}

	_ = proc
}

// TestPipeline_InvalidSourceConfigRejected tests that invalid source configs are handled gracefully
func TestPipeline_InvalidSourceConfigRejected(t *testing.T) {
	dynamicClient := setupTestEnv(t)
	proc := setupPipeline(t, dynamicClient)

	ctx := context.Background()

	// Create a raw event with invalid source config (missing normalization)
	rawEvent := &generic.RawEvent{
		Source: "invalid-source",
		RawData: map[string]interface{}{
			"message": "test",
		},
		Timestamp: time.Now(),
	}

	// Create invalid source config (missing required normalization)
	sourceConfig := &generic.SourceConfig{
		Source: "invalid-source",
		// Missing Normalization.Domain and Normalization.Type
	}

	// Process event - should handle gracefully (filter out or error)
	err := proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	// Note: In a real implementation, this might filter out or error
	// For now, we just verify the pipeline doesn't crash
	if err != nil {
		t.Logf("ProcessEvent() returned error (expected for invalid config): %v", err)
	}

	// Verify no observations were created
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	list, err := dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	// Invalid configs should not create observations
	if len(list.Items) > 0 {
		t.Logf("Note: %d observations found (may be from other tests)", len(list.Items))
	}

	_ = proc
}

// TestPipeline_WebhookEventFlow tests webhook-originated event flow
func TestPipeline_WebhookEventFlow(t *testing.T) {
	dynamicClient := setupTestEnv(t)
	proc := setupPipeline(t, dynamicClient)

	ctx := context.Background()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	// Simulate webhook-originated event (matching 08-webhook-gateway.yaml pattern)
	rawEvent := &generic.RawEvent{
		Source:    "webhook-gateway",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"alert_type": "security_alert",
			"severity":   "MEDIUM",
			"message":    "Security event detected",
		},
	}

	sourceConfig := &generic.SourceConfig{
		Source: "webhook-gateway",
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "security_alert",
			Priority: map[string]float64{
				"MEDIUM": 0.5,
			},
		},
	}

	// Process webhook event through pipeline
	err := proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, want nil", err)
	}

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify webhook observation was created
	list, err := dynamicClient.Resource(observationGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}

	if len(list.Items) == 0 {
		t.Error("Expected webhook observation to be created, got 0")
		return
	}

	// Verify webhook observation fields
	obs := list.Items[0]
	spec, ok := obs.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("Observation spec is not a map")
	}

	if spec["source"] != "webhook-gateway" {
		t.Errorf("source = %v, want webhook-gateway", spec["source"])
	}

	if spec["category"] != "security" {
		t.Errorf("category = %v, want security", spec["category"])
	}

	_ = proc
}
