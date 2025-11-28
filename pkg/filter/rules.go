package filter

import (
	"fmt"
	"log"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Filter provides filtering functionality for observations
type Filter struct {
	config *FilterConfig
}

// NewFilter creates a new filter with the given configuration
func NewFilter(config *FilterConfig) *Filter {
	return &Filter{
		config: config,
	}
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
	if f == nil || f.config == nil {
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
	sourceFilter := f.config.GetSourceFilter(source)
	if sourceFilter == nil {
		// No filter for this source - allow
		return true, ""
	}

	// Check if source is enabled
	if !sourceFilter.IsSourceEnabled() {
		log.Printf("  🚫 [FILTER] Source '%s' is disabled, filtering out observation", source)
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

	// Apply filters

	// 1. MinSeverity filter
	if sourceFilter.MinSeverity != "" {
		if !f.meetsMinSeverity(severity, sourceFilter.MinSeverity) {
			log.Printf("  🚫 [FILTER] Source '%s': severity '%s' below minimum '%s'", source, severity, sourceFilter.MinSeverity)
			return false, "min_severity"
		}
	}

	// 2. EventType filters
	if len(sourceFilter.ExcludeEventTypes) > 0 {
		for _, excluded := range sourceFilter.ExcludeEventTypes {
			if strings.EqualFold(eventType, excluded) {
				log.Printf("  🚫 [FILTER] Source '%s': eventType '%s' is excluded", source, eventType)
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
			log.Printf("  🚫 [FILTER] Source '%s': eventType '%s' not in include list", source, eventType)
			return false, "include_event_type"
		}
	}

	// 3. Namespace filters
	if len(sourceFilter.ExcludeNamespaces) > 0 {
		for _, excluded := range sourceFilter.ExcludeNamespaces {
			if strings.EqualFold(namespace, excluded) {
				log.Printf("  🚫 [FILTER] Source '%s': namespace '%s' is excluded", source, namespace)
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
			log.Printf("  🚫 [FILTER] Source '%s': namespace '%s' not in include list", source, namespace)
			return false, "include_namespace"
		}
	}

	// 4. Kind filters
	if len(sourceFilter.ExcludeKinds) > 0 {
		for _, excluded := range sourceFilter.ExcludeKinds {
			if strings.EqualFold(kind, excluded) {
				log.Printf("  🚫 [FILTER] Source '%s': kind '%s' is excluded", source, kind)
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
			log.Printf("  🚫 [FILTER] Source '%s': kind '%s' not in include list", source, kind)
			return false, "include_kind"
		}
	}

	// 5. Category filters
	if len(sourceFilter.ExcludeCategories) > 0 {
		for _, excluded := range sourceFilter.ExcludeCategories {
			if strings.EqualFold(category, excluded) {
				log.Printf("  🚫 [FILTER] Source '%s': category '%s' is excluded", source, category)
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
			log.Printf("  🚫 [FILTER] Source '%s': category '%s' not in include list", source, category)
			return false, "include_category"
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
