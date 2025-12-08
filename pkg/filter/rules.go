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
	"fmt"
	"strings"
	"sync"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Filter provides filtering functionality for observations
// Thread-safe: config can be updated dynamically via UpdateConfig()
type Filter struct {
	mu     sync.RWMutex
	config *FilterConfig
}

// NewFilter creates a new filter with the given configuration
func NewFilter(config *FilterConfig) *Filter {
	return &Filter{
		config: config,
	}
}

// UpdateConfig updates the filter configuration atomically (thread-safe)
func (f *Filter) UpdateConfig(config *FilterConfig) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// Validate config - ensure Sources map is initialized
	if config != nil && config.Sources == nil {
		config.Sources = make(map[string]SourceFilter)
	}
	f.config = config
	logger.Debug("Filter configuration updated dynamically",
		logger.Fields{
			Component: "filter",
			Operation: "config_update",
		})
}

// getConfig returns the current filter configuration (thread-safe read)
func (f *Filter) getConfig() *FilterConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config
}

// Allow checks if an observation should be allowed based on filter rules
// Returns true if the observation should be processed, false if it should be filtered out
// This is called BEFORE normalization and deduplication
func (f *Filter) Allow(observation *unstructured.Unstructured) bool {
	_, reason := f.AllowWithReason(observation)
	return reason == ""
}

// AllowWithReason checks if an observation should be allowed and returns the reason if filtered
// Returns (true, "") if allowed, (false, reason) if filtered
func (f *Filter) AllowWithReason(observation *unstructured.Unstructured) (bool, string) {
	if f == nil {
		// No filter configured - allow all
		return true, ""
	}

	// Get config atomically (thread-safe read)
	config := f.getConfig()
	if config == nil {
		// No filter configured - allow all
		return true, ""
	}

	// Extract source from observation
	sourceVal, sourceFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	if !sourceFound || sourceVal == nil {
		// No source - allow (will be handled elsewhere)
		return true, ""
	}
	source := strings.ToLower(fmt.Sprintf("%v", sourceVal))

	// Get source-specific filter
	sourceFilter := config.GetSourceFilter(source)
	if sourceFilter == nil {
		// No filter for this source - allow
		return true, ""
	}

	// Check if source is enabled
	if !sourceFilter.IsSourceEnabled() {
		logger.Debug("Source disabled, filtering out observation",
			logger.Fields{
				Component: "filter",
				Operation: "filter_check",
				Source:    source,
				Reason:    "source_disabled",
			})
		return false, "source_disabled"
	}

	// Extract fields from observation for filtering
	severityVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "severity")
	severity := strings.ToUpper(fmt.Sprintf("%v", severityVal))

	eventTypeVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "eventType")
	eventType := fmt.Sprintf("%v", eventTypeVal)

	categoryVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "category")
	category := fmt.Sprintf("%v", categoryVal)

	// Extract namespace from resource or metadata
	namespace := ""
	resourceVal, _, _ := unstructured.NestedMap(observation.Object, "spec", "resource")
	if resourceVal != nil {
		if ns, ok := resourceVal["namespace"].(string); ok && ns != "" {
			namespace = ns
		}
	}
	if namespace == "" {
		namespace, _, _ = unstructured.NestedString(observation.Object, "metadata", "namespace")
	}
	if namespace == "" {
		namespace = "default"
	}

	// Extract kind from resource
	kind := ""
	if resourceVal != nil {
		if k, ok := resourceVal["kind"].(string); ok {
			kind = k
		}
	}

	// Extract rule from details (for sources like Kyverno)
	rule := ""
	detailsVal, _, _ := unstructured.NestedMap(observation.Object, "spec", "details")
	if detailsVal != nil {
		if r, ok := detailsVal["rule"].(string); ok && r != "" {
			rule = r
		}
	}

	// Apply filters

	// 1. IncludeSeverity filter (takes precedence over MinSeverity if set)
	if len(sourceFilter.IncludeSeverity) > 0 {
		allowed := false
		for _, included := range sourceFilter.IncludeSeverity {
			if strings.EqualFold(severity, included) {
				allowed = true
				break
			}
		}
		if !allowed {
			logger.Debug("Severity not in include list, filtering out observation",
				logger.Fields{
					Component: "filter",
					Operation: "filter_check",
					Source:    source,
					Severity:  severity,
					Reason:    "include_severity",
					Additional: map[string]interface{}{
						"include_severity": sourceFilter.IncludeSeverity,
					},
				})
			return false, "include_severity"
		}
	} else if sourceFilter.MinSeverity != "" {
		// MinSeverity filter (only if IncludeSeverity is not set)
		if !f.meetsMinSeverity(severity, sourceFilter.MinSeverity) {
			logger.Debug("Severity below minimum, filtering out observation",
				logger.Fields{
					Component: "filter",
					Operation: "filter_check",
					Source:    source,
					Severity:  severity,
					Reason:    "min_severity",
					Additional: map[string]interface{}{
						"min_severity": sourceFilter.MinSeverity,
					},
				})
			return false, "min_severity"
		}
	}

	// 2. EventType filters
	if len(sourceFilter.ExcludeEventTypes) > 0 {
		for _, excluded := range sourceFilter.ExcludeEventTypes {
			if strings.EqualFold(eventType, excluded) {
				logger.Debug("EventType excluded, filtering out observation",
					logger.Fields{
						Component: "filter",
						Operation: "filter_check",
						Source:    source,
						EventType: eventType,
						Reason:    "exclude_event_type",
					})
				return false, "exclude_event_type"
			}
		}
	}
	if len(sourceFilter.IncludeEventTypes) > 0 {
		allowed := false
		for _, included := range sourceFilter.IncludeEventTypes {
			if strings.EqualFold(eventType, included) {
				allowed = true
				break
			}
		}
		if !allowed {
			logger.Debug("EventType not in include list, filtering out observation",
				logger.Fields{
					Component: "filter",
					Operation: "filter_check",
					Source:    source,
					EventType: eventType,
					Reason:    "include_event_type",
				})
			return false, "include_event_type"
		}
	}

	// 3. Namespace filters
	if len(sourceFilter.ExcludeNamespaces) > 0 {
		for _, excluded := range sourceFilter.ExcludeNamespaces {
			if strings.EqualFold(namespace, excluded) {
				logger.Debug("Namespace excluded, filtering out observation",
					logger.Fields{
						Component: "filter",
						Operation: "filter_check",
						Source:    source,
						Namespace: namespace,
						Reason:    "exclude_namespace",
					})
				return false, "exclude_namespace"
			}
		}
	}
	if len(sourceFilter.IncludeNamespaces) > 0 {
		allowed := false
		for _, included := range sourceFilter.IncludeNamespaces {
			if strings.EqualFold(namespace, included) {
				allowed = true
				break
			}
		}
		if !allowed {
			logger.Debug("Namespace not in include list, filtering out observation",
				logger.Fields{
					Component: "filter",
					Operation: "filter_check",
					Source:    source,
					Namespace: namespace,
					Reason:    "include_namespace",
				})
			return false, "include_namespace"
		}
	}

	// 4. Kind filters
	if len(sourceFilter.ExcludeKinds) > 0 {
		for _, excluded := range sourceFilter.ExcludeKinds {
			if strings.EqualFold(kind, excluded) {
				logger.Debug("Kind excluded, filtering out observation",
					logger.Fields{
						Component:    "filter",
						Operation:    "filter_check",
						Source:       source,
						ResourceKind: kind,
						Reason:       "exclude_kind",
					})
				return false, "exclude_kind"
			}
		}
	}
	if len(sourceFilter.IncludeKinds) > 0 {
		allowed := false
		for _, included := range sourceFilter.IncludeKinds {
			if strings.EqualFold(kind, included) {
				allowed = true
				break
			}
		}
		if !allowed {
			logger.Debug("Kind not in include list, filtering out observation",
				logger.Fields{
					Component:    "filter",
					Operation:    "filter_check",
					Source:       source,
					ResourceKind: kind,
					Reason:       "include_kind",
				})
			return false, "include_kind"
		}
	}

	// 5. Category filters
	if len(sourceFilter.ExcludeCategories) > 0 {
		for _, excluded := range sourceFilter.ExcludeCategories {
			if strings.EqualFold(category, excluded) {
				logger.Debug("Category excluded, filtering out observation",
					logger.Fields{
						Component: "filter",
						Operation: "filter_check",
						Source:    source,
						Additional: map[string]interface{}{
							"category": category,
						},
						Reason: "exclude_category",
					})
				return false, "exclude_category"
			}
		}
	}
	if len(sourceFilter.IncludeCategories) > 0 {
		allowed := false
		for _, included := range sourceFilter.IncludeCategories {
			if strings.EqualFold(category, included) {
				allowed = true
				break
			}
		}
		if !allowed {
			logger.Debug("Category not in include list, filtering out observation",
				logger.Fields{
					Component: "filter",
					Operation: "filter_check",
					Source:    source,
					Additional: map[string]interface{}{
						"category": category,
					},
					Reason: "include_category",
				})
			return false, "include_category"
		}
	}

	// 6. Rule filters (for sources like Kyverno)
	if len(sourceFilter.ExcludeRules) > 0 && rule != "" {
		for _, excluded := range sourceFilter.ExcludeRules {
			if strings.EqualFold(rule, excluded) {
				logger.Debug("Rule excluded, filtering out observation",
					logger.Fields{
						Component: "filter",
						Operation: "filter_check",
						Source:    source,
						Additional: map[string]interface{}{
							"rule": rule,
						},
						Reason: "exclude_rule",
					})
				return false, "exclude_rule"
			}
		}
	}

	// All filters passed
	return true, ""
}

// meetsMinSeverity checks if severity meets the minimum requirement
// Severity levels: CRITICAL > HIGH > MEDIUM > LOW > UNKNOWN
func (f *Filter) meetsMinSeverity(severity, minSeverity string) bool {
	severityLevels := map[string]int{
		"CRITICAL": 5,
		"HIGH":     4,
		"MEDIUM":   3,
		"LOW":      2,
		"UNKNOWN":  1,
	}

	severityUpper := strings.ToUpper(severity)
	minSeverityUpper := strings.ToUpper(minSeverity)

	severityLevel, ok1 := severityLevels[severityUpper]
	minLevel, ok2 := severityLevels[minSeverityUpper]

	if !ok1 || !ok2 {
		// Unknown severity - allow by default (conservative)
		return true
	}

	return severityLevel >= minLevel
}
