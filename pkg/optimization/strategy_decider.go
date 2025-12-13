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
	"github.com/kube-zen/zen-watcher/pkg/config"
)

// ProcessingStrategy represents the processing order strategy
type ProcessingStrategy int

const (
	// ProcessingStrategyFilterFirst processes filter → normalize → dedup → create
	ProcessingStrategyFilterFirst ProcessingStrategy = iota
	// ProcessingStrategyDedupFirst processes dedup → filter → normalize → create
	ProcessingStrategyDedupFirst
	// ProcessingStrategyHybrid uses a hybrid approach based on event characteristics
	ProcessingStrategyHybrid
	// ProcessingStrategyAdaptive uses machine learning for dynamic strategy selection
	ProcessingStrategyAdaptive
)

// String returns the string representation of the processing strategy
func (ps ProcessingStrategy) String() string {
	switch ps {
	case ProcessingStrategyFilterFirst:
		return "filter_first"
	case ProcessingStrategyDedupFirst:
		return "dedup_first"
	case ProcessingStrategyHybrid:
		return "hybrid"
	case ProcessingStrategyAdaptive:
		return "adaptive"
	default:
		return "filter_first"
	}
}

// OptimizationMetrics represents metrics used for optimization decisions
type OptimizationMetrics struct {
	Source                string
	EventsProcessed       int64
	EventsFiltered        int64
	EventsDeduped         int64
	ProcessingLatency     int64 // in milliseconds
	DeduplicationRate     float64
	FilterEffectiveness   float64
	FalsePositiveRate     float64
	CPUUsagePercent       float64
	MemoryUsageBytes      int64
	NetworkBytes          int64
	OptimizationCount     int
	LowSeverityPercent    float64
	ObservationsPerMinute float64
}

// StrategyDecider determines the optimal processing strategy based on metrics and configuration
type StrategyDecider struct {
	config *OptimizationConfig
}

// NewStrategyDecider creates a new StrategyDecider with default thresholds
func NewStrategyDecider() *StrategyDecider {
	return &StrategyDecider{
		config: DefaultOptimizationConfig(),
	}
}

// NewStrategyDeciderWithConfig creates a new StrategyDecider with custom config
func NewStrategyDeciderWithConfig(config *OptimizationConfig) *StrategyDecider {
	if config == nil {
		config = DefaultOptimizationConfig()
	}
	return &StrategyDecider{
		config: config,
	}
}

// NewStrategyDeciderWithThresholds creates a new StrategyDecider with custom thresholds
func NewStrategyDeciderWithThresholds(
	filterFirstThreshold, dedupFirstThreshold float64,
	adaptiveThreshold int64,
) *StrategyDecider {
	config := DefaultOptimizationConfig()
	config.FilterFirstThresholdLowSeverity = filterFirstThreshold
	config.DedupFirstThresholdEffectiveness = dedupFirstThreshold
	config.AdaptiveThresholdVolume = adaptiveThreshold
	return &StrategyDecider{
		config: config,
	}
}

// DetermineStrategy determines the optimal processing strategy based on metrics and config
func (sd *StrategyDecider) DetermineStrategy(
	metrics *OptimizationMetrics,
	config *config.SourceConfig,
) ProcessingStrategy {
	// If config specifies a non-auto order, use it
	if config != nil && config.Processing.Order != "" && config.Processing.Order != "auto" {
		return sd.parseStrategy(config.Processing.Order)
	}

	// If auto-optimization is disabled, use default
	if config != nil && !config.Processing.AutoOptimize {
		return sd.getDefaultStrategy(config.Source)
	}

	// If no metrics available, use default strategy
	if metrics == nil {
		return sd.getDefaultStrategy(config.Source)
	}

	// Rule 1: If high LOW severity (>70%), filter_first
	// This removes noise early before expensive dedup operations
	if metrics.LowSeverityPercent >= sd.config.FilterFirstThresholdLowSeverity {
		return ProcessingStrategyFilterFirst
	}

	// Rule 2: If high dedup effectiveness (>50%), dedup_first
	// This removes duplicates early before filter operations
	if metrics.DeduplicationRate >= sd.config.DedupFirstThresholdEffectiveness {
		return ProcessingStrategyDedupFirst
	}

	// Rule 3: If high volume and moderate dedup, use hybrid
	if metrics.EventsProcessed >= sd.config.AdaptiveThresholdVolume &&
		metrics.DeduplicationRate > 0.3 && metrics.DeduplicationRate < 0.5 {
		return ProcessingStrategyHybrid
	}

	// Rule 4: If very high volume and auto-optimize enabled, use adaptive
	if config != nil && config.Processing.AutoOptimize &&
		metrics.EventsProcessed >= sd.config.AdaptiveThresholdVolume*2 {
		return ProcessingStrategyAdaptive
	}

	// Default: Use source-specific default
	return sd.getDefaultStrategy(config.Source)
}

// parseStrategy parses a string strategy name to ProcessingStrategy
func (sd *StrategyDecider) parseStrategy(strategy string) ProcessingStrategy {
	switch strategy {
	case "filter_first":
		return ProcessingStrategyFilterFirst
	case "dedup_first":
		return ProcessingStrategyDedupFirst
	case "hybrid":
		return ProcessingStrategyHybrid
	case "adaptive":
		return ProcessingStrategyAdaptive
	default:
		return ProcessingStrategyFilterFirst
	}
}

// getDefaultStrategy returns the default strategy for a source
func (sd *StrategyDecider) getDefaultStrategy(source string) ProcessingStrategy {
	// Default: filter first (configurable via Ingester CRD)
	// Source-specific defaults are configured via YAML, not hardcoded
	return ProcessingStrategyFilterFirst
}

// ShouldOptimize determines if optimization should be triggered based on metrics
func (sd *StrategyDecider) ShouldOptimize(
	metrics *OptimizationMetrics,
	config *config.SourceConfig,
) bool {
	if config == nil || !config.Processing.AutoOptimize {
		return false
	}

	if metrics == nil {
		return false
	}

	// Trigger optimization if thresholds are exceeded
	thresholds := config.Processing.Thresholds

	// Check observationsPerMinute threshold
	if obsThreshold, ok := thresholds["observationsPerMinute"]; ok {
		if metrics.ObservationsPerMinute >= obsThreshold.Warning {
			return true
		}
	}

	// Check lowSeverityPercent threshold
	if lowSevThreshold, ok := thresholds["lowSeverityPercent"]; ok {
		if metrics.LowSeverityPercent >= lowSevThreshold.Warning {
			return true
		}
	}

	// Check dedupEffectiveness threshold
	if dedupThreshold, ok := thresholds["dedupEffectiveness"]; ok {
		if metrics.DeduplicationRate <= dedupThreshold.Critical {
			return true
		}
	}

	return false
}
