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

package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// createObservationFromWebhookPayload creates an Observation CRD from a webhook payload
// using Ingester destination mapping configuration
func (s *Server) createObservationFromWebhookPayload(
	payload map[string]interface{},
	ingesterConfig *config.IngesterConfig,
	source string,
	correlationID string,
) *unstructured.Unstructured {
	// Get destination mapping (from first CRD destination)
	var mapping *config.NormalizationConfig
	if ingesterConfig.Normalization != nil {
		mapping = ingesterConfig.Normalization
	}

	// Start building Observation
	now := time.Now().Format(time.RFC3339)
	namespace := "default"
	if ingesterConfig.Namespace != "" {
		namespace = ingesterConfig.Namespace
	}

	// Extract domain and type from mapping
	domain := "custom"
	typeStr := "webhook_event"
	if mapping != nil {
		if mapping.Domain != "" {
			domain = mapping.Domain
		}
		if mapping.Type != "" {
			typeStr = mapping.Type
		}
	}

	// Build observation spec
	spec := map[string]interface{}{
		"source":    source,
		"category":  domain,
		"eventType": typeStr,
		"detectedAt": now,
	}

	// Apply field mapping from Ingester destinations
	if mapping != nil && len(mapping.FieldMapping) > 0 {
		details := make(map[string]interface{})
		
		// Apply JSONPath field mappings
		for _, fm := range mapping.FieldMapping {
			value := extractJSONPath(payload, fm.From)
			if value != nil {
				// Apply transform if specified
				if fm.Transform != "" {
					value = applyTransform(value, fm.Transform)
				}
				details[fm.To] = value
			}
		}
		
		if len(details) > 0 {
			spec["details"] = details
		}

		// Apply priority mapping
		if len(mapping.Priority) > 0 {
			// Try to extract priority from payload
			priorityVal := extractJSONPath(payload, "$.priority")
			if priorityVal == nil {
				priorityVal = extractJSONPath(payload, "$.severity")
			}
			if priorityVal != nil {
				priorityStr := fmt.Sprintf("%v", priorityVal)
				if priority, ok := mapping.Priority[priorityStr]; ok {
					spec["priority"] = priority
				} else {
					// Default priority if not in mapping
					spec["priority"] = 0.5
				}
			} else {
				spec["priority"] = 0.5 // Default priority
			}
		} else {
			spec["priority"] = 0.5 // Default priority
		}
	} else {
		// No mapping - include raw payload in details
		spec["details"] = payload
		spec["priority"] = 0.5
	}

	// Set severity (default to UNKNOWN if not mapped)
	severity := "UNKNOWN"
	if mapping != nil && len(mapping.Priority) > 0 {
		// Try to determine severity from priority
		if priority, ok := spec["priority"].(float64); ok {
			if priority >= 0.9 {
				severity = "CRITICAL"
			} else if priority >= 0.7 {
				severity = "HIGH"
			} else if priority >= 0.5 {
				severity = "MEDIUM"
			} else if priority >= 0.3 {
				severity = "LOW"
			}
		}
	}
	spec["severity"] = severity

	// Build Observation object
	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"generateName": fmt.Sprintf("%s-", source),
				"namespace":    namespace,
				"labels": map[string]interface{}{
					"source":   source,
					"category": domain,
					"severity": severity,
				},
				"annotations": map[string]interface{}{
					"zen.kube-zen.io/correlation-id": correlationID,
					"zen.kube-zen.io/ingester":       ingesterConfig.Name,
				},
			},
			"spec": spec,
		},
	}

	return observation
}

// extractJSONPath extracts a value from a map using a simple JSONPath expression
// Supports: $.field, $.nested.field, $.array[0]
func extractJSONPath(obj map[string]interface{}, path string) interface{} {
	if path == "" || path == "$" {
		return obj
	}

	// Remove leading $.
	if len(path) > 2 && path[:2] == "$." {
		path = path[2:]
	}

	// Simple path traversal (no complex JSONPath for now)
	parts := splitPath(path)
	current := interface{}(obj)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil
			}
		case []interface{}:
			// Try to parse as array index
			if idx, err := parseInt(part); err == nil && idx >= 0 && idx < len(v) {
				current = v[idx]
			} else {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

// splitPath splits a JSONPath into parts (handles array indices)
func splitPath(path string) []string {
	var parts []string
	var current strings.Builder
	inBrackets := false

	for _, char := range path {
		if char == '[' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			inBrackets = true
		} else if char == ']' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			inBrackets = false
		} else if char == '.' && !inBrackets {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseInt parses a string as an integer
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// applyTransform applies a transform function to a value
func applyTransform(value interface{}, transform string) interface{} {
	if transform == "" {
		return value
	}

	str, ok := value.(string)
	if !ok {
		return value
	}

	switch transform {
	case "lower":
		return strings.ToLower(str)
	case "upper":
		return strings.ToUpper(str)
	default:
		// Handle truncate:N
		if strings.HasPrefix(transform, "truncate:") {
			var maxLen int
			if _, err := fmt.Sscanf(transform, "truncate:%d", &maxLen); err == nil {
				if len(str) > maxLen {
					return str[:maxLen]
				}
			}
		}
	}

	return value
}

