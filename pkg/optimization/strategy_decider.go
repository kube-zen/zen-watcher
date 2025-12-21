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
	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
)

// ProcessingStrategy represents the processing order strategy
type ProcessingStrategy int

const (
	// ProcessingStrategyFilterFirst processes filter → normalize → dedup → create
	ProcessingStrategyFilterFirst ProcessingStrategy = iota
	// ProcessingStrategyDedupFirst processes dedup → filter → normalize → create
	ProcessingStrategyDedupFirst
)

// String returns the string representation of the processing strategy
func (ps ProcessingStrategy) String() string {
	switch ps {
	case ProcessingStrategyFilterFirst:
		return "filter_first"
	case ProcessingStrategyDedupFirst:
		return "dedup_first"
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
	config *generic.SourceConfig,
) ProcessingStrategy {
	// If config specifies a non-auto order, use it
	if config != nil && config.Processing.Order != "" && config.Processing.Order != "auto" {
		return sd.parseStrategy(config.Processing.Order)
	}

	// Note: Auto-optimization has been removed. This code path is no longer used.
	// Processing order is now configured manually via config.Processing.Order

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

	// Note: Auto-optimization and adaptive mode have been removed.
	// Processing order is now configured manually via config.Processing.Order

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
// Note: Auto-optimization has been removed. This function always returns false.
func (sd *StrategyDecider) ShouldOptimize(
	metrics *OptimizationMetrics,
	config *generic.SourceConfig,
) bool {
	// Auto-optimization removed - always return false
	return false
}
