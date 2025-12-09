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
)

// OptimizationConfig centralizes all thresholds and configuration for the optimization subsystem
type OptimizationConfig struct {
	// Strategy decision thresholds
	FilterFirstThresholdLowSeverity  float64 // Default: 0.7 (70%)
	DedupFirstThresholdEffectiveness float64 // Default: 0.5 (50%)
	AdaptiveThresholdVolume          int64   // Default: 100 events/min

	// Optimization engine settings
	OptimizationInterval time.Duration // Default: 5 minutes
	MaxHistorySize       int           // Default: 100 decisions per source

	// Performance tracker settings
	MaxLatencies      int           // Default: 1000 latency measurements
	AggregationWindow time.Duration // Default: 10 minutes

	// Adaptive processor settings
	LearningRate     float64       // Default: 0.1
	AdaptationWindow time.Duration // Default: 15 minutes
	CooldownPeriod   time.Duration // Default: 5 minutes (hysteresis)

	// Confidence calculation thresholds
	MinEventsForConfidence int64   // Default: 100
	HighDedupRate          float64 // Default: 0.7
	LowDedupRate           float64 // Default: 0.2
	HighFilterRate         float64 // Default: 0.7
	LowFilterRate          float64 // Default: 0.2

	// Safety limits
	MaxDecisionFrequency time.Duration // Default: 1 minute (prevent oscillation)
}

// DefaultOptimizationConfig returns a config with sensible defaults
func DefaultOptimizationConfig() *OptimizationConfig {
	return &OptimizationConfig{
		FilterFirstThresholdLowSeverity:  0.7,
		DedupFirstThresholdEffectiveness: 0.5,
		AdaptiveThresholdVolume:          100,
		OptimizationInterval:             5 * time.Minute,
		MaxHistorySize:                   100,
		MaxLatencies:                     1000,
		AggregationWindow:                10 * time.Minute,
		LearningRate:                     0.1,
		AdaptationWindow:                 15 * time.Minute,
		CooldownPeriod:                   5 * time.Minute,
		MinEventsForConfidence:           100,
		HighDedupRate:                    0.7,
		LowDedupRate:                     0.2,
		HighFilterRate:                   0.7,
		LowFilterRate:                    0.2,
		MaxDecisionFrequency:             1 * time.Minute,
	}
}
