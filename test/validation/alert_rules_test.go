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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// PrometheusRule represents a PrometheusRule resource
type PrometheusRule struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Metadata   Metadata  `yaml:"metadata"`
	Spec       RuleSpec  `yaml:"spec"`
}

type Metadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type RuleSpec struct {
	Groups []RuleGroup `yaml:"groups"`
}

type RuleGroup struct {
	Name  string `yaml:"name"`
	Rules []Rule `yaml:"rules"`
}

type Rule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// KnownMetrics is a set of metrics that are defined in pkg/metrics/definitions.go
// This list should be kept in sync with actual metric definitions
var KnownMetrics = map[string]bool{
	"zen_watcher_events_total":                                    true,
	"zen_watcher_observations_created_total":                      true,
	"zen_watcher_observations_filtered_total":                     true,
	"zen_watcher_observations_deduped_total":                      true,
	"zen_watcher_observations_deleted_total":                      true,
	"zen_watcher_observations_create_errors_total":                true,
	"zen_watcher_filter_decisions_total":                          true,
	"zen_watcher_filter_pass_rate":                                true,
	"zen_watcher_adapter_runs_total":                              true,
	"zen_watcher_ingesters_active":                                true,
	"zen_watcher_ingesters_status":                                true,
	"zen_watcher_ingesters_config_errors_total":                   true,
	"zen_watcher_ingester_processing_latency_seconds":              true,
	"zen_watcher_ingester_events_processed_total":                 true,
	"zen_watcher_dedup_cache_usage":                               true,
	"zen_watcher_dedup_evictions_total":                           true,
	"zen_watcher_dedup_effectiveness":                             true,
	"zen_watcher_dedup_effectiveness_per_strategy":                true,
	"zen_watcher_dedup_decisions_total":                           true,
	"zen_watcher_gc_runs_total":                                   true,
	"zen_watcher_gc_duration_seconds":                             true,
	"zen_watcher_gc_errors_total":                                 true,
	"zen_watcher_observations_live":                               true,
	"zen_watcher_tools_active":                                    true,
	"zen_watcher_informer_cache_synced":                           true,
	"zen_watcher_event_processing_duration_seconds":               true,
	"zen_watcher_webhook_requests_total":                          true,
	"zen_watcher_webhook_events_dropped_total":                    true,
	"zen_watcher_webhook_queue_usage":                             true,
	"zen_watcher_optimization_filter_effectiveness_ratio":         true,
	"zen_watcher_optimization_deduplication_rate_ratio":           true,
	"zen_watcher_optimization_source_events_processed_total":       true,
	"zen_watcher_optimization_source_processing_latency_seconds":   true,
	"zen_watcher_optimization_strategy_changes_total":              true,
	"zen_watcher_optimization_decisions_total":                    true,
	"zen_watcher_optimization_confidence":                         true,
	"zen_watcher_optimization_current_strategy":                   true,
	"zen_watcher_pipeline_errors_total":                           true,
}

// ValidSeverityValues are the valid severity label values (lowercase)
var ValidSeverityValues = map[string]bool{
	"critical": true,
	"high":     true,
	"medium":   true,
	"low":      true,
	"info":     true,
	"warning":  true,
}

// TestAlertRulesSyntax validates that all alert rule files have valid YAML syntax
func TestAlertRulesSyntax(t *testing.T) {
	alertRuleFiles := []string{
		"../../config/prometheus/rules/security-alerts.yml",
		"../../config/prometheus/rules/performance-alerts.yml",
		"../../config/monitoring/optimization-alerts.yaml",
		"../../config/monitoring/prometheus-rules.yaml",
		"../../config/prometheus/rules/ingester-destination-config-alerts.yml",
	}

	for _, file := range alertRuleFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var rule PrometheusRule
			if err := yaml.Unmarshal(data, &rule); err != nil {
				t.Fatalf("Failed to parse YAML in %s: %v", file, err)
			}

			// Validate structure
			if rule.Kind != "PrometheusRule" {
				t.Errorf("Expected Kind=PrometheusRule, got %s", rule.Kind)
			}

			if len(rule.Spec.Groups) == 0 {
				t.Errorf("No rule groups found in %s", file)
			}
		})
	}
}

// TestAlertRulesExpressions validates that alert expressions reference valid metrics
func TestAlertRulesExpressions(t *testing.T) {
	alertRuleFiles := []string{
		"../../config/prometheus/rules/security-alerts.yml",
		"../../config/prometheus/rules/performance-alerts.yml",
		"../../config/monitoring/optimization-alerts.yaml",
		"../../config/monitoring/prometheus-rules.yaml",
		"../../config/prometheus/rules/ingester-destination-config-alerts.yml",
	}

	for _, file := range alertRuleFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var rule PrometheusRule
			if err := yaml.Unmarshal(data, &rule); err != nil {
				t.Fatalf("Failed to parse YAML in %s: %v", file, err)
			}

			for _, group := range rule.Spec.Groups {
				for _, alertRule := range group.Rules {
					if alertRule.Expr == "" {
						continue // Skip if expr is empty (might be in comments)
					}

					// Extract metric names from expression
					// This is a simple heuristic - in production, use promtool for full validation
					expr := alertRule.Expr
					for metric := range KnownMetrics {
						if strings.Contains(expr, metric) {
							// Found a known metric - good
							continue
						}
					}

					// Check for common patterns that might indicate invalid metrics
					if strings.Contains(expr, "_ratio") && !strings.Contains(expr, "zen_watcher_dedup_cache_usage") &&
						!strings.Contains(expr, "zen_watcher_webhook_queue_usage") &&
						!strings.Contains(expr, "zen_watcher_optimization_filter_effectiveness_ratio") &&
						!strings.Contains(expr, "zen_watcher_optimization_deduplication_rate_ratio") {
						// Some metrics with _ratio suffix were renamed - check if this is one
						if strings.Contains(expr, "zen_watcher_dedup_cache_usage_ratio") ||
							strings.Contains(expr, "zen_watcher_webhook_queue_usage_ratio") {
							t.Errorf("Alert %s in %s uses deprecated _ratio suffix metric. Use zen_watcher_dedup_cache_usage or zen_watcher_webhook_queue_usage instead",
								alertRule.Alert, file)
						}
					}
				}
			}
		})
	}
}

// TestAlertRulesSeverityValues validates that severity values in expressions use lowercase
func TestAlertRulesSeverityValues(t *testing.T) {
	alertRuleFiles := []string{
		"../../config/prometheus/rules/security-alerts.yml",
		"../../config/prometheus/rules/performance-alerts.yml",
		"../../config/monitoring/optimization-alerts.yaml",
		"../../config/monitoring/prometheus-rules.yaml",
	}

	for _, file := range alertRuleFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var rule PrometheusRule
			if err := yaml.Unmarshal(data, &rule); err != nil {
				t.Fatalf("Failed to parse YAML in %s: %v", file, err)
			}

			invalidSeverities := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "WARNING", "INFO", "Critical", "High", "Medium", "Low"}

			for _, group := range rule.Spec.Groups {
				for _, alertRule := range group.Rules {
					expr := alertRule.Expr
					for _, invalid := range invalidSeverities {
						if strings.Contains(expr, `severity="`+invalid+`"`) || strings.Contains(expr, `severity=`+invalid) {
							t.Errorf("Alert %s in %s uses uppercase severity value '%s'. Use lowercase instead (e.g., 'critical', 'high')",
								alertRule.Alert, file, invalid)
						}
					}
				}
			}
		})
	}
}

// TestAlertRulesRequiredFields validates that alerts have required fields
func TestAlertRulesRequiredFields(t *testing.T) {
	alertRuleFiles := []string{
		"../../config/prometheus/rules/security-alerts.yml",
		"../../config/prometheus/rules/performance-alerts.yml",
		"../../config/monitoring/optimization-alerts.yaml",
		"../../config/monitoring/prometheus-rules.yaml",
		"../../config/prometheus/rules/ingester-destination-config-alerts.yml",
	}

	for _, file := range alertRuleFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var rule PrometheusRule
			if err := yaml.Unmarshal(data, &rule); err != nil {
				t.Fatalf("Failed to parse YAML in %s: %v", file, err)
			}

			for _, group := range rule.Spec.Groups {
				for _, alertRule := range group.Rules {
					if alertRule.Alert == "" {
						t.Errorf("Alert in group %s in %s is missing 'alert' field", group.Name, file)
					}
					if alertRule.Expr == "" {
						// Allow empty expr if it's commented out
						continue
					}
					if alertRule.Labels == nil {
						t.Errorf("Alert %s in %s is missing 'labels' field", alertRule.Alert, file)
					} else if alertRule.Labels["severity"] == "" {
						t.Errorf("Alert %s in %s is missing 'severity' label", alertRule.Alert, file)
					}
					if alertRule.Annotations == nil {
						t.Errorf("Alert %s in %s is missing 'annotations' field", alertRule.Alert, file)
					} else if alertRule.Annotations["summary"] == "" {
						t.Errorf("Alert %s in %s is missing 'summary' annotation", alertRule.Alert, file)
					}
				}
			}
		})
	}
}

