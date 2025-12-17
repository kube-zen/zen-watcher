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

// TestStrategyDecider_ThresholdsMatchDocumentation verifies that thresholds
// used in StrategyDecider match documented thresholds
// Note: Auto-optimization removed, but thresholds are still used for manual guidance
func TestStrategyDecider_ThresholdsMatchDocumentation(t *testing.T) {
	sd := NewStrategyDecider()
	config := sd.config

	// According to documented thresholds:
	// - filter_first threshold: LOW severity > 70%
	// - dedup_first threshold: dedup effectiveness > 50%

	// Verify filter_first threshold
	expectedFilterFirstThreshold := 0.70
	if config.FilterFirstThresholdLowSeverity != expectedFilterFirstThreshold {
		t.Errorf("FilterFirstThresholdLowSeverity = %v, want %v (70%% as documented)",
			config.FilterFirstThresholdLowSeverity, expectedFilterFirstThreshold)
	}

	// Verify dedup_first threshold
	expectedDedupFirstThreshold := 0.50
	if config.DedupFirstThresholdEffectiveness != expectedDedupFirstThreshold {
		t.Errorf("DedupFirstThresholdEffectiveness = %v, want %v (50%% as documented)",
			config.DedupFirstThresholdEffectiveness, expectedDedupFirstThreshold)
	}
}

// TestStrategyDecider_HighLowSeverityTriggersFilterFirst verifies HIGH low-severity ratio → filter_first
func TestStrategyDecider_HighLowSeverityTriggersFilterFirst(t *testing.T) {
	sd := NewStrategyDecider()

	// Test with 75% low severity (above 70% threshold)
	metrics := &OptimizationMetrics{
		Source:             "test-source",
		EventsProcessed:    1000,
		LowSeverityPercent: 0.75, // 75% > 70% threshold
		DeduplicationRate:  0.3,  // Below dedup threshold
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "", // No explicit order
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyFilterFirst {
		t.Errorf("Expected filter_first for 75%% low severity (above 70%% threshold), got %s", strategy.String())
	}
}

// TestStrategyDecider_HighDedupEffectivenessTriggersDedupFirst verifies HIGH dedup effectiveness → dedup_first
func TestStrategyDecider_HighDedupEffectivenessTriggersDedupFirst(t *testing.T) {
	sd := NewStrategyDecider()

	// Test with 60% dedup effectiveness (above 50% threshold)
	metrics := &OptimizationMetrics{
		Source:             "test-source",
		EventsProcessed:    1000,
		LowSeverityPercent: 0.3, // Below filter threshold
		DeduplicationRate:  0.6, // 60% > 50% threshold
	}

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "", // No explicit order
		},
	}

	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyDedupFirst {
		t.Errorf("Expected dedup_first for 60%% dedup effectiveness (above 50%% threshold), got %s", strategy.String())
	}
}

// TestStrategyDecider_ThresholdBoundaryConditions tests boundary conditions
func TestStrategyDecider_ThresholdBoundaryConditions(t *testing.T) {
	sd := NewStrategyDecider()

	sourceConfig := &generic.SourceConfig{
		Source: "test-source",
		Processing: &generic.ProcessingConfig{
			Order: "", // No explicit order
		},
	}

	// Test exactly at filter_first threshold (70%)
	metrics := &OptimizationMetrics{
		Source:             "test-source",
		LowSeverityPercent: 0.70, // Exactly at threshold
		DeduplicationRate:  0.3,
	}
	strategy := sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyFilterFirst {
		t.Errorf("Expected filter_first at exactly 70%% threshold, got %s", strategy.String())
	}

	// Test just below filter_first threshold (69.9%)
	metrics.LowSeverityPercent = 0.699
	metrics.DeduplicationRate = 0.6 // Above dedup threshold
	strategy = sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyDedupFirst {
		t.Errorf("Expected dedup_first when below filter threshold and above dedup threshold, got %s", strategy.String())
	}

	// Test exactly at dedup_first threshold (50%)
	metrics.LowSeverityPercent = 0.3
	metrics.DeduplicationRate = 0.50 // Exactly at threshold
	strategy = sd.DetermineStrategy(metrics, sourceConfig)
	if strategy != ProcessingStrategyDedupFirst {
		t.Errorf("Expected dedup_first at exactly 50%% threshold, got %s", strategy.String())
	}
}
