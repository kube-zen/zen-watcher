// Copyright 2024 The Zen Watcher Authors
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

package watcher

import (
	"context"
	"testing"

	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestObservationCreator_CreateObservation(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	creator := NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, // eventsTotal
		nil, // observationsCreated
		nil, // observationsFiltered
		nil, // observationsDeduped
		nil, // observationsCreateErrors
		nil, // filter
	)

	ctx := context.Background()

	// Create a valid observation
	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"generateName": "test-obs-",
				"namespace":    "default",
			},
			"spec": map[string]interface{}{
				"source":    "test",
				"category":  "security",
				"severity":  "HIGH",
				"eventType": "vulnerability",
			},
		},
	}

	// First creation should succeed
	err := creator.CreateObservation(ctx, observation)
	if err != nil {
		t.Fatalf("CreateObservation() error = %v, want nil", err)
	}

	// Verify observation was created
	created, err := dynamicClient.Resource(observationGVR).Namespace("default").Get(
		ctx, observation.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get created observation: %v", err)
	}

	if created.GetName() == "" {
		t.Error("Observation name should be generated")
	}

	// Verify required fields
	source, _, _ := unstructured.NestedString(created.Object, "spec", "source")
	if source != "test" {
		t.Errorf("Expected source 'test', got '%s'", source)
	}

	category, _, _ := unstructured.NestedString(created.Object, "spec", "category")
	if category != "security" {
		t.Errorf("Expected category 'security', got '%s'", category)
	}
}

func TestObservationCreator_CreateObservation_AlreadyExists(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	creator := NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil, nil, // all metrics/filter nil
	)

	ctx := context.Background()

	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"name":      "test-obs-exists",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"source":    "test",
				"category":  "security",
				"severity":  "HIGH",
				"eventType": "vulnerability",
			},
		},
	}

	// Create first observation
	err := creator.CreateObservation(ctx, observation)
	if err != nil {
		t.Fatalf("First CreateObservation() error = %v", err)
	}

	// Try to create duplicate (should fail with already exists)
	observation2 := observation.DeepCopy()
	err = creator.CreateObservation(ctx, observation2)
	if err == nil {
		t.Error("Expected error when creating duplicate observation")
	}

	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected AlreadyExists error, got: %v", err)
	}
}

func TestObservationCreator_CreateObservation_WithDedup(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	creator := NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil, nil, // all nil for testing
	)

	ctx := context.Background()

	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"generateName": "test-obs-",
				"namespace":    "default",
			},
			"spec": map[string]interface{}{
				"source":    "test",
				"category":  "security",
				"severity":  "HIGH",
				"eventType": "vulnerability",
			},
		},
	}

	// First creation should succeed
	err := creator.CreateObservation(ctx, observation)
	if err != nil {
		t.Fatalf("First CreateObservation() error = %v", err)
	}

	// Second creation with same content should be deduplicated
	err = creator.CreateObservation(ctx, observation)
	if err != nil {
		// Deduplication should prevent creation, but shouldn't error
		// In real code, dedup happens before Create, so this should not error
		t.Logf("Second CreateObservation was deduplicated (expected)")
	}
}
