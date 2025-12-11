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

func TestObservation_YAMLRoundTrip(t *testing.T) {
	obs := &Observation{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Observation",
		Metadata: metav1.ObjectMeta{
			Name:      "test-obs",
			Namespace: "default",
		},
		Spec: ObservationSpec{
			Source:    "trivy",
			Category:  "security",
			Severity:  "high",
			EventType: "vulnerability",
			Resource: &ResourceRef{
				Kind: "Pod",
				Name: "test-pod",
			},
		},
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(obs)
	if err != nil {
		t.Fatalf("Failed to marshal to YAML: %v", err)
	}

	// Unmarshal from YAML
	var obs2 Observation
	if err := yaml.Unmarshal(yamlData, &obs2); err != nil {
		t.Fatalf("Failed to unmarshal from YAML: %v", err)
	}

	// Verify key fields
	if obs2.Spec.Source != "trivy" {
		t.Errorf("Source = %v, want trivy", obs2.Spec.Source)
	}
	if obs2.Spec.Category != "security" {
		t.Errorf("Category = %v, want security", obs2.Spec.Category)
	}
}

func TestValidateObservation_Valid(t *testing.T) {
	obs := &Observation{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Observation",
		Spec: ObservationSpec{
			Source:    "trivy",
			Category:  "security",
			Severity:  "high",
			EventType: "vulnerability",
		},
	}
	if err := ValidateObservation(obs); err != nil {
		t.Errorf("ValidateObservation returned error for valid observation: %v", err)
	}
}

func TestValidateObservation_MissingSource(t *testing.T) {
	obs := &Observation{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Observation",
		Spec: ObservationSpec{
			Category:  "security",
			Severity:  "high",
			EventType: "vulnerability",
		},
	}
	err := ValidateObservation(obs)
	if err == nil {
		t.Error("ValidateObservation should return error for missing source")
	}
	if ve, ok := err.(*ValidationError); !ok || ve.Field != "spec.source" {
		t.Errorf("Expected ValidationError for spec.source, got: %v", err)
	}
}

func TestValidateObservation_InvalidCategory(t *testing.T) {
	obs := &Observation{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Observation",
		Spec: ObservationSpec{
			Source:    "test",
			Category:  "invalid",
			Severity:  "high",
			EventType: "vulnerability",
		},
	}
	err := ValidateObservation(obs)
	if err == nil {
		t.Error("ValidateObservation should return error for invalid category")
	}
}

func TestValidateObservation_InvalidTTL(t *testing.T) {
	ttl := int64(0)
	obs := &Observation{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Observation",
		Spec: ObservationSpec{
			Source:                 "test",
			Category:               "security",
			Severity:               "high",
			EventType:              "vulnerability",
			TTLSecondsAfterCreation: &ttl,
		},
	}
	err := ValidateObservation(obs)
	if err == nil {
		t.Error("ValidateObservation should return error for TTL < 1")
	}
}

