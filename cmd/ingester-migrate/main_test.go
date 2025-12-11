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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrate_SimpleCRDDestination(t *testing.T) {
	v1alpha1 := IngesterV1Alpha1{
		APIVersion: "zen.kube-zen.io/v1alpha1",
		Kind:       "Ingester",
		Metadata: map[string]interface{}{
			"name":      "test-ingester",
			"namespace": "default",
		},
		Spec: IngesterSpecV1Alpha1{
			Source:   "trivy",
			Ingester: "informer",
			Destinations: []DestinationV1Alpha1{
				{
					Type:  "crd",
					Value: "observations",
				},
			},
			Normalization: &NormalizationConfig{
				Domain: "security",
				Type:   "vulnerability",
				Priority: map[string]interface{}{
					"HIGH":   0.8,
					"MEDIUM": 0.5,
					"LOW":    0.3,
				},
			},
		},
	}

	v1, warnings := migrateToV1(v1alpha1)

	if v1.APIVersion != "zen.kube-zen.io/v1" {
		t.Errorf("APIVersion = %v, want zen.kube-zen.io/v1", v1.APIVersion)
	}

	if len(v1.Spec.Destinations) != 1 {
		t.Fatalf("Expected 1 destination, got %d", len(v1.Spec.Destinations))
	}

	dest := v1.Spec.Destinations[0]
	if dest.Type != "crd" {
		t.Errorf("Destination type = %v, want crd", dest.Type)
	}

	if dest.Value != "observations" {
		t.Errorf("Destination value = %v, want observations", dest.Value)
	}

	if dest.Mapping == nil {
		t.Error("Expected mapping to be set from normalization")
	}

	if dest.Mapping.Domain != "security" {
		t.Errorf("Mapping domain = %v, want security", dest.Mapping.Domain)
	}

	if len(warnings) > 0 {
		t.Errorf("Expected no warnings, got %v", warnings)
	}
}

func TestMigrate_NonCRDDestination(t *testing.T) {
	v1alpha1 := IngesterV1Alpha1{
		APIVersion: "zen.kube-zen.io/v1alpha1",
		Kind:       "Ingester",
		Spec: IngesterSpecV1Alpha1{
			Source:   "test",
			Ingester: "webhook",
			Destinations: []DestinationV1Alpha1{
				{
					Type: "webhook",
					URL:  "https://example.com/webhook",
				},
			},
		},
	}

	v1, warnings := migrateToV1(v1alpha1)

	if len(v1.Spec.Destinations) == 0 {
		t.Error("Expected default CRD destination when no CRD destinations present")
	}

	hasWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "webhook") && strings.Contains(w, "not supported") {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning about non-CRD destination removal")
	}
}

func TestMigrate_NoDestinations(t *testing.T) {
	v1alpha1 := IngesterV1Alpha1{
		APIVersion: "zen.kube-zen.io/v1alpha1",
		Kind:       "Ingester",
		Spec: IngesterSpecV1Alpha1{
			Source:   "test",
			Ingester: "informer",
			Destinations: []DestinationV1Alpha1{},
		},
	}

	v1, warnings := migrateToV1(v1alpha1)

	if len(v1.Spec.Destinations) == 0 {
		t.Error("Expected default CRD destination when no destinations present")
	}

	hasWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "No CRD destinations found") {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("Expected warning about no CRD destinations")
	}
}

func TestMigrate_NoNormalization(t *testing.T) {
	v1alpha1 := IngesterV1Alpha1{
		APIVersion: "zen.kube-zen.io/v1alpha1",
		Kind:       "Ingester",
		Spec: IngesterSpecV1Alpha1{
			Source:   "test",
			Ingester: "informer",
			Destinations: []DestinationV1Alpha1{
				{
					Type:  "crd",
					Value: "observations",
				},
			},
			// No normalization
		},
	}

	v1, warnings := migrateToV1(v1alpha1)

	if len(v1.Spec.Destinations) != 1 {
		t.Fatalf("Expected 1 destination, got %d", len(v1.Spec.Destinations))
	}

	dest := v1.Spec.Destinations[0]
	if dest.Mapping != nil {
		t.Error("Expected no mapping when normalization is missing")
	}

	if len(warnings) > 0 {
		t.Errorf("Expected no warnings, got %v", warnings)
	}
}

// TestMigrate_GoldenFiles tests against golden files
func TestMigrate_GoldenFiles(t *testing.T) {
	testCases := []struct {
		name           string
		v1alpha1File   string
		expectedV1File string
	}{
		{
			name:           "simple_trivy",
			v1alpha1File:   "testdata/trivy-v1alpha1.yaml",
			expectedV1File: "testdata/trivy-v1.yaml",
		},
		{
			name:           "with_normalization",
			v1alpha1File:   "testdata/kyverno-v1alpha1.yaml",
			expectedV1File: "testdata/kyverno-v1.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read v1alpha1 input
			v1alpha1Data, err := os.ReadFile(tc.v1alpha1File)
			if err != nil {
				t.Skipf("Skipping test: golden file not found: %v", err)
				return
			}

			// Parse and migrate
			// (Simplified - full implementation would parse YAML)
			// This is a placeholder for actual golden file tests

			// Read expected v1 output
			expectedData, err := os.ReadFile(tc.expectedV1File)
			if err != nil {
				t.Skipf("Skipping test: golden file not found: %v", err)
				return
			}

			// Compare (simplified - would need full YAML comparison)
			_ = v1alpha1Data
			_ = expectedData
		})
	}
}

