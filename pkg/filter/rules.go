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
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Filter provides filtering functionality for observations
// Thread-safe: config can be updated dynamically via UpdateConfig()
type Filter struct {
	mu      sync.RWMutex
	config  *FilterConfig
	metrics *metrics.Metrics // Optional metrics
}

// NewFilter creates a new filter with the given configuration
func NewFilter(config *FilterConfig) *Filter {
	return NewFilterWithMetrics(config, nil)
}

// NewFilterWithMetrics creates a new filter with metrics support
func NewFilterWithMetrics(config *FilterConfig, m *metrics.Metrics) *Filter {
	return &Filter{
		config:  config,
		metrics: m,
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
	logger := sdklog.NewLogger("zen-watcher-filter")
	logger.Debug("Filter configuration updated dynamically",
		sdklog.Operation("config_update"))
}

// getConfig returns the current filter configuration (thread-safe read)
func (f *Filter) getConfig() *FilterConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config
}

// GetConfig returns a copy of the current filter configuration (thread-safe read)
// This is exported for use by external components that need to merge configs
func (f *Filter) GetConfig() *FilterConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.config == nil {
		return &FilterConfig{
			Sources: make(map[string]SourceFilter),
		}
	}
	// Return a shallow copy (caller should deep copy if modifying)
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
	startTime := time.Now()

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

	// Track evaluation latency
	defer func() {
		if f.metrics != nil {
			// Extract source for metrics
			sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
			source := "unknown"
			if sourceVal != nil {
				source = strings.ToLower(fmt.Sprintf("%v", sourceVal))
			}
			// Track evaluation duration (use "total" as rule_type for overall evaluation)
			duration := time.Since(startTime).Seconds()
			f.metrics.FilterRuleEvaluationDuration.WithLabelValues(source, "total").Observe(duration)
		}
	}()

	// Check if expression-based filtering is enabled
	if allowed, reason := f.checkExpressionFilter(config, observation); reason != "" {
		return allowed, reason
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
		if f.metrics != nil {
			f.metrics.FilterDecisions.WithLabelValues(source, "filter", "source_disabled").Inc()
		}
		logger := sdklog.NewLogger("zen-watcher-filter")
		logger.Debug("Source disabled, filtering out observation",
			sdklog.Operation("filter_check"),
			sdklog.String("source", source),
			sdklog.String("reason", "source_disabled"))
		return false, "source_disabled"
	}

	// Extract fields from observation for filtering
	fields := f.extractObservationFields(observation)

	// Apply global namespace filtering first (if enabled)
	if allowed, reason := f.checkGlobalNamespaceFilter(config, fields.namespace, source); !allowed {
		return false, reason
	}

	// Apply all source-specific filters
	if allowed, reason := f.checkSeverityFilter(sourceFilter, fields.severity, source); !allowed {
		return false, reason
	}
	if allowed, reason := f.checkEventTypeFilter(sourceFilter, fields.eventType, source); !allowed {
		return false, reason
	}
	if allowed, reason := f.checkNamespaceFilter(sourceFilter, fields.namespace, source); !allowed {
		return false, reason
	}
	if allowed, reason := f.checkKindFilter(sourceFilter, fields.kind, source); !allowed {
		return false, reason
	}
	if allowed, reason := f.checkCategoryFilter(sourceFilter, fields.category, source); !allowed {
		return false, reason
	}
	if allowed, reason := f.checkRuleFilter(sourceFilter, fields.rule, source); !allowed {
		return false, reason
	}

	// All filters passed
	if f.metrics != nil {
		f.metrics.FilterDecisions.WithLabelValues(source, "allow", "all_passed").Inc()
	}
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

// observationFields holds extracted fields from an observation
type observationFields struct {
	severity  string
	eventType string
	category  string
	namespace string
	kind      string
	rule      string
}

// extractObservationFields extracts fields from observation for filtering
func (f *Filter) extractObservationFields(observation *unstructured.Unstructured) observationFields {
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

	return observationFields{
		severity:  severity,
		eventType: eventType,
		category:  category,
		namespace: namespace,
		kind:      kind,
		rule:      rule,
	}
}

// checkGlobalNamespaceFilter checks global namespace filter
func (f *Filter) checkGlobalNamespaceFilter(config *FilterConfig, namespace, source string) (bool, string) {
	if config.GlobalNamespaceFilter == nil || !config.GlobalNamespaceFilter.Enabled {
		return true, ""
	}
	globalFilter := config.GlobalNamespaceFilter
	// Check excluded namespaces
	if len(globalFilter.ExcludedNamespaces) > 0 {
		for _, excluded := range globalFilter.ExcludedNamespaces {
			if strings.EqualFold(namespace, excluded) {
				if f.metrics != nil {
					f.metrics.FilterDecisions.WithLabelValues(source, "filter", "global_exclude_namespace").Inc()
				}
				logger := sdklog.NewLogger("zen-watcher-filter")
				logger.Debug("Namespace excluded by global filter",
					sdklog.Operation("filter_check"),
					sdklog.String("source", source),
					sdklog.String("namespace", namespace),
					sdklog.String("reason", "global_exclude_namespace"))
				return false, "global_exclude_namespace"
			}
		}
	}
	// Check included namespaces (if set, only these are allowed)
	if len(globalFilter.IncludedNamespaces) > 0 {
		allowed := false
		for _, included := range globalFilter.IncludedNamespaces {
			if strings.EqualFold(namespace, included) {
				allowed = true
				break
			}
		}
		if !allowed {
			if f.metrics != nil {
				f.metrics.FilterDecisions.WithLabelValues(source, "filter", "global_include_namespace").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-filter")
			logger.Debug("Namespace not in global include list",
				sdklog.Operation("filter_check"),
				sdklog.String("source", source),
				sdklog.String("namespace", namespace),
				sdklog.String("reason", "global_include_namespace"))
			return false, "global_include_namespace"
		}
	}
	return true, ""
}

// checkSeverityFilter checks severity filters
func (f *Filter) checkSeverityFilter(sourceFilter *SourceFilter, severity, source string) (bool, string) {
	if len(sourceFilter.IncludeSeverity) > 0 {
		allowed := false
		for _, included := range sourceFilter.IncludeSeverity {
			if strings.EqualFold(severity, included) {
				allowed = true
				break
			}
		}
		if !allowed {
			if f.metrics != nil {
				f.metrics.FilterDecisions.WithLabelValues(source, "filter", "include_severity").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-filter")
			logger.Debug("Severity not in include list, filtering out observation",
				sdklog.Operation("filter_check"),
				sdklog.String("source", source),
				sdklog.String("severity", severity),
				sdklog.String("reason", "include_severity"),
				sdklog.Strings("include_severity", sourceFilter.IncludeSeverity))
			return false, "include_severity"
		}
	} else if sourceFilter.MinSeverity != "" {
		if !f.meetsMinSeverity(severity, sourceFilter.MinSeverity) {
			if f.metrics != nil {
				f.metrics.FilterDecisions.WithLabelValues(source, "filter", "min_severity").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-filter")
			logger.Debug("Severity below minimum, filtering out observation",
				sdklog.Operation("filter_check"),
				sdklog.String("source", source),
				sdklog.String("severity", severity),
				sdklog.String("reason", "min_severity"),
				sdklog.String("min_severity", sourceFilter.MinSeverity))
			return false, "min_severity"
		}
	}
	return true, ""
}

// checkEventTypeFilter checks event type filters
func (f *Filter) checkEventTypeFilter(sourceFilter *SourceFilter, eventType, source string) (bool, string) {
	if len(sourceFilter.ExcludeEventTypes) > 0 {
		for _, excluded := range sourceFilter.ExcludeEventTypes {
			if strings.EqualFold(eventType, excluded) {
				if f.metrics != nil {
					f.metrics.FilterDecisions.WithLabelValues(source, "filter", "exclude_event_type").Inc()
				}
				logger := sdklog.NewLogger("zen-watcher-filter")
				logger.Debug("EventType excluded, filtering out observation",
					sdklog.Operation("filter_check"),
					sdklog.String("source", source),
					sdklog.String("event_type", eventType),
					sdklog.String("reason", "exclude_event_type"))
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
			if f.metrics != nil {
				f.metrics.FilterDecisions.WithLabelValues(source, "filter", "include_event_type").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-filter")
			logger.Debug("EventType not in include list, filtering out observation",
				sdklog.Operation("filter_check"),
				sdklog.String("source", source),
				sdklog.String("event_type", eventType),
				sdklog.String("reason", "include_event_type"))
			return false, "include_event_type"
		}
	}
	return true, ""
}

// checkNamespaceFilter checks namespace filters
func (f *Filter) checkNamespaceFilter(sourceFilter *SourceFilter, namespace, source string) (bool, string) {
	if len(sourceFilter.ExcludeNamespaces) > 0 {
		for _, excluded := range sourceFilter.ExcludeNamespaces {
			if strings.EqualFold(namespace, excluded) {
				if f.metrics != nil {
					f.metrics.FilterDecisions.WithLabelValues(source, "filter", "exclude_namespace").Inc()
				}
				logger := sdklog.NewLogger("zen-watcher-filter")
				logger.Debug("Namespace excluded, filtering out observation",
					sdklog.Operation("filter_check"),
					sdklog.String("source", source),
					sdklog.String("namespace", namespace),
					sdklog.String("reason", "exclude_namespace"))
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
			if f.metrics != nil {
				f.metrics.FilterDecisions.WithLabelValues(source, "filter", "include_namespace").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-filter")
			logger.Debug("Namespace not in include list, filtering out observation",
				sdklog.Operation("filter_check"),
				sdklog.String("source", source),
				sdklog.String("namespace", namespace),
				sdklog.String("reason", "include_namespace"))
			return false, "include_namespace"
		}
	}
	return true, ""
}

// checkKindFilter checks kind filters
func (f *Filter) checkKindFilter(sourceFilter *SourceFilter, kind, source string) (bool, string) {
	if len(sourceFilter.ExcludeKinds) > 0 {
		for _, excluded := range sourceFilter.ExcludeKinds {
			if strings.EqualFold(kind, excluded) {
				if f.metrics != nil {
					f.metrics.FilterDecisions.WithLabelValues(source, "filter", "exclude_kind").Inc()
				}
				logger := sdklog.NewLogger("zen-watcher-filter")
				logger.Debug("Kind excluded, filtering out observation",
					sdklog.Operation("filter_check"),
					sdklog.String("source", source),
					sdklog.String("resource_kind", kind),
					sdklog.String("reason", "exclude_kind"))
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
			if f.metrics != nil {
				f.metrics.FilterDecisions.WithLabelValues(source, "filter", "include_kind").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-filter")
			logger.Debug("Kind not in include list, filtering out observation",
				sdklog.Operation("filter_check"),
				sdklog.String("source", source),
				sdklog.String("resource_kind", kind),
				sdklog.String("reason", "include_kind"))
			return false, "include_kind"
		}
	}
	return true, ""
}

// checkCategoryFilter checks category filters
func (f *Filter) checkCategoryFilter(sourceFilter *SourceFilter, category, source string) (bool, string) {
	if len(sourceFilter.ExcludeCategories) > 0 {
		for _, excluded := range sourceFilter.ExcludeCategories {
			if strings.EqualFold(category, excluded) {
				if f.metrics != nil {
					f.metrics.FilterDecisions.WithLabelValues(source, "filter", "exclude_category").Inc()
				}
				logger := sdklog.NewLogger("zen-watcher-filter")
				logger.Debug("Category excluded, filtering out observation",
					sdklog.Operation("filter_check"),
					sdklog.String("source", source),
					sdklog.String("category", category),
					sdklog.String("reason", "exclude_category"))
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
			if f.metrics != nil {
				f.metrics.FilterDecisions.WithLabelValues(source, "filter", "include_category").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-filter")
			logger.Debug("Category not in include list, filtering out observation",
				sdklog.Operation("filter_check"),
				sdklog.String("source", source),
				sdklog.String("category", category),
				sdklog.String("reason", "include_category"))
			return false, "include_category"
		}
	}
	return true, ""
}

// checkRuleFilter checks rule filters
func (f *Filter) checkRuleFilter(sourceFilter *SourceFilter, rule, source string) (bool, string) {
	if len(sourceFilter.ExcludeRules) > 0 && rule != "" {
		for _, excluded := range sourceFilter.ExcludeRules {
			if strings.EqualFold(rule, excluded) {
				if f.metrics != nil {
					f.metrics.FilterDecisions.WithLabelValues(source, "filter", "exclude_rule").Inc()
				}
				logger := sdklog.NewLogger("zen-watcher-filter")
				logger.Debug("Rule excluded, filtering out observation",
					sdklog.Operation("filter_check"),
					sdklog.String("source", source),
					sdklog.String("rule", rule),
					sdklog.String("reason", "exclude_rule"))
				return false, "exclude_rule"
			}
		}
	}
	return true, ""
}

// checkExpressionFilter checks expression-based filtering
func (f *Filter) checkExpressionFilter(config *FilterConfig, observation *unstructured.Unstructured) (bool, string) {
	if config.Expression == "" {
		return true, "" // No expression, continue to list-based filtering
	}

	exprFilter, err := NewExpressionFilter(config.Expression)
	if err != nil {
		logger := sdklog.NewLogger("zen-watcher-filter")
		logger.Debug("Failed to parse filter expression, falling back to list-based filters",
			sdklog.Operation("filter_check"),
			sdklog.String("reason", "expression_parse_error"),
			sdklog.String("error", err.Error()))
		return true, "" // Fall through to legacy filtering
	}

	result, err := exprFilter.Evaluate(observation)
	if err != nil {
		logger := sdklog.NewLogger("zen-watcher-filter")
		logger.Debug("Failed to evaluate filter expression, falling back to list-based filters",
			sdklog.Operation("filter_check"),
			sdklog.String("reason", "expression_eval_error"),
			sdklog.String("error", err.Error()))
		return true, "" // Fall through to list-based filtering
	}

	// Extract source for metrics
	sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	source := "unknown"
	if sourceVal != nil {
		source = strings.ToLower(fmt.Sprintf("%v", sourceVal))
	}

	if !result {
		if f.metrics != nil {
			f.metrics.FilterDecisions.WithLabelValues(source, "filter", "expression_filtered").Inc()
		}
		return false, "expression_filtered"
	}

	if f.metrics != nil {
		f.metrics.FilterDecisions.WithLabelValues(source, "allow", "expression_passed").Inc()
	}
	return true, ""
}
