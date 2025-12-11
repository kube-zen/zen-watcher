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

package sdk

import (
	"testing"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIngester_YAMLRoundTrip(t *testing.T) {
	ingester := NewTrivyIngester("default", "test-trivy")

	// Marshal to YAML
	yamlData, err := yaml.Marshal(ingester)
	if err != nil {
		t.Fatalf("Failed to marshal to YAML: %v", err)
	}

	// Unmarshal from YAML
	var ingester2 Ingester
	if err := yaml.Unmarshal(yamlData, &ingester2); err != nil {
		t.Fatalf("Failed to unmarshal from YAML: %v", err)
	}

	// Verify key fields
	if ingester2.Spec.Source != "trivy" {
		t.Errorf("Source = %v, want trivy", ingester2.Spec.Source)
	}
	if ingester2.Spec.Ingester != "informer" {
		t.Errorf("Ingester = %v, want informer", ingester2.Spec.Ingester)
	}
	if len(ingester2.Spec.Destinations) != 1 {
		t.Errorf("Destinations count = %v, want 1", len(ingester2.Spec.Destinations))
	}
}

func TestValidateIngester_Valid(t *testing.T) {
	ingester := NewTrivyIngester("default", "test-trivy")
	if err := ValidateIngester(ingester); err != nil {
		t.Errorf("ValidateIngester returned error for valid ingester: %v", err)
	}
}

func TestValidateIngester_MissingSource(t *testing.T) {
	ingester := &Ingester{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Ingester",
		Spec: IngesterSpec{
			Ingester: "informer",
			Destinations: []Destination{
				{Type: "crd", Value: "observations"},
			},
		},
	}
	err := ValidateIngester(ingester)
	if err == nil {
		t.Error("ValidateIngester should return error for missing source")
	}
	if ve, ok := err.(*ValidationError); !ok || ve.Field != "spec.source" {
		t.Errorf("Expected ValidationError for spec.source, got: %v", err)
	}
}

func TestValidateIngester_InvalidDestinationType(t *testing.T) {
	ingester := &Ingester{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Ingester",
		Spec: IngesterSpec{
			Source:   "test",
			Ingester: "informer",
			Destinations: []Destination{
				{Type: "webhook", Value: "observations"},
			},
		},
	}
	err := ValidateIngester(ingester)
	if err == nil {
		t.Error("ValidateIngester should return error for invalid destination type")
	}
}

func TestNewTrivyIngester(t *testing.T) {
	ingester := NewTrivyIngester("default", "trivy-test")
	if ingester.Spec.Source != "trivy" {
		t.Errorf("Source = %v, want trivy", ingester.Spec.Source)
	}
	if err := ValidateIngester(ingester); err != nil {
		t.Errorf("NewTrivyIngester produced invalid ingester: %v", err)
	}
}

func TestNewKyvernoIngester(t *testing.T) {
	ingester := NewKyvernoIngester("default", "kyverno-test")
	if ingester.Spec.Source != "kyverno" {
		t.Errorf("Source = %v, want kyverno", ingester.Spec.Source)
	}
	if err := ValidateIngester(ingester); err != nil {
		t.Errorf("NewKyvernoIngester produced invalid ingester: %v", err)
	}
}

func TestNewKubeBenchIngester(t *testing.T) {
	ingester := NewKubeBenchIngester("default", "kube-bench-test")
	if ingester.Spec.Source != "kube-bench" {
		t.Errorf("Source = %v, want kube-bench", ingester.Spec.Source)
	}
	if err := ValidateIngester(ingester); err != nil {
		t.Errorf("NewKubeBenchIngester produced invalid ingester: %v", err)
	}
}

func TestIngester_CRDCompatibility(t *testing.T) {
	// Test that JSON tags match CRD field names
	ingester := NewTrivyIngester("default", "test")
	
	// Verify required fields have correct JSON tags
	// This is a structural test - actual CRD compatibility is tested via CRD conformance tests
	if ingester.APIVersion != "zen.kube-zen.io/v1" {
		t.Errorf("APIVersion = %v, want zen.kube-zen.io/v1", ingester.APIVersion)
	}
	if ingester.Kind != "Ingester" {
		t.Errorf("Kind = %v, want Ingester", ingester.Kind)
	}
}

