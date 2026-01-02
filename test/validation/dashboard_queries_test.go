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

package validation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dashboard represents a Grafana dashboard JSON structure
type Dashboard struct {
	Panels     []Panel     `json:"panels"`
	Templating Templating  `json:"templating,omitempty"`
	Title      string      `json:"title,omitempty"`
	UID        string      `json:"uid,omitempty"`
}

type Panel struct {
	Targets []Target `json:"targets,omitempty"`
	Title   string   `json:"title,omitempty"`
	Type    string   `json:"type,omitempty"`
	Panels  []Panel  `json:"panels,omitempty"` // For row panels
}

type Target struct {
	Expr string `json:"expr,omitempty"`
}

type Templating struct {
	List []Variable `json:"list,omitempty"`
}

type Variable struct {
	Name  string `json:"name,omitempty"`
	Query string `json:"query,omitempty"`
}

// TestDashboardQueriesSyntax validates that all dashboard JSON files are valid
func TestDashboardQueriesSyntax(t *testing.T) {
	dashboardFiles := []string{
		"../../config/dashboards/zen-watcher-executive.json",
		"../../config/dashboards/zen-watcher-operations.json",
		"../../config/dashboards/zen-watcher-security.json",
		"../../config/dashboards/zen-watcher-dashboard.json",
		"../../config/dashboards/zen-watcher-namespace-health.json",
		"../../config/dashboards/zen-watcher-explorer.json",
	}

	for _, file := range dashboardFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var dashboard Dashboard
			if err := json.Unmarshal(data, &dashboard); err != nil {
				t.Fatalf("Failed to parse JSON in %s: %v", file, err)
			}
		})
	}
}

// TestDashboardQueriesExpressions validates that dashboard queries reference valid metrics
func TestDashboardQueriesExpressions(t *testing.T) {
	dashboardFiles := []string{
		"../../config/dashboards/zen-watcher-executive.json",
		"../../config/dashboards/zen-watcher-operations.json",
		"../../config/dashboards/zen-watcher-security.json",
		"../../config/dashboards/zen-watcher-dashboard.json",
		"../../config/dashboards/zen-watcher-namespace-health.json",
		"../../config/dashboards/zen-watcher-explorer.json",
	}

	for _, file := range dashboardFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var dashboard Dashboard
			if err := json.Unmarshal(data, &dashboard); err != nil {
				t.Fatalf("Failed to parse JSON in %s: %v", file, err)
			}

			validatePanelQueries(t, dashboard.Panels, file)
		})
	}
}

func validatePanelQueries(t *testing.T, panels []Panel, file string) {
	for _, panel := range panels {
		// Recursively check nested panels (row panels)
		if len(panel.Panels) > 0 {
			validatePanelQueries(t, panel.Panels, file)
		}

		for _, target := range panel.Targets {
			if target.Expr == "" {
				continue
			}

			expr := target.Expr

			// Check for deprecated _ratio suffix metrics
			if strings.Contains(expr, "zen_watcher_dedup_cache_usage_ratio") {
				t.Errorf("Panel '%s' in %s uses deprecated metric zen_watcher_dedup_cache_usage_ratio. Use zen_watcher_dedup_cache_usage instead",
					panel.Title, file)
			}
			if strings.Contains(expr, "zen_watcher_webhook_queue_usage_ratio") {
				t.Errorf("Panel '%s' in %s uses deprecated metric zen_watcher_webhook_queue_usage_ratio. Use zen_watcher_webhook_queue_usage instead",
					panel.Title, file)
			}

			// Check for uppercase severity values
			invalidSeverities := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "WARNING", "INFO", "Critical", "High", "Medium", "Low"}
			for _, invalid := range invalidSeverities {
				if strings.Contains(expr, `severity="`+invalid+`"`) || strings.Contains(expr, `severity=`+invalid) {
					t.Errorf("Panel '%s' in %s uses uppercase severity value '%s'. Use lowercase instead (e.g., 'critical', 'high')",
						panel.Title, file, invalid)
				}
			}
		}
	}
}

// TestDashboardVariables validates that dashboard variables are properly configured
func TestDashboardVariables(t *testing.T) {
	dashboardFiles := []string{
		"../../config/dashboards/zen-watcher-executive.json",
		"../../config/dashboards/zen-watcher-operations.json",
		"../../config/dashboards/zen-watcher-security.json",
	}

	for _, file := range dashboardFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var dashboard Dashboard
			if err := json.Unmarshal(data, &dashboard); err != nil {
				t.Fatalf("Failed to parse JSON in %s: %v", file, err)
			}

			// Check that primary dashboards have source variable
			hasSource := false
			for _, variable := range dashboard.Templating.List {
				if variable.Name == "source" {
					hasSource = true
					break
				}
			}

			if !hasSource {
				t.Errorf("Dashboard %s is missing 'source' variable", file)
			}
		})
	}
}

// TestDashboardSeverityFilters validates that severity filters use lowercase values
func TestDashboardSeverityFilters(t *testing.T) {
	dashboardFiles := []string{
		"../../config/dashboards/zen-watcher-executive.json",
		"../../config/dashboards/zen-watcher-operations.json",
		"../../config/dashboards/zen-watcher-security.json",
		"../../config/dashboards/zen-watcher-dashboard.json",
		"../../config/dashboards/zen-watcher-namespace-health.json",
		"../../config/dashboards/zen-watcher-explorer.json",
	}

	for _, file := range dashboardFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			// Read as raw JSON to check for uppercase severity in filters
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("Failed to parse JSON in %s: %v", file, err)
			}

			// Convert to string and check for uppercase severity patterns
			jsonStr := string(data)
			invalidSeverities := []string{`severity="CRITICAL"`, `severity="HIGH"`, `severity="MEDIUM"`, `severity="LOW"`, `severity="WARNING"`}
			for _, invalid := range invalidSeverities {
				if strings.Contains(jsonStr, invalid) {
					t.Errorf("Dashboard %s contains uppercase severity filter: %s. Use lowercase instead",
						file, invalid)
				}
			}
		})
	}
}

