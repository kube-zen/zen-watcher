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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type LintResult struct {
	File    string  `json:"file"`
	Issues  []Issue `json:"issues"`
	Summary Summary `json:"summary"`
}

type Issue struct {
	Severity string `json:"severity"` // error, warning, info
	Code     string `json:"code"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type Summary struct {
	Total    int `json:"total"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
}

type IngesterSpec struct {
	APIVersion string                 `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                 `yaml:"kind" json:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata" json:"metadata"`
	Spec       map[string]interface{} `yaml:"spec" json:"spec"`
}

var (
	failOnThreshold = flag.String("fail-on", "error", "Fail if issues >= threshold (error, warning, info)")
	outputJSON      = flag.Bool("json", false, "Output JSON format")
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: ingester-lint [-fail-on=error|warning|info] [-json] <file1.yaml> [file2.yaml ...]\n")
		os.Exit(1)
	}

	var allResults []LintResult
	exitCode := 0

	for _, filename := range flag.Args() {
		result := lintFile(filename)
		allResults = append(allResults, result)

		// Determine exit code based on threshold
		if shouldFail(result, *failOnThreshold) {
			exitCode = 1
		}
	}

	if *outputJSON {
		jsonOutput, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonOutput))
	} else {
		for _, result := range allResults {
			printTextOutput(result)
		}
	}

	os.Exit(exitCode)
}

func lintFile(filename string) LintResult {
	data, err := os.ReadFile(filename)
	if err != nil {
		return LintResult{
			File: filename,
			Issues: []Issue{
				{
					Severity: "error",
					Code:     "FILE_READ_ERROR",
					Message:  fmt.Sprintf("Failed to read file: %v", err),
				},
			},
			Summary: Summary{Total: 1, Errors: 1},
		}
	}

	var ingester IngesterSpec
	if err := yaml.Unmarshal(data, &ingester); err != nil {
		return LintResult{
			File: filename,
			Issues: []Issue{
				{
					Severity: "error",
					Code:     "PARSE_ERROR",
					Message:  fmt.Sprintf("Failed to parse YAML: %v", err),
				},
			},
			Summary: Summary{Total: 1, Errors: 1},
		}
	}

	// Skip non-Ingester resources
	if ingester.Kind != "Ingester" || !strings.Contains(ingester.APIVersion, "zen.kube-zen.io") {
		return LintResult{
			File:    filename,
			Issues:  []Issue{},
			Summary: Summary{Total: 0},
		}
	}

	issues := lintIngester(ingester, filename)
	summary := Summary{
		Total:    len(issues),
		Errors:   countBySeverity(issues, "error"),
		Warnings: countBySeverity(issues, "warning"),
		Infos:    countBySeverity(issues, "info"),
	}

	return LintResult{
		File:    filename,
		Issues:  issues,
		Summary: summary,
	}
}

func lintIngester(ingester IngesterSpec, filename string) []Issue {
	var issues []Issue

	spec := ingester.Spec
	if spec == nil {
		return []Issue{
			{
				Severity: "error",
				Code:     "MISSING_SPEC",
				Message:  "Ingester spec is missing",
				Field:    "spec",
			},
		}
	}

	// Check required fields
	issues = append(issues, lintRequiredFields(spec)...)

	// Lint destinations
	destinations, _ := spec["destinations"].([]interface{})
	issues = append(issues, lintDestinations(destinations)...)

	// Dangerous settings checks
	source, _ := spec["source"].(string)
	ingesterType, _ := spec["ingester"].(string)
	issues = append(issues, lintDangerousSettings(spec, source, ingesterType)...)

	// Check normalization hints
	issues = append(issues, lintNormalizationHints(destinations)...)

	return issues
}

func lintRequiredFields(spec map[string]interface{}) []Issue {
	var issues []Issue

	if source, ok := spec["source"].(string); !ok || source == "" {
		issues = append(issues, Issue{
			Severity: "error",
			Code:     "MISSING_SOURCE",
			Message:  "spec.source is required",
			Field:    "spec.source",
		})
	}

	if ingesterType, ok := spec["ingester"].(string); !ok || ingesterType == "" {
		issues = append(issues, Issue{
			Severity: "error",
			Code:     "MISSING_INGESTER",
			Message:  "spec.ingester is required",
			Field:    "spec.ingester",
		})
	}

	destinations, _ := spec["destinations"].([]interface{})
	if len(destinations) == 0 {
		issues = append(issues, Issue{
			Severity: "error",
			Code:     "MISSING_DESTINATIONS",
			Message:  "spec.destinations is required and must have at least one destination",
			Field:    "spec.destinations",
		})
	}

	return issues
}

func lintDestinations(destinations []interface{}) []Issue {
	var issues []Issue

	for i, dest := range destinations {
		destMap, ok := dest.(map[string]interface{})
		if !ok {
			continue
		}

		destType, _ := destMap["type"].(string)
		if destType != "crd" {
			issues = append(issues, Issue{
				Severity: "error",
				Code:     "INVALID_DESTINATION_TYPE",
				Message:  fmt.Sprintf("Destination type '%s' is not supported in v1 (only 'crd' is supported)", destType),
				Field:    fmt.Sprintf("spec.destinations[%d].type", i),
			})
		}

		if destType == "crd" {
			value, _ := destMap["value"].(string)
			if value == "" {
				issues = append(issues, Issue{
					Severity: "error",
					Code:     "MISSING_DESTINATION_VALUE",
					Message:  "Destination value is required for type 'crd'",
					Field:    fmt.Sprintf("spec.destinations[%d].value", i),
				})
			} else if !matchesPattern(value, `^[a-z0-9-]+$`) {
				issues = append(issues, Issue{
					Severity: "error",
					Code:     "INVALID_DESTINATION_VALUE",
					Message:  fmt.Sprintf("Destination value '%s' does not match pattern ^[a-z0-9-]+$", value),
					Field:    fmt.Sprintf("spec.destinations[%d].value", i),
				})
			}
		}
	}

	return issues
}

func lintDangerousSettings(spec map[string]interface{}, source, ingesterType string) []Issue {
	var issues []Issue

	// Check for high-rate sources without filters
	if isHighRateSource(source, ingesterType) {
		filters, ok := spec["filters"].(map[string]interface{})
		if !ok || len(filters) == 0 {
			issues = append(issues, Issue{
				Severity: "error",
				Code:     "NO_FILTERS_HIGH_RATE",
				Message:  fmt.Sprintf("Source '%s' is known for high event rates but has no filters configured", source),
				Field:    "spec.filters",
			})
		}
	}

	// Check for wide matchers
	if informer, ok := spec["informer"].(map[string]interface{}); ok {
		namespace, _ := informer["namespace"].(string)
		labelSelector, _ := informer["labelSelector"].(string)
		fieldSelector, _ := informer["fieldSelector"].(string)

		if namespace == "" && labelSelector == "" && fieldSelector == "" {
			issues = append(issues, Issue{
				Severity: "warning",
				Code:     "WIDE_MATCHER",
				Message:  "Informer has no namespace, label, or field selectors - will watch all resources",
				Field:    "spec.informer",
			})
		}
	}

	// Check for missing deduplication on duplicate-prone sources
	if isDuplicateProneSource(source) {
		dedup, ok := spec["deduplication"].(map[string]interface{})
		enabled, _ := dedup["enabled"].(bool)
		if !ok || !enabled {
			issues = append(issues, Issue{
				Severity: "error",
				Code:     "NO_DEDUP_DUPLICATE_PRONE",
				Message:  fmt.Sprintf("Source '%s' is known for duplicate events but deduplication is disabled or missing", source),
				Field:    "spec.deduplication",
			})
		}
	}

	return issues
}

func lintNormalizationHints(destinations []interface{}) []Issue {
	var issues []Issue

	for i, dest := range destinations {
		destMap, ok := dest.(map[string]interface{})
		if !ok {
			continue
		}

		mapping, ok := destMap["mapping"].(map[string]interface{})
		if !ok {
			continue
		}

		priority, _ := mapping["priority"].(map[string]interface{})
		if len(priority) == 0 {
			issues = append(issues, Issue{
				Severity: "info",
				Code:     "NO_PRIORITY_MAPPING",
				Message:  "No priority mapping configured - events may get default priority",
				Field:    fmt.Sprintf("spec.destinations[%d].mapping.priority", i),
			})
		}

		severityMap, _ := mapping["severityMap"].(map[string]interface{})
		if len(severityMap) == 0 {
			issues = append(issues, Issue{
				Severity: "info",
				Code:     "NO_SEVERITY_MAPPING",
				Message:  "No severity mapping configured - severity may not be normalized correctly",
				Field:    fmt.Sprintf("spec.destinations[%d].mapping.severityMap", i),
			})
		}

		fieldMapping, _ := mapping["fieldMapping"].([]interface{})
		if len(fieldMapping) == 0 {
			issues = append(issues, Issue{
				Severity: "info",
				Code:     "NO_FIELD_MAPPING",
				Message:  "No field mappings configured - source-specific fields may not be extracted",
				Field:    fmt.Sprintf("spec.destinations[%d].mapping.fieldMapping", i),
			})
		}
	}

	return issues
}

func isHighRateSource(source, ingesterType string) bool {
	highRateSources := []string{"k8s-events", "audit", "kubernetes-events"}
	for _, s := range highRateSources {
		if source == s || ingesterType == s {
			return true
		}
	}
	return false
}

func isDuplicateProneSource(source string) bool {
	duplicateProneSources := []string{"cert-manager", "sealed-secrets"}
	for _, s := range duplicateProneSources {
		if source == s {
			return true
		}
	}
	return false
}

func matchesPattern(s, pattern string) bool {
	// Simple pattern matching for ^[a-z0-9-]+$
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

func countBySeverity(issues []Issue, severity string) int {
	count := 0
	for _, issue := range issues {
		if issue.Severity == severity {
			count++
		}
	}
	return count
}

func shouldFail(result LintResult, threshold string) bool {
	switch threshold {
	case "error":
		return result.Summary.Errors > 0
	case "warning":
		return result.Summary.Errors > 0 || result.Summary.Warnings > 0
	case "info":
		return result.Summary.Total > 0
	default:
		return result.Summary.Errors > 0
	}
}

func printTextOutput(result LintResult) {
	if result.Summary.Total == 0 {
		fmt.Printf("✓ %s: No issues found\n", result.File)
		return
	}

	fmt.Printf("%s:\n", result.File)
	for _, issue := range result.Issues {
		severity := strings.ToUpper(issue.Severity)
		fmt.Printf("  [%s] %s: %s", severity, issue.Code, issue.Message)
		if issue.Field != "" {
			fmt.Printf(" (field: %s)", issue.Field)
		}
		fmt.Println()
	}
	fmt.Printf("  Summary: %d total (%d errors, %d warnings, %d infos)\n",
		result.Summary.Total, result.Summary.Errors, result.Summary.Warnings, result.Summary.Infos)
}
