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

package optimization

import (
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// AdaptiveProcessor provides adaptive processing capabilities
type AdaptiveProcessor struct {
	source             string
	config             *generic.SourceConfig
	metricsCollector   *PerSourceMetricsCollector
	performanceTracker *PerformanceTracker

	// Adaptation parameters
	learningRate     float64
	adaptationWindow time.Duration
	lastAdaptation   time.Time
}

// NewAdaptiveProcessor creates a new adaptive processor
func NewAdaptiveProcessor(
	source string,
	config *generic.SourceConfig,
	metricsCollector *PerSourceMetricsCollector,
	performanceTracker *PerformanceTracker,
) *AdaptiveProcessor {
	return &AdaptiveProcessor{
		source:             source,
		config:             config,
		metricsCollector:   metricsCollector,
		performanceTracker: performanceTracker,
		learningRate:       0.1, // Default learning rate
		adaptationWindow:   15 * time.Minute,
	}
}

// ShouldAdapt determines if adaptation should be performed
func (ap *AdaptiveProcessor) ShouldAdapt() bool {
	if !ap.config.Processing.AutoOptimize {
		return false
	}

	// Don't adapt too frequently
	if time.Since(ap.lastAdaptation) < ap.adaptationWindow {
		return false
	}

	// Get current metrics
	metrics := ap.metricsCollector.GetOptimizationMetrics()
	if metrics == nil {
		return false
	}

	// Adapt if performance degradation detected
	if ap.detectPerformanceDegradation(metrics) {
		return true
	}

	// Adapt if thresholds exceeded
	strategyDecider := NewStrategyDecider()
	return strategyDecider.ShouldOptimize(metrics, ap.config)
}

// detectPerformanceDegradation detects if performance has degraded
func (ap *AdaptiveProcessor) detectPerformanceDegradation(metrics *OptimizationMetrics) bool {
	// Check if latency has increased significantly
	if metrics.ProcessingLatency > 1000 { // > 1 second
		return true
	}

	// Check if dedup effectiveness has dropped
	if metrics.DeduplicationRate < 0.1 && metrics.EventsProcessed > 100 {
		return true
	}

	// Check if filter effectiveness is very low
	if metrics.FilterEffectiveness < 0.05 && metrics.EventsProcessed > 100 {
		return true
	}

	return false
}

// Adapt performs adaptive processing adjustments
func (ap *AdaptiveProcessor) Adapt() error {
	metrics := ap.metricsCollector.GetOptimizationMetrics()
	if metrics == nil {
		return nil
	}

	// Adjust filter if needed
	// TODO: Filter configuration is now handled separately, not in SourceConfig
	// if ap.config.Filter != nil && ap.config.Filter.AdaptiveEnabled {
	// 	ap.adaptFilter(metrics)
	// }

	// Adjust deduplication if needed
	// TODO: Deduplication configuration is in ap.config.Dedup, but adaptive features need to be implemented
	// if ap.config.Dedup != nil {
	// 	ap.adaptDeduplication(metrics)
	// }

	ap.lastAdaptation = time.Now()
	return nil
}

// adaptFilter adapts filtering parameters
// TODO: Implement adaptive filtering with new filter configuration structure
func (ap *AdaptiveProcessor) adaptFilter(metrics *OptimizationMetrics) {
	// Filter configuration is now handled separately from SourceConfig
	// This needs to be reimplemented to work with the new structure
	logger.Debug("Adaptive filter adjustment not yet implemented with new config structure",
		logger.Fields{
			Component: "optimization",
			Operation: "adapt_filter",
			Source:    ap.source,
		})
}

// adaptDeduplication adapts deduplication parameters
// TODO: Implement adaptive deduplication with new DedupConfig structure
func (ap *AdaptiveProcessor) adaptDeduplication(metrics *OptimizationMetrics) {
	// Deduplication configuration is in ap.config.Dedup, but adaptive window adjustment
	// needs to be reimplemented to work with the string-based Window field
	if ap.config.Dedup == nil {
		return
	}

	logger.Debug("Adaptive deduplication adjustment not yet fully implemented",
		logger.Fields{
			Component: "optimization",
			Operation: "adapt_dedup",
			Source:    ap.source,
			Additional: map[string]interface{}{
				"dedup_rate": metrics.DeduplicationRate,
			},
		})
}
