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
	"strconv"
	"time"
)

// Opportunity represents an optimization opportunity found by metrics analysis
type Opportunity struct {
	Source      string
	Type        string // high_low_severity, low_dedup_effectiveness, high_observation_rate, etc.
	Severity    string // low, medium, high
	Confidence  float64 // 0.0-1.0
	Metrics     map[string]interface{} // Source-specific metrics
	Description string
}

// Suggestion represents an actionable optimization suggestion
type Suggestion struct {
	Source      string
	Type        string
	Urgency     string // low, medium, high
	Confidence  float64 // 0.0-1.0
	Title       string
	Description string
	Command     string // kubectl command to apply
	Impact      string // Expected impact description
	Reduction   float64 // Expected reduction percentage (0.0-1.0)
}

// FormatForLog formats a suggestion for professional logging
func (s Suggestion) FormatForLog() string {
	urgencyPrefix := ""
	switch s.Urgency {
	case "high":
		urgencyPrefix = "[HIGH PRIORITY]"
	case "medium":
		urgencyPrefix = "[MEDIUM PRIORITY]"
	case "low":
		urgencyPrefix = "[LOW PRIORITY]"
	}

	return fmt.Sprintf("[OPTIMIZATION] %s %s: %s\n"+
		"  Suggestion: %s\n"+
		"  Command: %s\n"+
		"  Confidence: %s\n"+
		"  Expected Impact: %s",
		urgencyPrefix, s.Source, s.Description,
		s.Title, s.Command, formatPercent(s.Confidence), s.Impact)
}

// ImpactMetrics tracks the impact of optimizations
type ImpactMetrics struct {
	Source              string
	OptimizationsApplied int
	ObservationsReduced  int64
	ReductionPercent     float64
	CPUSavingsMinutes    float64
	LastOptimizedAt      time.Time
	MostEffective        string // Description of most effective optimization
}

// ProcessingOrder represents the processing order strategy
type ProcessingOrder string

const (
	ProcessingOrderAuto       ProcessingOrder = "auto"
	ProcessingOrderFilterFirst ProcessingOrder = "filter_first"
	ProcessingOrderDedupFirst ProcessingOrder = "dedup_first"
)

// SourceMetrics represents metrics for a source
type SourceMetrics struct {
	Source                string
	ObservationsPerMinute float64
	LowSeverityPercent    float64
	DedupEffectiveness    float64
	FilterPassRate        float64
	TotalObservations     int64
	FilteredCount          int64
	DedupedCount           int64
	CreatedCount           int64
}

// formatPercent formats a float as a percentage string
func formatPercent(v float64) string {
	return formatFloat(v*100) + "%"
}

// formatFloat formats a float to 1 decimal place
func formatFloat(v float64) string {
	return formatFloatPrec(v, 1)
}

// formatFloatPrec formats a float to specified precision
func formatFloatPrec(v float64, prec int) string {
	return strconv.FormatFloat(v, 'f', prec, 64)
}

