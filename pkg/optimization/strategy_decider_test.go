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
	"testing"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
)

func TestStrategyDecider_DetermineStrategy_FilterFirst(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:             "test-source",
		EventsProcessed:    1000,
		LowSeverityPercent: 0.75, // 75% low severity
		DeduplicationRate:  0.3,
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "", // No explicit order - will use default
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyFilterFirst {
		t.Errorf("Expected filter_first strategy for high low-severity (75%%), got %s", strategy.String())
	}
}

func TestStrategyDecider_DetermineStrategy_DedupFirst(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:             "test-source",
		EventsProcessed:    1000,
		LowSeverityPercent: 0.3,
		DeduplicationRate:  0.6, // 60% dedup effectiveness
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "", // No explicit order - will use default
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyDedupFirst {
		t.Errorf("Expected dedup_first strategy for high dedup effectiveness (60%%), got %s", strategy.String())
	}
}

// TestStrategyDecider_DetermineStrategy_Adaptive removed - adaptive mode no longer supported
// Auto-optimization has been removed. Processing order is now configured manually.

func TestStrategyDecider_DetermineStrategy_NoMetrics(t *testing.T) {
	sd := NewStrategyDecider()

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "", // No explicit order - will use default
		},
	}

	strategy := sd.DetermineStrategy(nil, sourceConfig)
	if strategy != ProcessingStrategyFilterFirst {
		t.Errorf("Expected default filter_first strategy when no metrics, got %s", strategy.String())
	}
}

func TestStrategyDecider_DetermineStrategy_ConfigOverride(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:             "test-source",
		LowSeverityPercent: 0.8, // Would normally trigger filter_first
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "dedup_first", // Explicit override
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyDedupFirst {
		t.Errorf("Expected dedup_first strategy from config override, got %s", strategy.String())
	}
}

func TestStrategyDecider_ShouldOptimize_ThresholdsExceeded(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:                "test-source",
		ObservationsPerMinute: 150, // Above warning threshold
		LowSeverityPercent:    0.8,
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "", // No explicit order
		},
		// Note: Thresholds are now in generic.ThresholdsConfig, not ProcessingConfig
		Thresholds: &generic.ThresholdsConfig{
			ObservationsPerMinute: &generic.ThresholdValues{
				Warning:  100,
				Critical: 200,
			},
		},
	}

	// Auto-optimization removed - ShouldOptimize always returns false
	shouldOptimize := sd.ShouldOptimize(metrics, sourceConfig)
	if shouldOptimize {
		t.Error("Expected ShouldOptimize to return false (auto-optimization removed)")
	}
}

func TestStrategyDecider_ShouldOptimize_AutoOptimizeDisabled(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:                "test-source",
		ObservationsPerMinute: 150,
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "filter_first", // Manual order selection
		},
	}

	// Auto-optimization removed - ShouldOptimize always returns false
	shouldOptimize := sd.ShouldOptimize(metrics, sourceConfig)
	if shouldOptimize {
		t.Error("Expected ShouldOptimize to return false (auto-optimization removed)")
	}
}
