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

	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// AdaptiveProcessor provides adaptive processing capabilities
type AdaptiveProcessor struct {
	source          string
	config          *config.SourceConfig
	metricsCollector *PerSourceMetricsCollector
	performanceTracker *PerformanceTracker
	
	// Adaptation parameters
	learningRate    float64
	adaptationWindow time.Duration
	lastAdaptation  time.Time
}

// NewAdaptiveProcessor creates a new adaptive processor
func NewAdaptiveProcessor(
	source string,
	config *config.SourceConfig,
	metricsCollector *PerSourceMetricsCollector,
	performanceTracker *PerformanceTracker,
) *AdaptiveProcessor {
	return &AdaptiveProcessor{
		source:            source,
		config:            config,
		metricsCollector:  metricsCollector,
		performanceTracker: performanceTracker,
		learningRate:      0.1, // Default learning rate
		adaptationWindow:  15 * time.Minute,
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
	if ap.config.Filter.AdaptiveEnabled {
		ap.adaptFilter(metrics)
	}

	// Adjust deduplication if needed
	if ap.config.Deduplication.Adaptive {
		ap.adaptDeduplication(metrics)
	}

	ap.lastAdaptation = time.Now()
	return nil
}

// adaptFilter adapts filtering parameters
func (ap *AdaptiveProcessor) adaptFilter(metrics *OptimizationMetrics) {
	// If too many low-severity events are getting through, tighten filter
	if metrics.LowSeverityPercent > 0.8 && ap.config.Filter.MinPriority < 0.5 {
		newPriority := ap.config.Filter.MinPriority + (ap.learningRate * 0.1)
		if newPriority > 1.0 {
			newPriority = 1.0
		}
		
		logger.Info("Adapting filter priority",
			logger.Fields{
				Component: "optimization",
				Operation: "adapt_filter",
				Source:    ap.source,
				Additional: map[string]interface{}{
					"old_priority": ap.config.Filter.MinPriority,
					"new_priority": newPriority,
					"low_severity_percent": metrics.LowSeverityPercent,
				},
			})
		
		ap.config.Filter.MinPriority = newPriority
		ap.config.FilterMinPriority = newPriority
	}

	// If too few events passing filter, relax it
	if metrics.FilterEffectiveness > 0.9 && ap.config.Filter.MinPriority > 0.1 {
		newPriority := ap.config.Filter.MinPriority - (ap.learningRate * 0.1)
		if newPriority < 0.0 {
			newPriority = 0.0
		}
		
		logger.Info("Relaxing filter priority",
			logger.Fields{
				Component: "optimization",
				Operation: "relax_filter",
				Source:    ap.source,
				Additional: map[string]interface{}{
					"old_priority": ap.config.Filter.MinPriority,
					"new_priority": newPriority,
					"filter_effectiveness": metrics.FilterEffectiveness,
				},
			})
		
		ap.config.Filter.MinPriority = newPriority
		ap.config.FilterMinPriority = newPriority
	}
}

// adaptDeduplication adapts deduplication parameters
func (ap *AdaptiveProcessor) adaptDeduplication(metrics *OptimizationMetrics) {
	// If deduplication is very effective, window might be too large (wasteful)
	// If deduplication is ineffective, window might be too small
	if metrics.DeduplicationRate > 0.8 && ap.config.Deduplication.Window > 1*time.Hour {
		// Very effective, can reduce window slightly
		newWindow := ap.config.Deduplication.Window - (10 * time.Minute)
		if newWindow < 5*time.Minute {
			newWindow = 5 * time.Minute
		}
		
		logger.Info("Reducing dedup window",
			logger.Fields{
				Component: "optimization",
				Operation: "adapt_dedup_window",
				Source:    ap.source,
				Additional: map[string]interface{}{
					"old_window": ap.config.Deduplication.Window.String(),
					"new_window": newWindow.String(),
					"dedup_rate": metrics.DeduplicationRate,
				},
			})
		
		ap.config.Deduplication.Window = newWindow
		ap.config.DedupWindow = newWindow
	} else if metrics.DeduplicationRate < 0.2 && ap.config.Deduplication.Window < 24*time.Hour {
		// Ineffective, increase window
		newWindow := ap.config.Deduplication.Window + (10 * time.Minute)
		if newWindow > 24*time.Hour {
			newWindow = 24 * time.Hour
		}
		
		logger.Info("Increasing dedup window",
			logger.Fields{
				Component: "optimization",
				Operation: "adapt_dedup_window",
				Source:    ap.source,
				Additional: map[string]interface{}{
					"old_window": ap.config.Deduplication.Window.String(),
					"new_window": newWindow.String(),
					"dedup_rate": metrics.DeduplicationRate,
				},
			})
		
		ap.config.Deduplication.Window = newWindow
		ap.config.DedupWindow = newWindow
	}
}

