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

package sdk

import (
	"fmt"
	"regexp"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// ValidateIngester validates an Ingester spec
func ValidateIngester(ingester *Ingester) error {
	if ingester == nil {
		return &ValidationError{Field: "ingester", Message: "ingester is nil"}
	}

	// Validate APIVersion
	if ingester.APIVersion != "zen.kube-zen.io/v1" && ingester.APIVersion != "zen.kube-zen.io/v1alpha1" {
		return &ValidationError{Field: "apiVersion", Message: "must be zen.kube-zen.io/v1 or zen.kube-zen.io/v1alpha1"}
	}

	// Validate Kind
	if ingester.Kind != "Ingester" {
		return &ValidationError{Field: "kind", Message: "must be 'Ingester'"}
	}

	spec := ingester.Spec

	// Validate required fields
	if spec.Source == "" {
		return &ValidationError{Field: "spec.source", Message: "is required"}
	}

	if !matchesPattern(spec.Source, `^[a-z0-9-]+$`) {
		return &ValidationError{Field: "spec.source", Message: "must match pattern ^[a-z0-9-]+$"}
	}

	if spec.Ingester == "" {
		return &ValidationError{Field: "spec.ingester", Message: "is required"}
	}

	validIngesterTypes := map[string]bool{
		"informer":   true,
		"webhook":     true,
		"logs":        true,
		"k8s-events": true,
	}
	if !validIngesterTypes[spec.Ingester] {
		return &ValidationError{Field: "spec.ingester", Message: fmt.Sprintf("must be one of: informer, webhook, logs, k8s-events (got: %s)", spec.Ingester)}
	}

	if len(spec.Destinations) == 0 {
		return &ValidationError{Field: "spec.destinations", Message: "must have at least one destination"}
	}

	// Validate destinations
	for i, dest := range spec.Destinations {
		if dest.Type != "crd" {
			return &ValidationError{Field: fmt.Sprintf("spec.destinations[%d].type", i), Message: "only 'crd' is supported in v1"}
		}

		if dest.Value == "" {
			return &ValidationError{Field: fmt.Sprintf("spec.destinations[%d].value", i), Message: "is required for type 'crd'"}
		}

		if !matchesPattern(dest.Value, `^[a-z0-9-]+$`) {
			return &ValidationError{Field: fmt.Sprintf("spec.destinations[%d].value", i), Message: "must match pattern ^[a-z0-9-]+$"}
		}
	}

	// Validate deduplication if present
	if spec.Deduplication != nil {
		if err := validateDeduplication(spec.Deduplication); err != nil {
			return err
		}
	}

	// Validate filters if present
	if spec.Filters != nil {
		if err := validateFilters(spec.Filters); err != nil {
			return err
		}
	}

	return nil
}

// ValidateObservation validates an Observation spec
func ValidateObservation(obs *Observation) error {
	if obs == nil {
		return &ValidationError{Field: "observation", Message: "observation is nil"}
	}

	// Validate APIVersion
	if obs.APIVersion != "zen.kube-zen.io/v1" {
		return &ValidationError{Field: "apiVersion", Message: "must be zen.kube-zen.io/v1"}
	}

	// Validate Kind
	if obs.Kind != "Observation" {
		return &ValidationError{Field: "kind", Message: "must be 'Observation'"}
	}

	spec := obs.Spec

	// Validate required fields
	if spec.Source == "" {
		return &ValidationError{Field: "spec.source", Message: "is required"}
	}

	if !matchesPattern(spec.Source, `^[a-z0-9-]+$`) {
		return &ValidationError{Field: "spec.source", Message: "must match pattern ^[a-z0-9-]+$"}
	}

	if spec.Category == "" {
		return &ValidationError{Field: "spec.category", Message: "is required"}
	}

	validCategories := map[string]bool{
		"security":    true,
		"compliance":  true,
		"performance": true,
		"operations":  true,
		"cost":        true,
	}
	if !validCategories[spec.Category] {
		return &ValidationError{Field: "spec.category", Message: fmt.Sprintf("must be one of: security, compliance, performance, operations, cost (got: %s)", spec.Category)}
	}

	if spec.Severity == "" {
		return &ValidationError{Field: "spec.severity", Message: "is required"}
	}

	validSeverities := map[string]bool{
		"critical": true,
		"high":     true,
		"medium":   true,
		"low":      true,
		"info":     true,
	}
	if !validSeverities[spec.Severity] {
		return &ValidationError{Field: "spec.severity", Message: fmt.Sprintf("must be one of: critical, high, medium, low, info (got: %s)", spec.Severity)}
	}

	if spec.EventType == "" {
		return &ValidationError{Field: "spec.eventType", Message: "is required"}
	}

	if !matchesPattern(spec.EventType, `^[a-z0-9_]+$`) {
		return &ValidationError{Field: "spec.eventType", Message: "must match pattern ^[a-z0-9_]+$"}
	}

	// Validate priority if present (legacy field, but may still be in some Observations)
	// Note: v1 Observations don't have priority, but we validate for compatibility

	// Validate TTL if present
	if spec.TTLSecondsAfterCreation != nil {
		ttl := *spec.TTLSecondsAfterCreation
		if ttl < 1 {
			return &ValidationError{Field: "spec.ttlSecondsAfterCreation", Message: "must be >= 1"}
		}
		if ttl > 31536000 {
			return &ValidationError{Field: "spec.ttlSecondsAfterCreation", Message: "must be <= 31536000 (1 year)"}
		}
	}

	return nil
}

func validateDeduplication(dedup *DeduplicationConfig) error {
	if dedup.Strategy != "" {
		validStrategies := map[string]bool{
			"fingerprint": true,
			"key":         true,
			"hybrid":      true,
			"adaptive":    true,
		}
		if !validStrategies[dedup.Strategy] {
			return &ValidationError{Field: "spec.deduplication.strategy", Message: fmt.Sprintf("must be one of: fingerprint, key, hybrid, adaptive (got: %s)", dedup.Strategy)}
		}
	}

	if dedup.LearningRate != nil {
		lr := *dedup.LearningRate
		if lr < 0 || lr > 1 {
			return &ValidationError{Field: "spec.deduplication.learningRate", Message: "must be between 0.0 and 1.0"}
		}
	}

	if dedup.MinChange != nil {
		mc := *dedup.MinChange
		if mc < 0 || mc > 1 {
			return &ValidationError{Field: "spec.deduplication.minChange", Message: "must be between 0.0 and 1.0"}
		}
	}

	if dedup.WindowSeconds != nil {
		ws := *dedup.WindowSeconds
		if ws < 1 || ws > 31536000 {
			return &ValidationError{Field: "spec.deduplication.windowSeconds", Message: "must be between 1 and 31536000"}
		}
	}

	if dedup.Window != "" {
		if !matchesPattern(dedup.Window, `^[0-9]+(ns|us|µs|ms|s|m|h)$`) {
			return &ValidationError{Field: "spec.deduplication.window", Message: "must match pattern ^[0-9]+(ns|us|µs|ms|s|m|h)$"}
		}
	}

	return nil
}

func validateFilters(filters *FilterConfig) error {
	if filters.MinPriority != nil {
		mp := *filters.MinPriority
		if mp < 0 || mp > 1 {
			return &ValidationError{Field: "spec.filters.minPriority", Message: "must be between 0.0 and 1.0"}
		}
	}

	if filters.MinSeverity != "" {
		validSeverities := map[string]bool{
			"CRITICAL": true,
			"HIGH":     true,
			"MEDIUM":   true,
			"LOW":      true,
			"UNKNOWN":  true,
		}
		if !validSeverities[filters.MinSeverity] {
			return &ValidationError{Field: "spec.filters.minSeverity", Message: fmt.Sprintf("must be one of: CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN (got: %s)", filters.MinSeverity)}
		}
	}

	return nil
}

func matchesPattern(s, pattern string) bool {
	matched, err := regexp.MatchString(pattern, s)
	return err == nil && matched
}

