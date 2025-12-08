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

package filter

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFilter_Allow_MinSeverity(t *testing.T) {
	tests := []struct {
		name           string
		filterConfig   *FilterConfig
		observation    *unstructured.Unstructured
		expectedAllow  bool
		expectedReason string
	}{
		{
			name: "Trivy HIGH severity passes MEDIUM minimum",
			filterConfig: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {
						MinSeverity: "MEDIUM",
					},
				},
			},
			observation:   createObservation("trivy", "security", "HIGH", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name: "Trivy LOW severity fails MEDIUM minimum",
			filterConfig: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {
						MinSeverity: "MEDIUM",
					},
				},
			},
			observation:   createObservation("trivy", "security", "LOW", "default", "Pod", "test-pod"),
			expectedAllow: false,
		},
		{
			name: "Trivy CRITICAL severity passes HIGH minimum",
			filterConfig: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {
						MinSeverity: "HIGH",
					},
				},
			},
			observation:   createObservation("trivy", "security", "CRITICAL", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name: "Trivy MEDIUM severity fails HIGH minimum",
			filterConfig: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {
						MinSeverity: "HIGH",
					},
				},
			},
			observation:   createObservation("trivy", "security", "MEDIUM", "default", "Pod", "test-pod"),
			expectedAllow: false,
		},
		{
			name: "No filter configured allows all",
			filterConfig: &FilterConfig{
				Sources: make(map[string]SourceFilter),
			},
			observation:   createObservation("trivy", "security", "LOW", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name:          "Nil filter config allows all",
			filterConfig:  nil,
			observation:   createObservation("trivy", "security", "LOW", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_Allow_ExcludeEventTypes(t *testing.T) {
	filterConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"kyverno": {
				ExcludeEventTypes: []string{"audit", "info"},
			},
		},
	}

	tests := []struct {
		name          string
		observation   *unstructured.Unstructured
		expectedAllow bool
	}{
		{
			name:          "Policy violation allowed",
			observation:   createObservationWithEventType("kyverno", "security", "MEDIUM", "default", "Pod", "test-pod", "policy-violation"),
			expectedAllow: true,
		},
		{
			name:          "Audit event excluded",
			observation:   createObservationWithEventType("kyverno", "security", "MEDIUM", "default", "Pod", "test-pod", "audit"),
			expectedAllow: false,
		},
		{
			name:          "Info event excluded",
			observation:   createObservationWithEventType("kyverno", "security", "MEDIUM", "default", "Pod", "test-pod", "info"),
			expectedAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_Allow_IncludeEventTypes(t *testing.T) {
	filterConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"audit": {
				IncludeEventTypes: []string{"resource-deletion", "secret-access"},
			},
		},
	}

	tests := []struct {
		name          string
		observation   *unstructured.Unstructured
		expectedAllow bool
	}{
		{
			name:          "Resource deletion allowed",
			observation:   createObservationWithEventType("audit", "compliance", "HIGH", "default", "Pod", "test-pod", "resource-deletion"),
			expectedAllow: true,
		},
		{
			name:          "Secret access allowed",
			observation:   createObservationWithEventType("audit", "compliance", "HIGH", "default", "Secret", "test-secret", "secret-access"),
			expectedAllow: true,
		},
		{
			name:          "Other event type excluded",
			observation:   createObservationWithEventType("audit", "compliance", "HIGH", "default", "Pod", "test-pod", "audit-event"),
			expectedAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_Allow_ExcludeNamespaces(t *testing.T) {
	filterConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				ExcludeNamespaces: []string{"kube-system", "kube-public"},
			},
		},
	}

	tests := []struct {
		name          string
		observation   *unstructured.Unstructured
		expectedAllow bool
	}{
		{
			name:          "Default namespace allowed",
			observation:   createObservation("trivy", "security", "HIGH", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name:          "Kube-system excluded",
			observation:   createObservation("trivy", "security", "HIGH", "kube-system", "Pod", "test-pod"),
			expectedAllow: false,
		},
		{
			name:          "Kube-public excluded",
			observation:   createObservation("trivy", "security", "HIGH", "kube-public", "Pod", "test-pod"),
			expectedAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_Allow_IncludeNamespaces(t *testing.T) {
	filterConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"falco": {
				IncludeNamespaces: []string{"production", "staging"},
			},
		},
	}

	tests := []struct {
		name          string
		observation   *unstructured.Unstructured
		expectedAllow bool
	}{
		{
			name:          "Production namespace allowed",
			observation:   createObservation("falco", "security", "HIGH", "production", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name:          "Staging namespace allowed",
			observation:   createObservation("falco", "security", "HIGH", "staging", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name:          "Default namespace excluded",
			observation:   createObservation("falco", "security", "HIGH", "default", "Pod", "test-pod"),
			expectedAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_Allow_ExcludeKinds(t *testing.T) {
	filterConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"kyverno": {
				ExcludeKinds: []string{"ConfigMap", "Secret"},
			},
		},
	}

	tests := []struct {
		name          string
		observation   *unstructured.Unstructured
		expectedAllow bool
	}{
		{
			name:          "Pod allowed",
			observation:   createObservation("kyverno", "security", "MEDIUM", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name:          "ConfigMap excluded",
			observation:   createObservation("kyverno", "security", "MEDIUM", "default", "ConfigMap", "test-cm"),
			expectedAllow: false,
		},
		{
			name:          "Secret excluded",
			observation:   createObservation("kyverno", "security", "MEDIUM", "default", "Secret", "test-secret"),
			expectedAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_Allow_ExcludeCategories(t *testing.T) {
	filterConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"kube-bench": {
				ExcludeCategories: []string{"compliance"},
			},
		},
	}

	tests := []struct {
		name          string
		observation   *unstructured.Unstructured
		expectedAllow bool
	}{
		{
			name:          "Security category allowed",
			observation:   createObservation("kube-bench", "security", "MEDIUM", "default", "Node", "test-node"),
			expectedAllow: true,
		},
		{
			name:          "Compliance category excluded",
			observation:   createObservation("kube-bench", "compliance", "MEDIUM", "default", "Node", "test-node"),
			expectedAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_Allow_Enabled(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name          string
		filterConfig  *FilterConfig
		observation   *unstructured.Unstructured
		expectedAllow bool
	}{
		{
			name: "Source enabled allows",
			filterConfig: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {
						Enabled: &enabled,
					},
				},
			},
			observation:   createObservation("trivy", "security", "HIGH", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
		{
			name: "Source disabled filters out",
			filterConfig: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {
						Enabled: &disabled,
					},
				},
			},
			observation:   createObservation("trivy", "security", "HIGH", "default", "Pod", "test-pod"),
			expectedAllow: false,
		},
		{
			name: "Source not configured defaults to enabled",
			filterConfig: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {},
				},
			},
			observation:   createObservation("trivy", "security", "HIGH", "default", "Pod", "test-pod"),
			expectedAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.filterConfig)
			result := f.Allow(tt.observation)
			if result != tt.expectedAllow {
				t.Errorf("Allow() = %v, want %v", result, tt.expectedAllow)
			}
		})
	}
}

func TestFilter_meetsMinSeverity(t *testing.T) {
	f := &Filter{}

	tests := []struct {
		name        string
		severity    string
		minSeverity string
		expected    bool
	}{
		{"CRITICAL meets CRITICAL", "CRITICAL", "CRITICAL", true},
		{"CRITICAL meets HIGH", "CRITICAL", "HIGH", true},
		{"CRITICAL meets MEDIUM", "CRITICAL", "MEDIUM", true},
		{"HIGH meets HIGH", "HIGH", "HIGH", true},
		{"HIGH meets MEDIUM", "HIGH", "MEDIUM", true},
		{"HIGH fails CRITICAL", "HIGH", "CRITICAL", false},
		{"MEDIUM meets MEDIUM", "MEDIUM", "MEDIUM", true},
		{"MEDIUM fails HIGH", "MEDIUM", "HIGH", false},
		{"LOW fails MEDIUM", "LOW", "MEDIUM", false},
		{"Unknown severity fails HIGH minimum", "UNKNOWN", "HIGH", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.meetsMinSeverity(tt.severity, tt.minSeverity)
			if result != tt.expected {
				t.Errorf("meetsMinSeverity(%q, %q) = %v, want %v", tt.severity, tt.minSeverity, result, tt.expected)
			}
		})
	}
}

// Helper functions to create test observations

func createObservation(source, category, severity, namespace, kind, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"source":    source,
				"category":  category,
				"severity":  severity,
				"eventType": "test-event",
				"resource": map[string]interface{}{
					"kind":      kind,
					"name":      name,
					"namespace": namespace,
				},
			},
		},
	}
}

func createObservationWithEventType(source, category, severity, namespace, kind, name, eventType string) *unstructured.Unstructured {
	obs := createObservation(source, category, severity, namespace, kind, name)
	unstructured.SetNestedField(obs.Object, eventType, "spec", "eventType")
	return obs
}
