// Copyright 2024 The Zen Watcher Authors
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

package advisor

import (
	"fmt"
	"strings"
)

// SuggestionEngine generates actionable suggestions from opportunities
type SuggestionEngine struct {
	rules []SuggestionRule
}

// SuggestionRule defines how to convert an opportunity into a suggestion
type SuggestionRule struct {
	Type            string
	Title           string
	DescriptionTmpl string
	CommandTmpl     string
	ImpactTmpl      string
	Urgency         string
}

// NewSuggestionEngine creates a new suggestion engine with default rules
func NewSuggestionEngine() *SuggestionEngine {
	return &SuggestionEngine{
		rules: getDefaultRules(),
	}
}

// GenerateSuggestions converts opportunities into actionable suggestions
func (se *SuggestionEngine) GenerateSuggestions(opportunities []Opportunity) []Suggestion {
	suggestions := make([]Suggestion, 0)

	for _, opp := range opportunities {
		rule := se.findRule(opp.Type)
		if rule == nil {
			continue
		}

		suggestion := se.createSuggestion(opp, rule)
		if suggestion != nil {
			suggestions = append(suggestions, *suggestion)
		}
	}

	return suggestions
}

// findRule finds a rule for the given opportunity type
func (se *SuggestionEngine) findRule(oppType string) *SuggestionRule {
	for _, rule := range se.rules {
		if rule.Type == oppType {
			return &rule
		}
	}
	return nil
}

// createSuggestion creates a suggestion from an opportunity and rule
func (se *SuggestionEngine) createSuggestion(opp Opportunity, rule *SuggestionRule) *Suggestion {
	// Calculate expected reduction
	reduction := se.calculateReduction(opp)

	// Format description
	description := se.formatTemplate(rule.DescriptionTmpl, opp, reduction)

	// Format command
	command := se.formatCommand(rule.CommandTmpl, opp, reduction)

	// Format impact
	impact := se.formatTemplate(rule.ImpactTmpl, opp, reduction)

	return &Suggestion{
		Source:      opp.Source,
		Type:        opp.Type,
		Urgency:     rule.Urgency,
		Confidence:  opp.Confidence,
		Title:       rule.Title,
		Description: description,
		Command:     command,
		Impact:      impact,
		Reduction:   reduction,
	}
}

// calculateReduction calculates expected reduction percentage
func (se *SuggestionEngine) calculateReduction(opp Opportunity) float64 {
	switch opp.Type {
	case "high_low_severity":
		// Reduction equals the low severity percentage
		if percent, ok := opp.Metrics["low_severity_percent"].(float64); ok {
			return percent
		}
		return 0.0

	case "low_dedup_effectiveness":
		// Expected improvement: switch to dedup_first could improve effectiveness
		return 0.5 // Conservative estimate: 50% improvement

	case "high_observation_rate":
		// Reduction depends on filter/dedup optimization
		return 0.6 // Estimate: 60% reduction possible

	case "low_filter_pass_rate":
		// Suggests filter is too aggressive - reduction would be negative (increase)
		return -0.3 // -30% means we'd allow 30% more through

	default:
		return 0.0
	}
}

// formatTemplate formats a template string with opportunity data
func (se *SuggestionEngine) formatTemplate(tmpl string, opp Opportunity, reduction float64) string {
	result := tmpl
	result = strings.ReplaceAll(result, "{{.source}}", opp.Source)
	result = strings.ReplaceAll(result, "{{.reduction}}", fmt.Sprintf("%.0f", reduction*100))
	result = strings.ReplaceAll(result, "{{.description}}", opp.Description)
	return result
}

// formatCommand formats a kubectl command template
func (se *SuggestionEngine) formatCommand(tmpl string, opp Opportunity, reduction float64) string {
	result := tmpl
	result = strings.ReplaceAll(result, "{{.source}}", opp.Source)
	result = strings.ReplaceAll(result, "{{.reduction}}", fmt.Sprintf("%.0f", reduction*100))

	// Format minPriority value
	if opp.Type == "high_low_severity" {
		// Suggest minPriority = 0.5 for high low severity
		result = strings.ReplaceAll(result, "{{.minPriority}}", "0.5")
	}

	return result
}

// getDefaultRules returns default suggestion rules
func getDefaultRules() []SuggestionRule {
	return []SuggestionRule{
		{
			Type:            "high_low_severity",
			Title:           "Set filter.minPriority to reduce noise",
			DescriptionTmpl: "{{.description}} - Potential: {{.reduction}}% noise reduction with filter.minPriority=0.5",
			CommandTmpl:     "kubectl patch observationsourceconfig {{.source}} --type=merge -p '{\"spec\":{\"filter\":{\"minPriority\":{{.minPriority}}}}}'",
			ImpactTmpl:      "Expected to reduce observations by {{.reduction}}% by filtering out LOW severity events",
			Urgency:         "medium",
		},
		{
			Type:            "low_dedup_effectiveness",
			Title:           "Switch to dedup_first processing order",
			DescriptionTmpl: "{{.description}} - Consider switching processing order to dedup_first",
			CommandTmpl:     "kubectl patch observationsourceconfig {{.source}} --type=merge -p '{\"spec\":{\"processing\":{\"order\":\"dedup_first\"}}}'",
			ImpactTmpl:      "Expected to improve deduplication effectiveness by processing duplicates earlier",
			Urgency:         "low",
		},
		{
			Type:            "high_observation_rate",
			Title:           "Optimize filter and dedup settings",
			DescriptionTmpl: "{{.description}} - Consider optimizing filter rules or dedup window",
			CommandTmpl:     "kubectl get observationsourceconfig {{.source}} -o yaml",
			ImpactTmpl:      "Review current configuration and optimize to reduce observation rate",
			Urgency:         "high",
		},
		{
			Type:            "low_filter_pass_rate",
			Title:           "Review filter configuration",
			DescriptionTmpl: "{{.description}} - Filter may be too aggressive",
			CommandTmpl:     "kubectl get observationsourceconfig {{.source}} -o yaml",
			ImpactTmpl:      "Consider relaxing filter rules to allow more observations through",
			Urgency:         "low",
		},
	}
}

