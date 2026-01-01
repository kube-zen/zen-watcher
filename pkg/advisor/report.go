// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may Obtain a copy of the License at
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
	"time"
)

// WeeklyReport generates a weekly optimization impact report
type WeeklyReport struct {
	StartDate                time.Time
	EndDate                  time.Time
	TotalOptimizations       int
	TotalObservationsReduced int64
	AverageReductionPercent  float64
	TotalCPUSavingsMinutes   float64
	MostEffectiveSource      string
	MostEffectiveAction      string
	SourcesOptimized         []string
	NextOpportunities        []Opportunity
}

// GenerateWeeklyReport generates a weekly report from impact tracker
func (it *ImpactTracker) GenerateWeeklyReport() *WeeklyReport {
	allImpacts := it.GetAllImpacts()

	report := &WeeklyReport{
		StartDate:         time.Now().AddDate(0, 0, -7),
		EndDate:           time.Now(),
		SourcesOptimized:  make([]string, 0),
		NextOpportunities: make([]Opportunity, 0),
	}

	for source, impact := range allImpacts {
		if impact.OptimizationsApplied > 0 {
			report.TotalOptimizations += impact.OptimizationsApplied
			report.TotalObservationsReduced += impact.ObservationsReduced
			report.TotalCPUSavingsMinutes += impact.CPUSavingsMinutes
			report.SourcesOptimized = append(report.SourcesOptimized, source)

			// Track most effective
			if impact.ReductionPercent > report.AverageReductionPercent {
				report.AverageReductionPercent = impact.ReductionPercent
				report.MostEffectiveSource = source
				report.MostEffectiveAction = impact.MostEffective
			}
		}
	}

	// Calculate average
	if len(report.SourcesOptimized) > 0 {
		report.AverageReductionPercent = report.AverageReductionPercent / float64(len(report.SourcesOptimized))
	}

	return report
}

// Format formats the weekly report as a string
func (wr *WeeklyReport) Format() string {
	report := "\n=== Weekly Optimization Report ===\n\n"
	report += fmt.Sprintf("Period: %s to %s\n\n", wr.StartDate.Format("2006-01-02"), wr.EndDate.Format("2006-01-02"))
	report += "Summary:\n"
	report += fmt.Sprintf("  Total Optimizations Applied: %d\n", wr.TotalOptimizations)
	report += fmt.Sprintf("  Total Observations Reduced: %d\n", wr.TotalObservationsReduced)
	report += fmt.Sprintf("  Average Reduction: %.1f%%\n", wr.AverageReductionPercent*100)
	report += fmt.Sprintf("  CPU Savings: %.1f minutes\n", wr.TotalCPUSavingsMinutes)
	report += "\n"
	report += "Most Effective:\n"
	report += fmt.Sprintf("  Source: %s\n", wr.MostEffectiveSource)
	report += fmt.Sprintf("  Action: %s\n", wr.MostEffectiveAction)
	report += "\n"
	report += fmt.Sprintf("Sources Optimized: %d\n", len(wr.SourcesOptimized))
	for _, source := range wr.SourcesOptimized {
		report += fmt.Sprintf("  - %s\n", source)
	}
	report += "\n"

	return report
}
