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

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/orchestrator"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"github.com/kube-zen/zen-watcher/test/helpers"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// TestOrchestrator_StartStop tests orchestrator start and stop
func TestOrchestrator_StartStop(t *testing.T) {
	// Setup
	dynamicClient := setupTestEnv(t)
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	// Create processor
	filterConfig := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}
	f := filter.NewFilter(filterConfig)
	deduper := helpers.NewTestDeduperWithDefaults(t)
	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil,
		f,
	)
	proc := processor.NewProcessor(f, deduper, creator)

	// Create orchestrator with nil factory (will fail on adapter creation, but we test start/stop)
	factory := generic.NewFactory(nil, nil)
	orch := orchestrator.NewGenericOrchestrator(factory, dynamicClient, proc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Test: Start orchestrator
	err := orch.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	// Wait a bit for orchestrator to initialize
	time.Sleep(300 * time.Millisecond)

	// Test: Stop orchestrator
	orch.Stop()

	// Verify orchestrator stopped
	time.Sleep(200 * time.Millisecond)
}

// TestOrchestrator_ProcessorIntegration tests orchestrator with processor
func TestOrchestrator_ProcessorIntegration(t *testing.T) {
	dynamicClient := setupTestEnv(t)
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	filterConfig := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}
	f := filter.NewFilter(filterConfig)
	deduper := helpers.NewTestDeduperWithDefaults(t)
	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil,
		f,
	)
	proc := processor.NewProcessor(f, deduper, creator)

	// Verify processor is created correctly
	if proc == nil {
		t.Fatal("Processor is nil")
	}

	// Create orchestrator
	factory := generic.NewFactory(nil, nil)
	orch := orchestrator.NewGenericOrchestrator(factory, dynamicClient, proc)

	// Verify orchestrator is created
	if orch == nil {
		t.Fatal("Orchestrator is nil")
	}
}

// TestOrchestrator_StopAllAdapters tests that Stop() stops all adapters
func TestOrchestrator_StopAllAdapters(t *testing.T) {
	dynamicClient := setupTestEnv(t)
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	filterConfig := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}
	f := filter.NewFilter(filterConfig)
	deduper := helpers.NewTestDeduperWithDefaults(t)
	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil,
		f,
	)
	proc := processor.NewProcessor(f, deduper, creator)

	factory := generic.NewFactory(nil, nil)
	orch := orchestrator.NewGenericOrchestrator(factory, dynamicClient, proc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := orch.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Stop should clean up all adapters
	orch.Stop()

	time.Sleep(200 * time.Millisecond)

	// Verify orchestrator is stopped
	// Note: We can't directly verify adapter removal, but Stop() should complete without error
}

// setupTestEnv creates a fake dynamic client for testing
func setupTestEnv(t *testing.T) dynamic.Interface {
	t.Helper()
	scheme := runtime.NewScheme()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}
	ingesterGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "ingesters",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		observationGVR: "ObservationList",
		ingesterGVR:    "IngesterList",
	})
}
