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

package logging

import (
	"fmt"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/advisor"
	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// OptimizationLogger provides professional logging for optimization events
type OptimizationLogger struct {
	summaryInterval time.Duration
	lastSummary     time.Time
}

// NewOptimizationLogger creates a new optimization logger
func NewOptimizationLogger(summaryInterval time.Duration) *OptimizationLogger {
	return &OptimizationLogger{
		summaryInterval: summaryInterval,
		lastSummary:     time.Now(),
	}
}

// LogSuggestion logs an optimization suggestion
func (ol *OptimizationLogger) LogSuggestion(suggestion advisor.Suggestion) {
	if suggestion.Confidence < 0.7 {
		// Only log high-confidence suggestions
		return
	}

	urgencyPrefix := ""
	switch suggestion.Urgency {
	case "high":
		urgencyPrefix = "[HIGH PRIORITY]"
	case "medium":
		urgencyPrefix = "[MEDIUM PRIORITY]"
	case "low":
		urgencyPrefix = "[LOW PRIORITY]"
	}

	logger.Info("Optimization suggestion generated",
		logger.Fields{
			Component: "optimization",
			Operation: "suggestion",
			Source:    suggestion.Source,
			Additional: map[string]interface{}{
				"urgency":     urgencyPrefix,
				"type":        suggestion.Type,
				"confidence":  fmt.Sprintf("%.0f%%", suggestion.Confidence*100),
				"title":       suggestion.Title,
				"description": suggestion.Description,
				"command":     suggestion.Command,
				"impact":      suggestion.Impact,
				"reduction":   fmt.Sprintf("%.0f%%", suggestion.Reduction*100),
			},
		})
}

// LogSummary logs a periodic optimization summary
func (ol *OptimizationLogger) LogSummary(source string, stats map[string]interface{}) {
	now := time.Now()
	if now.Sub(ol.lastSummary) < ol.summaryInterval {
		return
	}
	ol.lastSummary = now

	logger.Info("Optimization summary",
		logger.Fields{
			Component:  "optimization",
			Operation:  "summary",
			Source:     source,
			Additional: stats,
		})
}

// LogThresholdAlert logs when a threshold is exceeded
func (ol *OptimizationLogger) LogThresholdAlert(source, threshold string, value float64, warningThreshold, criticalThreshold float64) {
	severity := "warning"
	if value >= criticalThreshold {
		severity = "critical"
	}

	logger.Warn("Threshold exceeded",
		logger.Fields{
			Component: "optimization",
			Operation: "threshold_alert",
			Source:    source,
			Additional: map[string]interface{}{
				"threshold":          threshold,
				"value":              value,
				"warning_threshold":  warningThreshold,
				"critical_threshold": criticalThreshold,
				"severity":           severity,
			},
		})
}

// LogAutoOptimization logs when auto-optimization is applied
func (ol *OptimizationLogger) LogAutoOptimization(source, action, reason string, expectedImpact string) {
	logger.Info("Auto-optimization applied",
		logger.Fields{
			Component: "optimization",
			Operation: "auto_optimization",
			Source:    source,
			Additional: map[string]interface{}{
				"action":          action,
				"reason":          reason,
				"expected_impact": expectedImpact,
			},
		})
}

// LogProcessingOrderChange logs when processing order changes
func (ol *OptimizationLogger) LogProcessingOrderChange(source, oldOrder, newOrder, reason string) {
	logger.Info("Processing order changed",
		logger.Fields{
			Component: "optimization",
			Operation: "order_change",
			Source:    source,
			Additional: map[string]interface{}{
				"old_order": oldOrder,
				"new_order": newOrder,
				"reason":    reason,
			},
		})
}

// LogWeeklyReport logs a weekly optimization report
func (ol *OptimizationLogger) LogWeeklyReport(impacts map[string]*advisor.ImpactMetrics) {
	totalOptimizations := 0
	totalReduced := int64(0)
	mostEffective := ""

	for source, impact := range impacts {
		totalOptimizations += impact.OptimizationsApplied
		totalReduced += impact.ObservationsReduced
		if impact.MostEffective != "" && mostEffective == "" {
			mostEffective = fmt.Sprintf("%s: %s", source, impact.MostEffective)
		}
	}

	logger.Info("Weekly optimization report",
		logger.Fields{
			Component: "optimization",
			Operation: "weekly_report",
			Additional: map[string]interface{}{
				"total_optimizations":        totalOptimizations,
				"total_observations_reduced": totalReduced,
				"most_effective":             mostEffective,
			},
		})
}
