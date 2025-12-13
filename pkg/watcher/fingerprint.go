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
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// RawEvent represents a raw event before any processing
type RawEvent struct {
	Source    string
	Type      string                 // Optional: pre-detected type
	Data      map[string]interface{} // Raw event data (YAML-compatible)
	Resources []ResourceReference    // Optional: pre-extracted resources
}

// ResourceReference represents a Kubernetes resource reference
type ResourceReference struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

// GenerateFingerprint generates a content-based fingerprint from raw event data
// This should be called BEFORE any normalization or processing
func GenerateFingerprint(rawEvent RawEvent) string {
	source := rawEvent.Source
	if source == "" {
		source = "unknown"
	}

	// Generate fingerprint from raw event data
	// Try common identifier fields (configurable via Ingester CRD)
	if idField := extractString(rawEvent.Data, "id", "identifier", "ID", "Identifier"); idField != "" {
		resourceName := extractResourceName(rawEvent.Resources)
		return fmt.Sprintf("%s/%s/%s", source, idField, resourceName)
	}

	// Fallback: hash the raw JSON data
	jsonBytes, err := json.Marshal(rawEvent.Data)
	if err != nil {
		// Last resort: use string representation
		jsonBytes = []byte(fmt.Sprintf("%v", rawEvent.Data))
	}
	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%s/%x", source, hash[:16])
}

// extractString extracts a string value from data map, trying multiple keys
func extractString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	return ""
}

// extractResourceName extracts resource name from resources array
func extractResourceName(resources []ResourceReference) string {
	if len(resources) > 0 {
		return fmt.Sprintf("%s/%s/%s", resources[0].Kind, resources[0].Namespace, resources[0].Name)
	}
	return "unknown"
}
