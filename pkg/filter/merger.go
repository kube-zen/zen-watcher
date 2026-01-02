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
	"strings"
)

// MergeFilterConfigs merges multiple FilterConfig objects into a single config.
// When multiple filters exist for the same source, they are merged with the following rules:
//
//   - Lists (exclude/include): Union of all lists (deduplicated)
//   - MinSeverity: Most restrictive (highest priority) wins
//   - IncludeSeverity: Intersection (only severities in ALL filters)
//   - Enabled: AND logic (all must be enabled, or explicit false wins)
//
// Order: ConfigMap filters are applied first, then CRD filters are merged on top.
func MergeFilterConfigs(configs ...*FilterConfig) *FilterConfig {
	if len(configs) == 0 {
		return &FilterConfig{
			Sources: make(map[string]SourceFilter),
		}
	}

	if len(configs) == 1 {
		// Single config - return a copy
		result := &FilterConfig{
			Sources: make(map[string]SourceFilter),
		}
		if configs[0] != nil && configs[0].Sources != nil {
			for k, v := range configs[0].Sources {
				result.Sources[k] = v
			}
		}
		return result
	}

	// Multiple configs - merge them
	result := &FilterConfig{
		Sources: make(map[string]SourceFilter),
	}

	for _, config := range configs {
		if config == nil || config.Sources == nil {
			continue
		}

		for sourceName, sourceFilter := range config.Sources {
			sourceName = strings.ToLower(sourceName)
			existing, exists := result.Sources[sourceName]

			if !exists {
				// First filter for this source - copy it
				result.Sources[sourceName] = sourceFilter
				continue
			}

			// Merge existing filter with new filter
			merged := mergeSourceFilters(existing, sourceFilter)
			result.Sources[sourceName] = merged
		}
	}

	return result
}

// mergeSourceFilters merges two SourceFilter objects
func mergeSourceFilters(f1, f2 SourceFilter) SourceFilter {
	result := SourceFilter{}

	// MinSeverity: More restrictive (higher priority) wins
	// Priority: CRITICAL > HIGH > MEDIUM > LOW > UNKNOWN
	result.MinSeverity = moreRestrictiveSeverity(f1.MinSeverity, f2.MinSeverity)

	// IncludeSeverity: Intersection (severities that appear in BOTH)
	result.IncludeSeverity = intersectStringLists(f1.IncludeSeverity, f2.IncludeSeverity)

	// ExcludeEventTypes: Union (deduplicated)
	result.ExcludeEventTypes = unionStringLists(f1.ExcludeEventTypes, f2.ExcludeEventTypes)

	// IncludeEventTypes: Intersection (event types in BOTH)
	result.IncludeEventTypes = intersectStringLists(f1.IncludeEventTypes, f2.IncludeEventTypes)

	// ExcludeNamespaces: Union
	result.ExcludeNamespaces = unionStringLists(f1.ExcludeNamespaces, f2.ExcludeNamespaces)

	// IncludeNamespaces: Intersection
	result.IncludeNamespaces = intersectStringLists(f1.IncludeNamespaces, f2.IncludeNamespaces)

	// ExcludeKinds: Union
	result.ExcludeKinds = unionStringLists(f1.ExcludeKinds, f2.ExcludeKinds)

	// IncludeKinds: Intersection
	result.IncludeKinds = intersectStringLists(f1.IncludeKinds, f2.IncludeKinds)

	// ExcludeCategories: Union
	result.ExcludeCategories = unionStringLists(f1.ExcludeCategories, f2.ExcludeCategories)

	// IncludeCategories: Intersection
	result.IncludeCategories = intersectStringLists(f1.IncludeCategories, f2.IncludeCategories)

	// ExcludeRules: Union
	result.ExcludeRules = unionStringLists(f1.ExcludeRules, f2.ExcludeRules)

	// IgnoreKinds: Union (will be merged into ExcludeKinds by GetSourceFilter)
	result.IgnoreKinds = unionStringLists(f1.IgnoreKinds, f2.IgnoreKinds)

	// Enabled: AND logic (if either is explicitly false, result is false)
	if f1.Enabled != nil && !*f1.Enabled {
		disabled := false
		result.Enabled = &disabled
	} else if f2.Enabled != nil && !*f2.Enabled {
		disabled := false
		result.Enabled = &disabled
	} else if f1.Enabled != nil {
		result.Enabled = f1.Enabled
	} else if f2.Enabled != nil {
		result.Enabled = f2.Enabled
	}
	// If both are nil, result.Enabled is nil (defaults to true)

	return result
}

// moreRestrictiveSeverity returns the more restrictive severity level
// Priority: CRITICAL > HIGH > MEDIUM > LOW > UNKNOWN
func moreRestrictiveSeverity(s1, s2 string) string {
	if s1 == "" {
		return s2
	}
	if s2 == "" {
		return s1
	}

	priority := map[string]int{
		"CRITICAL": 5,
		"HIGH":     4,
		"MEDIUM":   3,
		"LOW":      2,
		"UNKNOWN":  1,
	}

	upperS1 := strings.ToUpper(s1)
	upperS2 := strings.ToUpper(s2)

	p1 := priority[upperS1]
	p2 := priority[upperS2]

	if p1 > p2 {
		return s1
	}
	if p2 > p1 {
		return s2
	}
	// Equal priority - return first one
	return s1
}

// unionStringLists returns a deduplicated union of two string lists (case-insensitive)
func unionStringLists(list1, list2 []string) []string {
	if len(list1) == 0 {
		return list2
	}
	if len(list2) == 0 {
		return list1
	}

	seen := make(map[string]string) // lowercase -> original case
	for _, item := range list1 {
		lower := strings.ToLower(item)
		if _, exists := seen[lower]; !exists {
			seen[lower] = item
		}
	}
	for _, item := range list2 {
		lower := strings.ToLower(item)
		if _, exists := seen[lower]; !exists {
			seen[lower] = item
		}
	}

	result := make([]string, 0, len(seen))
	for _, v := range seen {
		result = append(result, v)
	}
	return result
}

// intersectStringLists returns the intersection of two string lists (case-insensitive)
func intersectStringLists(list1, list2 []string) []string {
	if len(list1) == 0 {
		return list2
	}
	if len(list2) == 0 {
		return list1
	}

	// Build lookup map from list1 (case-insensitive)
	seen := make(map[string]string) // lowercase -> original case
	for _, item := range list1 {
		lower := strings.ToLower(item)
		if _, exists := seen[lower]; !exists {
			seen[lower] = item
		}
	}

	// Find items in list2 that are also in list1
	result := make([]string, 0)
	for _, item := range list2 {
		lower := strings.ToLower(item)
		if orig, exists := seen[lower]; exists {
			result = append(result, orig)
		}
	}

	return result
}
