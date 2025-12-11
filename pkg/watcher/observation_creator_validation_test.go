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

package watcher

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// TestObservationCreator_ValidEnumValues tests that valid enum values are accepted
func TestObservationCreator_ValidEnumValues(t *testing.T) {
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
		nil, nil, nil, nil, nil, nil,
	)

	ctx := context.Background()

	// Test valid severity values (lowercase, as per enum)
	testCases := []struct {
		name     string
		category string
		severity string
		valid    bool
	}{
		{"valid-security-critical", "security", "critical", true},
		{"valid-security-high", "security", "high", true},
		{"valid-compliance-medium", "compliance", "medium", true},
		{"valid-performance-low", "performance", "low", true},
		{"valid-operations-info", "operations", "info", true},
		{"valid-cost-high", "cost", "high", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
						"category":  tc.category,
						"severity":  tc.severity,
						"eventType": "test-event",
					},
				},
			}

			err := creator.CreateObservation(ctx, observation)
			if tc.valid && err != nil {
				t.Errorf("Expected valid observation, got error: %v", err)
			}
			// Note: Invalid enum values would be caught by CRD validation,
			// not by our code. This test verifies our code handles valid values.
		})
	}
}

// TestObservationCreator_ValidTTLRange tests that TTL values within valid range are accepted
func TestObservationCreator_ValidTTLRange(t *testing.T) {
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
		nil, nil, nil, nil, nil, nil,
	)

	ctx := context.Background()

	// Test valid TTL values
	testCases := []struct {
		name string
		ttl  int64
		valid bool
	}{
		{"ttl-minimum", 1, true},
		{"ttl-1-hour", 3600, true},
		{"ttl-1-day", 86400, true},
		{"ttl-1-week", 604800, true},
		{"ttl-1-year", 31536000, true},
		{"ttl-maximum", 31536000, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "zen.kube-zen.io/v1",
					"kind":       "Observation",
					"metadata": map[string]interface{}{
						"generateName": "test-obs-",
						"namespace":    "default",
					},
					"spec": map[string]interface{}{
						"source":                "test",
						"category":              "security",
						"severity":              "high",
						"eventType":             "test-event",
						"ttlSecondsAfterCreation": tc.ttl,
					},
				},
			}

			err := creator.CreateObservation(ctx, observation)
			if tc.valid && err != nil {
				t.Errorf("Expected valid observation with TTL %d, got error: %v", tc.ttl, err)
			}
			// Note: Invalid TTL values (too high) would be caught by CRD validation,
			// not by our code. This test verifies our code handles valid values.
		})
	}
}

// TestObservationCreator_CanonicalSample validates that canonical sample Observations still work
func TestObservationCreator_CanonicalSample(t *testing.T) {
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
		nil, nil, nil, nil, nil, nil,
	)

	ctx := context.Background()

	// Canonical sample from tests (matches existing test patterns)
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
				"severity":  "high",  // lowercase (will be normalized to HIGH by code)
				"eventType": "vulnerability",
			},
		},
	}

	// This should succeed - validates backward compatibility
	err := creator.CreateObservation(ctx, observation)
	if err != nil {
		t.Fatalf("Canonical sample observation failed: %v", err)
	}

	// Verify it was created
	created, err := dynamicClient.Resource(observationGVR).Namespace("default").Get(
		ctx, observation.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get created observation: %v", err)
	}

	// Verify required fields are present
	source, _, _ := unstructured.NestedString(created.Object, "spec", "source")
	if source != "test" {
		t.Errorf("Expected source 'test', got '%s'", source)
	}

	category, _, _ := unstructured.NestedString(created.Object, "spec", "category")
	if category != "security" {
		t.Errorf("Expected category 'security', got '%s'", category)
	}
}
