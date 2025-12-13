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

func TestExpressionFilter_BasicComparisons(t *testing.T) {
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"severity": "HIGH",
				"category": "security",
			},
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
		wantErr    bool
	}{
		{
			name:       "equality match",
			expression: `spec.severity = "HIGH"`,
			expected:   true,
		},
		{
			name:       "equality mismatch",
			expression: `spec.severity = "LOW"`,
			expected:   false,
		},
		{
			name:       "greater than severity",
			expression: `spec.severity >= "HIGH"`,
			expected:   true,
		},
		{
			name:       "less than severity",
			expression: `spec.severity < "CRITICAL"`,
			expected:   true,
		},
		{
			name:       "IN operator match",
			expression: `spec.category IN ["security", "compliance"]`,
			expected:   true,
		},
		{
			name:       "IN operator mismatch",
			expression: `spec.category IN ["ops", "monitoring"]`,
			expected:   false,
		},
		{
			name:       "NOT IN operator",
			expression: `spec.category NOT IN ["ops", "monitoring"]`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewExpressionFilter(tt.expression)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("NewExpressionFilter() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			result, err := filter.Evaluate(obs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if result != tt.expected {
				t.Errorf("Evaluate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpressionFilter_LogicalOperators(t *testing.T) {
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"severity": "HIGH",
				"category": "security",
			},
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "AND both true",
			expression: `spec.severity >= "HIGH" AND spec.category = "security"`,
			expected:   true,
		},
		{
			name:       "AND one false",
			expression: `spec.severity >= "HIGH" AND spec.category = "compliance"`,
			expected:   false,
		},
		{
			name:       "OR both true",
			expression: `spec.severity >= "CRITICAL" OR spec.category = "security"`,
			expected:   true,
		},
		{
			name:       "OR one true",
			expression: `spec.severity >= "CRITICAL" OR spec.category = "security"`,
			expected:   true,
		},
		{
			name:       "OR both false",
			expression: `spec.severity >= "CRITICAL" OR spec.category = "compliance"`,
			expected:   false,
		},
		{
			name:       "NOT operator",
			expression: `NOT (spec.category = "compliance")`,
			expected:   true,
		},
		{
			name:       "parentheses grouping",
			expression: `(spec.severity >= "HIGH") AND (spec.category IN ["security", "compliance"])`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewExpressionFilter(tt.expression)
			if err != nil {
				t.Fatalf("NewExpressionFilter() error = %v", err)
			}

			result, err := filter.Evaluate(obs)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Evaluate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpressionFilter_Macros(t *testing.T) {
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"severity": "CRITICAL",
				"category": "security",
			},
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "is_critical macro",
			expression: `is_critical`,
			expected:   true,
		},
		{
			name:       "is_security macro",
			expression: `is_security`,
			expected:   true,
		},
		{
			name:       "macro with AND",
			expression: `is_critical AND is_security`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewExpressionFilter(tt.expression)
			if err != nil {
				t.Fatalf("NewExpressionFilter() error = %v", err)
			}

			result, err := filter.Evaluate(obs)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Evaluate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpressionFilter_FieldExistence(t *testing.T) {
	obsWithField := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"details": map[string]interface{}{
					"vulnerabilityID": "CVE-2024-1234",
				},
			},
		},
	}

	obsWithoutField := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"severity": "HIGH",
			},
		},
	}

	tests := []struct {
		name       string
		expression string
		obs        *unstructured.Unstructured
		expected   bool
	}{
		{
			name:       "EXISTS field present",
			expression: `spec.details.vulnerabilityID EXISTS`,
			obs:        obsWithField,
			expected:   true,
		},
		{
			name:       "EXISTS field missing",
			expression: `spec.details.vulnerabilityID EXISTS`,
			obs:        obsWithoutField,
			expected:   false,
		},
		{
			name:       "NOT EXISTS field missing",
			expression: `spec.details.vulnerabilityID NOT EXISTS`,
			obs:        obsWithoutField,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewExpressionFilter(tt.expression)
			if err != nil {
				t.Fatalf("NewExpressionFilter() error = %v", err)
			}

			result, err := filter.Evaluate(tt.obs)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Evaluate() = %v, want %v", result, tt.expected)
			}
		})
	}
}
