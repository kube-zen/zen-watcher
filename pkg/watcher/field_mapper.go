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

package watcher

import (
	"fmt"
	"strconv"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FieldMapper applies field mappings to observations, supporting constant values and static mappings
type FieldMapper struct {
	fieldExtractor *FieldExtractor
}

// NewFieldMapper creates a new field mapper
func NewFieldMapper() *FieldMapper {
	return &FieldMapper{
		fieldExtractor: NewFieldExtractor(),
	}
}

// ApplyFieldMapping applies a field mapping to an observation
func (fm *FieldMapper) ApplyFieldMapping(
	observation *unstructured.Unstructured,
	mapping generic.FieldMapping,
	sourceData map[string]interface{},
) error {
	// Handle constant values (highest priority)
	if mapping.Constant != "" {
		return fm.setField(observation, mapping.To, mapping.Constant)
	}

	// Handle static mappings
	if len(mapping.StaticMappings) > 0 {
		if mapping.From == "" {
			return fmt.Errorf("static mappings require 'from' field to be set")
		}

		// Extract source value
		sourceVal := fm.getNestedField(sourceData, mapping.From)
		if sourceVal == nil {
			// Source field not found, skip mapping
			return nil
		}

		// Convert source value to string for lookup
		sourceValStr := fmt.Sprint(sourceVal)

		// Look up in static mappings
		if mappedVal, exists := mapping.StaticMappings[sourceValStr]; exists {
			return fm.setField(observation, mapping.To, mappedVal)
		}

		// No mapping found, skip
		return nil
	}

	// Handle regular field mapping (from -> to)
	if mapping.From == "" {
		return fmt.Errorf("field mapping requires either 'from', 'constant', or 'staticMappings'")
	}

	sourceVal := fm.getNestedField(sourceData, mapping.From)
	if sourceVal != nil {
		return fm.setField(observation, mapping.To, sourceVal)
	}

	return nil
}

// ApplyTTLMapping applies TTL mapping based on field mappings
// Returns TTL in seconds, or 0 if not mapped
func (fm *FieldMapper) ApplyTTLMapping(
	mapping generic.FieldMapping,
	sourceData map[string]interface{},
) (int64, error) {
	var ttlStr string

	// Handle constant TTL
	if mapping.Constant != "" {
		ttlStr = mapping.Constant
	} else if len(mapping.StaticMappings) > 0 {
		// Handle static TTL mappings (e.g., severity -> TTL)
		if mapping.From == "" {
			return 0, fmt.Errorf("static TTL mappings require 'from' field")
		}

		sourceVal := fm.getNestedField(sourceData, mapping.From)
		if sourceVal == nil {
			return 0, nil // No source value, skip
		}

		sourceValStr := fmt.Sprint(sourceVal)
		if mappedTTL, exists := mapping.StaticMappings[sourceValStr]; exists {
			ttlStr = mappedTTL
		} else {
			return 0, nil // No mapping found
		}
	} else if mapping.From != "" {
		// Regular field mapping
		sourceVal := fm.getNestedField(sourceData, mapping.From)
		if sourceVal == nil {
			return 0, nil
		}
		ttlStr = fmt.Sprint(sourceVal)
	} else {
		return 0, nil
	}

	// Parse TTL string to seconds
	// Supports formats: "1w", "3d", "24h", "5m", "300" (seconds)
	ttlSeconds, err := parseTTLToSeconds(ttlStr)
	if err != nil {
		logger := sdklog.NewLogger("zen-watcher")
		logger.Warn("Failed to parse TTL value",
			sdklog.Operation("ttl_parsing"),
			sdklog.Error(err),
			sdklog.String("ttl_string", ttlStr),
			sdklog.String("from_field", mapping.From),
			sdklog.String("to_field", mapping.To))
		return 0, fmt.Errorf("invalid TTL format '%s': %w", ttlStr, err)
	}

	return ttlSeconds, nil
}

// parseTTLToSeconds parses TTL string to seconds
// Supports: "1w", "3d", "24h", "5m", "300" (seconds)
func parseTTLToSeconds(ttlStr string) (int64, error) {
	if ttlStr == "" {
		return 0, fmt.Errorf("empty TTL string")
	}

	// Try parsing as duration string first
	if duration, err := time.ParseDuration(ttlStr); err == nil {
		return int64(duration.Seconds()), nil
	}

	// Try parsing as number (assume seconds)
	if seconds, err := strconv.ParseInt(ttlStr, 10, 64); err == nil {
		return seconds, nil
	}

	// Try parsing with day/week suffixes
	var multiplier int64 = 1
	var numStr string

	if len(ttlStr) > 1 {
		lastChar := ttlStr[len(ttlStr)-1:]
		switch lastChar {
		case "w":
			multiplier = 7 * 24 * 60 * 60 // weeks to seconds
			numStr = ttlStr[:len(ttlStr)-1]
		case "d":
			multiplier = 24 * 60 * 60 // days to seconds
			numStr = ttlStr[:len(ttlStr)-1]
		case "h":
			multiplier = 60 * 60 // hours to seconds
			numStr = ttlStr[:len(ttlStr)-1]
		case "m":
			multiplier = 60 // minutes to seconds
			numStr = ttlStr[:len(ttlStr)-1]
		case "s":
			multiplier = 1
			numStr = ttlStr[:len(ttlStr)-1]
		default:
			return 0, fmt.Errorf("unknown TTL suffix: %s", lastChar)
		}

		if num, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			return num * multiplier, nil
		}
	}

	return 0, fmt.Errorf("unable to parse TTL: %s", ttlStr)
}

// setField sets a field in the observation spec
func (fm *FieldMapper) setField(observation *unstructured.Unstructured, fieldPath string, value interface{}) error {
	// Ensure spec exists
	spec, _ := fm.fieldExtractor.ExtractMap(observation.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
		unstructured.SetNestedMap(observation.Object, spec, "spec")
	}

	// Handle nested paths (e.g., "ttl" or "resource.name")
	parts := splitPath(fieldPath)
	if len(parts) == 1 {
		spec[parts[0]] = value
	} else {
		// Nested path - create nested structure
		current := spec
		for i := 0; i < len(parts)-1; i++ {
			part := parts[i]
			if nested, ok := current[part].(map[string]interface{}); ok {
				current = nested
			} else {
				newNested := make(map[string]interface{})
				current[part] = newNested
				current = newNested
			}
		}
		current[parts[len(parts)-1]] = value
	}

	unstructured.SetNestedMap(observation.Object, spec, "spec")
	return nil
}

// getNestedField extracts a nested field from source data using JSONPath-like syntax
func (fm *FieldMapper) getNestedField(data map[string]interface{}, path string) interface{} {
	parts := splitPath(path)
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}

		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}

	return nil
}

// splitPath splits a dot-separated path into parts
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	parts := []string{}
	current := ""
	for _, char := range path {
		if char == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
