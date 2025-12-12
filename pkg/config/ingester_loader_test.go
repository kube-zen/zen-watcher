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

package config

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConvertToIngesterConfig_ProcessingFilter(t *testing.T) {
	// Test W58: spec.processing.filter should be loaded (canonical location)
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-ingester",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"source":    "test-source",
				"ingester":  "informer",
				"processing": map[string]interface{}{
					"filter": map[string]interface{}{
						"expression": "severity >= HIGH",
						"minPriority": 0.5,
						"includeNamespaces": []interface{}{"ns1", "ns2"},
						"excludeNamespaces": []interface{}{"kube-system"},
					},
					"dedup": map[string]interface{}{
						"enabled": true,
						"window":  "60s",
						"strategy": "fingerprint",
					},
				},
			},
		},
	}

	ii := &IngesterInformer{}
	config := ii.convertToIngesterConfig(u)

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	// Verify filter config is loaded from spec.processing.filter
	if config.Filter == nil {
		t.Fatal("Expected Filter config to be loaded from spec.processing.filter")
	}

	if config.Filter.Expression != "severity >= HIGH" {
		t.Errorf("Expected expression 'severity >= HIGH', got '%s'", config.Filter.Expression)
	}

	if config.Filter.MinPriority != 0.5 {
		t.Errorf("Expected MinPriority 0.5, got %f", config.Filter.MinPriority)
	}

	if len(config.Filter.IncludeNamespaces) != 2 {
		t.Errorf("Expected 2 include namespaces, got %d", len(config.Filter.IncludeNamespaces))
	}

	if len(config.Filter.ExcludeNamespaces) != 1 {
		t.Errorf("Expected 1 exclude namespace, got %d", len(config.Filter.ExcludeNamespaces))
	}

	// Verify dedup config is also loaded
	if config.Dedup == nil {
		t.Fatal("Expected Dedup config to be loaded")
	}

	if config.Dedup.Strategy != "fingerprint" {
		t.Errorf("Expected dedup strategy 'fingerprint', got '%s'", config.Dedup.Strategy)
	}
}

func TestConvertToIngesterConfig_LegacyFilters(t *testing.T) {
	// Test backward compatibility: spec.filters should still work
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-ingester",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"source":   "test-source",
				"ingester": "informer",
				"filters": map[string]interface{}{
					"expression": "severity = CRITICAL",
					"minPriority": 0.7,
				},
			},
		},
	}

	ii := &IngesterInformer{}
	config := ii.convertToIngesterConfig(u)

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	// Verify filter config is loaded from legacy spec.filters
	if config.Filter == nil {
		t.Fatal("Expected Filter config to be loaded from legacy spec.filters")
	}

	if config.Filter.Expression != "severity = CRITICAL" {
		t.Errorf("Expected expression 'severity = CRITICAL', got '%s'", config.Filter.Expression)
	}
}

func TestConvertToIngesterConfig_ProcessingFilterTakesPrecedence(t *testing.T) {
	// Test that spec.processing.filter takes precedence over spec.filters
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-ingester",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"source":   "test-source",
				"ingester": "informer",
				"processing": map[string]interface{}{
					"filter": map[string]interface{}{
						"expression": "severity >= HIGH", // Canonical location
					},
				},
				"filters": map[string]interface{}{
					"expression": "severity = CRITICAL", // Legacy location (should be ignored)
				},
			},
		},
	}

	ii := &IngesterInformer{}
	config := ii.convertToIngesterConfig(u)

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	// Canonical location should take precedence
	if config.Filter == nil {
		t.Fatal("Expected Filter config")
	}

	if config.Filter.Expression != "severity >= HIGH" {
		t.Errorf("Expected canonical expression 'severity >= HIGH', got '%s'", config.Filter.Expression)
	}
}

