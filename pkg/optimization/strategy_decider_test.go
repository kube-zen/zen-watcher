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

	"github.com/kube-zen/zen-watcher/pkg/config"
)

func TestStrategyDecider_DetermineStrategy_FilterFirst(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:             "test-source",
		EventsProcessed:    1000,
		LowSeverityPercent: 0.75, // 75% low severity
		DeduplicationRate:  0.3,
	}

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
			AutoOptimize: true,
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

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
			AutoOptimize: true,
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyDedupFirst {
		t.Errorf("Expected dedup_first strategy for high dedup effectiveness (60%%), got %s", strategy.String())
	}
}

func TestStrategyDecider_DetermineStrategy_Hybrid(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:             "test-source",
		EventsProcessed:    150, // High volume
		LowSeverityPercent: 0.4,
		DeduplicationRate:  0.4, // Moderate dedup (30-50%)
	}

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
			AutoOptimize: true,
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyHybrid {
		t.Errorf("Expected hybrid strategy for high volume and moderate dedup, got %s", strategy.String())
	}
}

func TestStrategyDecider_DetermineStrategy_Adaptive(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:             "test-source",
		EventsProcessed:    250, // Very high volume (2x threshold)
		LowSeverityPercent: 0.4,
		DeduplicationRate:  0.3,
	}

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
			AutoOptimize: true,
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyAdaptive {
		t.Errorf("Expected adaptive strategy for very high volume, got %s", strategy.String())
	}
}

func TestStrategyDecider_DetermineStrategy_NoMetrics(t *testing.T) {
	sd := NewStrategyDecider()

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
			AutoOptimize: true,
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

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
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

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
			AutoOptimize: true,
			Thresholds: map[string]config.Threshold{
				"observationsPerMinute": {
					Warning:  100,
					Critical: 200,
				},
			},
		},
	}

	shouldOptimize := sd.ShouldOptimize(metrics, sourceConfig)
	if !shouldOptimize {
		t.Error("Expected ShouldOptimize to return true when thresholds exceeded")
	}
}

func TestStrategyDecider_ShouldOptimize_AutoOptimizeDisabled(t *testing.T) {
	sd := NewStrategyDecider()

	metrics := &OptimizationMetrics{
		Source:                "test-source",
		ObservationsPerMinute: 150,
	}

	sourceConfig := &config.SourceConfig{
		Source: "test-source",
		Processing: config.ProcessingConfig{
			AutoOptimize: false, // Disabled
		},
	}

	shouldOptimize := sd.ShouldOptimize(metrics, sourceConfig)
	if shouldOptimize {
		t.Error("Expected ShouldOptimize to return false when auto-optimize is disabled")
	}
}
