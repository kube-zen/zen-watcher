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

// TestFilter_ExpressionVsListBased tests that expression-based filters work
// and that list-based filters still work when expression is not set
func TestFilter_ExpressionVsListBased(t *testing.T) {
	// Test observation
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "HIGH",
				"category": "security",
			},
		},
	}

	// Test 1: Expression-based filter (should work)
	configWithExpression := &FilterConfig{
		Expression: `spec.severity >= "HIGH" AND spec.category = "security"`,
		Sources:    make(map[string]SourceFilter),
	}
	filterWithExpression := NewFilter(configWithExpression)
	allowed, reason := filterWithExpression.AllowWithReason(obs)
	if !allowed {
		t.Errorf("Expression filter should allow observation, got reason: %s", reason)
	}

	// Test 2: List-based filter (should still work when expression is not set)
	configLegacy := &FilterConfig{
		Expression: "", // No expression
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "HIGH",
			},
		},
	}
	filterListBased := NewFilter(configLegacy)
	allowed, reason = filterListBased.AllowWithReason(obs)
	if !allowed {
		t.Errorf("List-based filter should allow observation, got reason: %s", reason)
	}

	// Test 3: Expression takes precedence over list-based fields
	configBoth := &FilterConfig{
		Expression: `spec.severity = "LOW"`, // This should filter out HIGH
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "HIGH", // List-based field should be ignored when expression is set
			},
		},
	}
	filterBoth := NewFilter(configBoth)
	allowed, reason = filterBoth.AllowWithReason(obs)
	if allowed {
		t.Errorf("Expression should take precedence and filter out HIGH severity")
	}
	if reason != "expression_filtered" {
		t.Errorf("Expected reason 'expression_filtered', got: %s", reason)
	}
}

// TestFilter_InvalidExpressionFallback tests that invalid expressions
// fall back to list-based filters gracefully
func TestFilter_InvalidExpressionFallback(t *testing.T) {
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "HIGH",
			},
		},
	}

	// Invalid expression should fall back to list-based filters
	config := &FilterConfig{
		Expression: `invalid syntax !!!`, // Invalid
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "HIGH",
			},
		},
	}
	filter := NewFilter(config)
	allowed, _ := filter.AllowWithReason(obs)
	if !allowed {
		t.Error("Invalid expression should fall back to list-based filters and allow HIGH severity")
	}
}

// TestFilter_ExpressionErrorHandling tests error handling for expression evaluation errors
func TestFilter_ExpressionErrorHandling(t *testing.T) {
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "HIGH",
			},
		},
	}

	// Expression that causes evaluation error (type mismatch)
	config := &FilterConfig{
		Expression: `spec.severity > 123`, // Comparing string to number
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "HIGH",
			},
		},
	}
	filter := NewFilter(config)
	allowed, _ := filter.AllowWithReason(obs)
	// Should fall back to list-based filters
	if !allowed {
		t.Error("Expression evaluation error should fall back to list-based filters")
	}
}
