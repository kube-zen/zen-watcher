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
	"strings"
)

// normalizeSeverity normalizes severity strings to lowercase standard levels
// CRD expects: critical, high, medium, low, info
func normalizeSeverity(severity string) string {
	upper := strings.ToUpper(severity)
	switch upper {
	case "CRITICAL", "FATAL", "EMERGENCY":
		return "critical"
	case "HIGH", "ERROR", "ALERT":
		return "high"
	case "MEDIUM", "WARNING", "WARN":
		return "medium"
	case "LOW", "INFORMATIONAL":
		return "low"
	case "INFO":
		return "info"
	default:
		return "info" // Default to info instead of unknown
	}
}

// normalizeEventType normalizes eventType to match CRD validation pattern ^[a-z0-9_]+$
// Replaces hyphens and other invalid characters with underscores, converts to lowercase
func normalizeEventType(eventType string) string {
	if eventType == "" {
		return "custom_event" // Default
	}
	// Convert to lowercase and replace hyphens and other non-alphanumeric chars (except underscore) with underscores
	normalized := strings.ToLower(eventType)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	// Remove any remaining invalid characters (keep only a-z, 0-9, _)
	var result strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	if result.Len() == 0 {
		return "custom_event" // Fallback if all chars were invalid
	}
	return result.String()
}
