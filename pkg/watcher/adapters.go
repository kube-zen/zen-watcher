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
