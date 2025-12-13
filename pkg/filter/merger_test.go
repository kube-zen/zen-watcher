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
)

func TestMergeFilterConfigs_EmptyConfigs(t *testing.T) {
	tests := []struct {
		name           string
		configs        []*FilterConfig
		expectedSource string
	}{
		{
			name:           "No configs",
			configs:        []*FilterConfig{},
			expectedSource: "",
		},
		{
			name:           "Nil config",
			configs:        []*FilterConfig{nil},
			expectedSource: "",
		},
		{
			name: "Empty config",
			configs: []*FilterConfig{
				{Sources: make(map[string]SourceFilter)},
			},
			expectedSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeFilterConfigs(tt.configs...)
			if result == nil {
				t.Fatal("MergeFilterConfigs returned nil")
			}
			if result.Sources == nil {
				t.Fatal("MergeFilterConfigs returned nil Sources map")
			}
			if len(result.Sources) != 0 {
				t.Errorf("Expected empty Sources map, got %d sources", len(result.Sources))
			}
		})
	}
}

func TestMergeFilterConfigs_SingleConfig(t *testing.T) {
	config := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "HIGH",
				Enabled:     boolPtr(true),
			},
		},
	}

	result := MergeFilterConfigs(config)
	if result == nil {
		t.Fatal("MergeFilterConfigs returned nil")
	}

	if len(result.Sources) != 1 {
		t.Fatalf("Expected 1 source, got %d", len(result.Sources))
	}

	trivyFilter, exists := result.Sources["trivy"]
	if !exists {
		t.Fatal("Expected trivy source in merged config")
	}

	if trivyFilter.MinSeverity != "HIGH" {
		t.Errorf("Expected MinSeverity HIGH, got %s", trivyFilter.MinSeverity)
	}

	// Verify it's a copy, not the same reference
	trivyResult := result.Sources["trivy"]
	trivyConfig := config.Sources["trivy"]
	if &trivyResult == &trivyConfig {
		t.Error("Expected a copy, but got the same reference")
	}
}

func TestMergeFilterConfigs_MinSeverity_MostRestrictive(t *testing.T) {
	tests := []struct {
		name           string
		config1        *FilterConfig
		config2        *FilterConfig
		expectedMinSev string
	}{
		{
			name: "CRITICAL more restrictive than HIGH",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: "HIGH"},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: "CRITICAL"},
				},
			},
			expectedMinSev: "CRITICAL",
		},
		{
			name: "HIGH more restrictive than MEDIUM",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: "MEDIUM"},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: "HIGH"},
				},
			},
			expectedMinSev: "HIGH",
		},
		{
			name: "First config wins if equal priority",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: "HIGH"},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: "HIGH"},
				},
			},
			expectedMinSev: "HIGH",
		},
		{
			name: "Empty severity uses other",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: ""},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {MinSeverity: "MEDIUM"},
				},
			},
			expectedMinSev: "MEDIUM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeFilterConfigs(tt.config1, tt.config2)
			if result == nil {
				t.Fatal("MergeFilterConfigs returned nil")
			}

			trivyFilter, exists := result.Sources["trivy"]
			if !exists {
				t.Fatal("Expected trivy source in merged config")
			}

			if trivyFilter.MinSeverity != tt.expectedMinSev {
				t.Errorf("Expected MinSeverity %s, got %s", tt.expectedMinSev, trivyFilter.MinSeverity)
			}
		})
	}
}

func TestMergeFilterConfigs_IncludeSeverity_Intersection(t *testing.T) {
	config1 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				IncludeSeverity: []string{"CRITICAL", "HIGH", "MEDIUM"},
			},
		},
	}
	config2 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				IncludeSeverity: []string{"HIGH", "MEDIUM", "LOW"},
			},
		},
	}

	result := MergeFilterConfigs(config1, config2)
	trivyFilter := result.Sources["trivy"]

	// Intersection should be HIGH and MEDIUM (present in both)
	expected := []string{"HIGH", "MEDIUM"}
	if !equalStringSlice(trivyFilter.IncludeSeverity, expected) {
		t.Errorf("Expected IncludeSeverity %v, got %v", expected, trivyFilter.IncludeSeverity)
	}
}

func TestMergeFilterConfigs_ExcludeLists_Union(t *testing.T) {
	config1 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"kyverno": {
				ExcludeNamespaces: []string{"kube-system", "default"},
				ExcludeKinds:      []string{"Pod", "Secret"},
			},
		},
	}
	config2 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"kyverno": {
				ExcludeNamespaces: []string{"kube-public", "default"}, // default is duplicate
				ExcludeKinds:      []string{"ConfigMap"},
			},
		},
	}

	result := MergeFilterConfigs(config1, config2)
	kyvernoFilter := result.Sources["kyverno"]

	// Union should contain all namespaces (deduplicated)
	expectedNamespaces := []string{"kube-system", "default", "kube-public"}
	if !containsAll(kyvernoFilter.ExcludeNamespaces, expectedNamespaces) {
		t.Errorf("Expected ExcludeNamespaces to contain %v, got %v", expectedNamespaces, kyvernoFilter.ExcludeNamespaces)
	}

	// Union should contain all kinds (deduplicated)
	expectedKinds := []string{"Pod", "Secret", "ConfigMap"}
	if !containsAll(kyvernoFilter.ExcludeKinds, expectedKinds) {
		t.Errorf("Expected ExcludeKinds to contain %v, got %v", expectedKinds, kyvernoFilter.ExcludeKinds)
	}
}

func TestMergeFilterConfigs_IncludeLists_Intersection(t *testing.T) {
	config1 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"falco": {
				IncludeNamespaces: []string{"production", "staging"},
				IncludeKinds:      []string{"Pod", "Deployment"},
			},
		},
	}
	config2 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"falco": {
				IncludeNamespaces: []string{"production", "testing"},
				IncludeKinds:      []string{"Pod", "Service"},
			},
		},
	}

	result := MergeFilterConfigs(config1, config2)
	falcoFilter := result.Sources["falco"]

	// Intersection should be only "production"
	expectedNamespaces := []string{"production"}
	if !equalStringSlice(falcoFilter.IncludeNamespaces, expectedNamespaces) {
		t.Errorf("Expected IncludeNamespaces %v, got %v", expectedNamespaces, falcoFilter.IncludeNamespaces)
	}

	// Intersection should be only "Pod"
	expectedKinds := []string{"Pod"}
	if !equalStringSlice(falcoFilter.IncludeKinds, expectedKinds) {
		t.Errorf("Expected IncludeKinds %v, got %v", expectedKinds, falcoFilter.IncludeKinds)
	}
}

func TestMergeFilterConfigs_Enabled_ANDLogic(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name          string
		config1       *FilterConfig
		config2       *FilterConfig
		expectedValue *bool
	}{
		{
			name: "Both enabled -> enabled",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {Enabled: &enabled},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {Enabled: &enabled},
				},
			},
			expectedValue: &enabled,
		},
		{
			name: "First disabled -> disabled",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {Enabled: &disabled},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {Enabled: &enabled},
				},
			},
			expectedValue: &disabled,
		},
		{
			name: "Second disabled -> disabled",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {Enabled: &enabled},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {Enabled: &disabled},
				},
			},
			expectedValue: &disabled,
		},
		{
			name: "Both nil -> nil (defaults to enabled)",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {},
				},
			},
			expectedValue: nil,
		},
		{
			name: "One nil, one enabled -> enabled",
			config1: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {},
				},
			},
			config2: &FilterConfig{
				Sources: map[string]SourceFilter{
					"trivy": {Enabled: &enabled},
				},
			},
			expectedValue: &enabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeFilterConfigs(tt.config1, tt.config2)
			trivyFilter := result.Sources["trivy"]

			if tt.expectedValue == nil {
				if trivyFilter.Enabled != nil {
					t.Errorf("Expected Enabled to be nil, got %v", *trivyFilter.Enabled)
				}
			} else {
				if trivyFilter.Enabled == nil {
					t.Fatal("Expected Enabled to be set, got nil")
				}
				if *trivyFilter.Enabled != *tt.expectedValue {
					t.Errorf("Expected Enabled %v, got %v", *tt.expectedValue, *trivyFilter.Enabled)
				}
			}
		})
	}
}

func TestMergeFilterConfigs_MultipleSources(t *testing.T) {
	config1 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "HIGH",
			},
			"falco": {
				MinSeverity: "CRITICAL",
			},
		},
	}
	config2 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM", // Should merge with HIGH (HIGH wins)
			},
			"kyverno": {
				MinSeverity: "MEDIUM",
			},
		},
	}

	result := MergeFilterConfigs(config1, config2)

	if len(result.Sources) != 3 {
		t.Fatalf("Expected 3 sources, got %d", len(result.Sources))
	}

	if result.Sources["trivy"].MinSeverity != "HIGH" {
		t.Errorf("Expected trivy MinSeverity HIGH (most restrictive), got %s", result.Sources["trivy"].MinSeverity)
	}

	if result.Sources["falco"].MinSeverity != "CRITICAL" {
		t.Errorf("Expected falco MinSeverity CRITICAL, got %s", result.Sources["falco"].MinSeverity)
	}

	if result.Sources["kyverno"].MinSeverity != "MEDIUM" {
		t.Errorf("Expected kyverno MinSeverity MEDIUM, got %s", result.Sources["kyverno"].MinSeverity)
	}
}

func TestMergeFilterConfigs_CaseInsensitive(t *testing.T) {
	config1 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"Trivy": { // Capital T
				MinSeverity: "HIGH",
			},
		},
	}
	config2 := &FilterConfig{
		Sources: map[string]SourceFilter{
			"TRIVY": { // All caps
				MinSeverity: "MEDIUM",
			},
		},
	}

	result := MergeFilterConfigs(config1, config2)

	// Should merge into lowercase "trivy"
	if len(result.Sources) != 1 {
		t.Fatalf("Expected 1 source (case-insensitive merge), got %d", len(result.Sources))
	}

	_, exists := result.Sources["trivy"]
	if !exists {
		t.Fatal("Expected merged source to be lowercase 'trivy'")
	}
}

func TestMergeFilterConfigs_ComplexMerge(t *testing.T) {
	// ConfigMap filter
	configMapConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity:       "MEDIUM",
				ExcludeNamespaces: []string{"kube-system"},
				ExcludeKinds:      []string{"Pod"},
				Enabled:           boolPtr(true),
			},
		},
	}

	// Ingester CRD filter configuration
	crdConfig := &FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity:       "HIGH",                  // More restrictive
				ExcludeNamespaces: []string{"kube-public"}, // Union with kube-system
				ExcludeKinds:      []string{"Secret"},      // Union with Pod
				Enabled:           boolPtr(true),
			},
		},
	}

	result := MergeFilterConfigs(configMapConfig, crdConfig)
	trivyFilter := result.Sources["trivy"]

	// MinSeverity: HIGH should win (more restrictive)
	if trivyFilter.MinSeverity != "HIGH" {
		t.Errorf("Expected MinSeverity HIGH (most restrictive), got %s", trivyFilter.MinSeverity)
	}

	// ExcludeNamespaces: Union of both
	expectedNamespaces := []string{"kube-system", "kube-public"}
	if !containsAll(trivyFilter.ExcludeNamespaces, expectedNamespaces) {
		t.Errorf("Expected ExcludeNamespaces to contain %v, got %v", expectedNamespaces, trivyFilter.ExcludeNamespaces)
	}

	// ExcludeKinds: Union of both
	expectedKinds := []string{"Pod", "Secret"}
	if !containsAll(trivyFilter.ExcludeKinds, expectedKinds) {
		t.Errorf("Expected ExcludeKinds to contain %v, got %v", expectedKinds, trivyFilter.ExcludeKinds)
	}

	// Enabled: Both true -> true
	if trivyFilter.Enabled == nil || !*trivyFilter.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, s := range a {
		aMap[s] = true
	}
	for _, s := range b {
		if !aMap[s] {
			return false
		}
	}
	return true
}

func containsAll(slice []string, items []string) bool {
	sliceMap := make(map[string]bool)
	for _, s := range slice {
		sliceMap[s] = true
	}
	for _, item := range items {
		if !sliceMap[item] {
			return false
		}
	}
	return true
}
