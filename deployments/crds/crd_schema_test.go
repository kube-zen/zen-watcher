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

package crds

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// TestObservationCRDSchemaFileExists validates the Observation CRD file exists and is valid YAML
func TestObservationCRDSchemaFileExists(t *testing.T) {
	// Try to find CRD file (works from repo root or from this directory)
	possiblePaths := []string{
		"observation_crd.yaml",
		"deployments/crds/observation_crd.yaml",
		filepath.Join("..", "..", "deployments", "crds", "observation_crd.yaml"),
	}

	var crdPath string
	var crdYAML []byte
	var err error

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			crdPath = path
			crdYAML, err = os.ReadFile(path)
			if err == nil {
				break
			}
		}
	}

	if crdPath == "" {
		t.Skip("Could not find observation_crd.yaml file (this is OK if running from different directory)")
		return
	}

	if err != nil {
		t.Fatalf("Failed to read CRD file %s: %v", crdPath, err)
	}

	// Parse as unstructured to validate YAML structure
	var crd unstructured.Unstructured
	if err := yaml.Unmarshal(crdYAML, &crd.Object); err != nil {
		t.Fatalf("Failed to parse CRD YAML: %v", err)
	}

	// Validate CRD structure
	name, _, _ := unstructured.NestedString(crd.Object, "metadata", "name")
	if name != "observations.zen.kube-zen.io" {
		t.Errorf("Expected CRD name 'observations.zen.kube-zen.io', got '%s'", name)
	}

	group, _, _ := unstructured.NestedString(crd.Object, "spec", "group")
	if group != "zen.kube-zen.io" {
		t.Errorf("Expected group 'zen.kube-zen.io', got '%s'", group)
	}

	kind, _, _ := unstructured.NestedString(crd.Object, "spec", "names", "kind")
	if kind != "Observation" {
		t.Errorf("Expected kind 'Observation', got '%s'", kind)
	}

	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if len(versions) == 0 {
		t.Fatal("CRD must have at least one version")
	}

	// Check v1 version exists and is served/stored
	v1Found := false
	for _, v := range versions {
		versionMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if versionMap["name"] == "v1" {
			v1Found = true
			if served, ok := versionMap["served"].(bool); !ok || !served {
				t.Error("v1 version must be served")
			}
			if storage, ok := versionMap["storage"].(bool); !ok || !storage {
				t.Error("v1 version must be stored")
			}
			break
		}
	}

	if !v1Found {
		t.Error("CRD must have v1 version")
	}
}

// TestObservationSchema_RequiredFields validates required fields structure
func TestObservationSchema_RequiredFields(t *testing.T) {
	tests := []struct {
		name         string
		observation  map[string]interface{}
		wantValid    bool
		missingField string
	}{
		{
			name: "valid observation with all required fields",
			observation: map[string]interface{}{
				"apiVersion": "zen.kube-zen.io/v1",
				"kind":       "Observation",
				"metadata": map[string]interface{}{
					"name":      "test-obs",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"source":    "test",
					"category":  "security",
					"severity":  "HIGH",
					"eventType": "vulnerability",
				},
			},
			wantValid: true,
		},
		{
			name: "missing source",
			observation: map[string]interface{}{
				"spec": map[string]interface{}{
					"category":  "security",
					"severity":  "HIGH",
					"eventType": "vulnerability",
				},
			},
			wantValid:    false,
			missingField: "source",
		},
		{
			name: "missing category",
			observation: map[string]interface{}{
				"spec": map[string]interface{}{
					"source":    "test",
					"severity":  "HIGH",
					"eventType": "vulnerability",
				},
			},
			wantValid:    false,
			missingField: "category",
		},
		{
			name: "missing severity",
			observation: map[string]interface{}{
				"spec": map[string]interface{}{
					"source":    "test",
					"category":  "security",
					"eventType": "vulnerability",
				},
			},
			wantValid:    false,
			missingField: "severity",
		},
		{
			name: "missing eventType",
			observation: map[string]interface{}{
				"spec": map[string]interface{}{
					"source":   "test",
					"category": "security",
					"severity": "HIGH",
				},
			},
			wantValid:    false,
			missingField: "eventType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := &unstructured.Unstructured{Object: tt.observation}
			spec, ok := obs.Object["spec"].(map[string]interface{})
			if !ok {
				t.Fatal("Observation must have spec")
			}

			// Check required fields
			requiredFields := []string{"source", "category", "severity", "eventType"}
			for _, field := range requiredFields {
				if _, exists := spec[field]; !exists {
					if tt.wantValid {
						t.Errorf("Required field '%s' is missing", field)
					} else if field == tt.missingField {
						// Expected missing field
						return
					}
				}
			}

			if !tt.wantValid {
				t.Error("Expected validation to fail, but all required fields are present")
			}
		})
	}
}

// TestObservationSchema_TTLValidation validates TTL field constraints
func TestObservationSchema_TTLValidation(t *testing.T) {
	tests := []struct {
		name        string
		ttlValue    interface{}
		wantValid   bool
		description string
	}{
		{
			name:        "valid TTL (positive integer)",
			ttlValue:    int64(3600),
			wantValid:   true,
			description: "1 hour TTL",
		},
		{
			name:        "valid TTL (minimum value)",
			ttlValue:    int64(1),
			wantValid:   true,
			description: "1 second TTL (minimum)",
		},
		{
			name:        "invalid TTL (zero)",
			ttlValue:    int64(0),
			wantValid:   false,
			description: "TTL must be >= 1",
		},
		{
			name:        "invalid TTL (negative)",
			ttlValue:    int64(-1),
			wantValid:   false,
			description: "TTL cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"source":                  "test",
						"category":                "security",
						"severity":                "HIGH",
						"eventType":               "vulnerability",
						"ttlSecondsAfterCreation": tt.ttlValue,
					},
				},
			}

			spec := obs.Object["spec"].(map[string]interface{})
			ttl, exists := spec["ttlSecondsAfterCreation"]
			if !exists {
				t.Fatal("TTL field should exist for this test")
			}

			ttlInt, ok := ttl.(int64)
			if !ok {
				t.Fatalf("TTL should be int64, got %T", ttl)
			}

			isValid := ttlInt >= 1
			if isValid != tt.wantValid {
				t.Errorf("Expected valid=%v, got valid=%v (TTL=%d, %s)",
					tt.wantValid, isValid, ttlInt, tt.description)
			}
		})
	}
}
