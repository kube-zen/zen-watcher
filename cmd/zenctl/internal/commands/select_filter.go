package commands

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// SelectPattern represents a parsed select pattern
type SelectPattern struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
}

// ParseSelectPattern parses a select pattern into its components
// Supports formats:
//   - Kind/name
//   - Kind/namespace/name
//   - Group/Kind/name
//   - Group/Kind/namespace/name
func ParseSelectPattern(pattern string) (*SelectPattern, error) {
	parts := strings.Split(pattern, "/")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, fmt.Errorf("invalid select pattern format: %s (expected Kind/name, Kind/namespace/name, Group/Kind/name, or Group/Kind/namespace/name)", pattern)
	}

	sp := &SelectPattern{}

	if len(parts) == 2 {
		// Kind/name
		sp.Kind = parts[0]
		sp.Name = parts[1]
	} else if len(parts) == 3 {
		// Could be Kind/namespace/name or Group/Kind/name
		// We'll check if the first part contains a dot (group indicator)
		if strings.Contains(parts[0], ".") {
			// Group/Kind/name
			sp.Group = parts[0]
			sp.Kind = parts[1]
			sp.Name = parts[2]
		} else {
			// Kind/namespace/name
			sp.Kind = parts[0]
			sp.Namespace = parts[1]
			sp.Name = parts[2]
		}
	} else if len(parts) == 4 {
		// Group/Kind/namespace/name
		sp.Group = parts[0]
		sp.Kind = parts[1]
		sp.Namespace = parts[2]
		sp.Name = parts[3]
	}

	return sp, nil
}

// NormalizeSelectPatterns normalizes and sorts select patterns for deterministic evaluation
func NormalizeSelectPatterns(patterns []string) ([]SelectPattern, []string, error) {
	normalized := make([]SelectPattern, 0, len(patterns))
	var errors []string

	for _, pattern := range patterns {
		parsed, err := ParseSelectPattern(pattern)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		normalized = append(normalized, *parsed)
	}

	// Sort for deterministic evaluation
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Group != normalized[j].Group {
			return normalized[i].Group < normalized[j].Group
		}
		if normalized[i].Kind != normalized[j].Kind {
			return normalized[i].Kind < normalized[j].Kind
		}
		if normalized[i].Namespace != normalized[j].Namespace {
			return normalized[i].Namespace < normalized[j].Namespace
		}
		return normalized[i].Name < normalized[j].Name
	})

	if len(errors) > 0 {
		return normalized, errors, fmt.Errorf("parse errors: %s", strings.Join(errors, "; "))
	}

	return normalized, nil, nil
}

// MatchesSelect checks if an unstructured object matches a select pattern
func MatchesSelect(obj *unstructured.Unstructured, pattern SelectPattern) bool {
	// Check group
	if pattern.Group != "" {
		gvk := obj.GroupVersionKind()
		if gvk.Group != pattern.Group {
			return false
		}
	}

	// Check kind
	if pattern.Kind != "" && obj.GetKind() != pattern.Kind {
		return false
	}

	// Check namespace
	if pattern.Namespace != "" && obj.GetNamespace() != pattern.Namespace {
		return false
	}

	// Check name
	if pattern.Name != "" && obj.GetName() != pattern.Name {
		return false
	}

	return true
}

// MatchesAnySelect checks if an object matches any of the select patterns
func MatchesAnySelect(obj *unstructured.Unstructured, patterns []SelectPattern) bool {
	for _, pattern := range patterns {
		if MatchesSelect(obj, pattern) {
			return true
		}
	}
	return false
}

// FilterObjectsBySelect filters objects based on select patterns
func FilterObjectsBySelect(objects []*unstructured.Unstructured, patterns []SelectPattern) ([]*unstructured.Unstructured, []string) {
	if len(patterns) == 0 {
		return objects, nil
	}

	filtered := make([]*unstructured.Unstructured, 0)
	matchedPatterns := make(map[string]bool)

	for _, obj := range objects {
		for _, pattern := range patterns {
			if MatchesSelect(obj, pattern) {
				filtered = append(filtered, obj)
				matchedPatterns[patternKey(pattern)] = true
				break // Only add once
			}
		}
	}

	// Generate warnings for unmatched patterns
	var warnings []string
	for _, pattern := range patterns {
		key := patternKey(pattern)
		if !matchedPatterns[key] {
			warnings = append(warnings, fmt.Sprintf("select pattern matched no resources: %s", formatPattern(pattern)))
		}
	}

	return filtered, warnings
}

// FilterObjectsByLabelSelector filters objects based on label selector
func FilterObjectsByLabelSelector(objects []*unstructured.Unstructured, selectorStr string) ([]*unstructured.Unstructured, error) {
	if selectorStr == "" {
		return objects, nil
	}

	selector, err := labels.Parse(selectorStr)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %w", err)
	}

	filtered := make([]*unstructured.Unstructured, 0)
	for _, obj := range objects {
		objLabels := labels.Set(obj.GetLabels())
		if selector.Matches(objLabels) {
			filtered = append(filtered, obj)
		}
	}

	return filtered, nil
}

// patternKey generates a canonical key for a pattern (for tracking matches)
func patternKey(pattern SelectPattern) string {
	parts := []string{}
	if pattern.Group != "" {
		parts = append(parts, pattern.Group)
	}
	if pattern.Kind != "" {
		parts = append(parts, pattern.Kind)
	}
	if pattern.Namespace != "" {
		parts = append(parts, pattern.Namespace)
	}
	if pattern.Name != "" {
		parts = append(parts, pattern.Name)
	}
	return strings.Join(parts, "/")
}

// formatPattern formats a pattern for display in warnings
func formatPattern(pattern SelectPattern) string {
	parts := []string{}
	if pattern.Group != "" {
		parts = append(parts, pattern.Group)
	}
	if pattern.Kind != "" {
		parts = append(parts, pattern.Kind)
	}
	if pattern.Namespace != "" {
		parts = append(parts, pattern.Namespace)
	}
	if pattern.Name != "" {
		parts = append(parts, pattern.Name)
	}
	return strings.Join(parts, "/")
}

