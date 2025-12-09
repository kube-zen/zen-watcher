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
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
)

func TestOptimizationStateManager_GetState_NewSource(t *testing.T) {
	osm := NewOptimizationStateManager()

	state := osm.GetState("test-source")

	if state == nil {
		t.Fatal("Expected non-nil state for new source")
	}

	if state.Source != "test-source" {
		t.Errorf("Expected source 'test-source', got %s", state.Source)
	}

	if state.CurrentStrategy != "filter_first" {
		t.Errorf("Expected default strategy 'filter_first', got %s", state.CurrentStrategy)
	}
}

func TestOptimizationStateManager_GetState_ExistingSource(t *testing.T) {
	osm := NewOptimizationStateManager()

	state1 := osm.GetState("test-source")
	state1.CurrentStrategy = "dedup_first"

	state2 := osm.GetState("test-source")

	if state2 != state1 {
		t.Error("Expected same state instance for existing source")
	}

	if state2.CurrentStrategy != "dedup_first" {
		t.Errorf("Expected strategy 'dedup_first', got %s", state2.CurrentStrategy)
	}
}

func TestOptimizationStateManager_RecordDecision(t *testing.T) {
	osm := NewOptimizationStateManager()

	decision := OptimizationDecision{
		Type:          "strategy_change",
		PreviousValue: "filter_first",
		NewValue:      "dedup_first",
		Confidence:    0.85,
		Reason:        "High dedup effectiveness detected",
		ImpactMetrics: map[string]float64{
			"dedup_effectiveness": 0.7,
		},
	}

	err := osm.RecordDecision("test-source", decision)
	if err != nil {
		t.Fatalf("Unexpected error recording decision: %v", err)
	}

	state := osm.GetState("test-source")
	if state.CurrentStrategy != "dedup_first" {
		t.Errorf("Expected strategy updated to 'dedup_first', got %s", state.CurrentStrategy)
	}

	if len(state.DecisionHistory) != 1 {
		t.Errorf("Expected 1 decision in history, got %d", len(state.DecisionHistory))
	}

	if state.DecisionHistory[0].Reason != decision.Reason {
		t.Errorf("Expected reason '%s', got '%s'", decision.Reason, state.DecisionHistory[0].Reason)
	}
}

func TestOptimizationStateManager_RecordDecision_HistoryLimit(t *testing.T) {
	osm := NewOptimizationStateManager()

	// Record more decisions than maxHistorySize
	for i := 0; i < 150; i++ {
		decision := OptimizationDecision{
			Type:       "strategy_change",
			NewValue:   "filter_first",
			Confidence: 0.5,
			Reason:     "Test decision",
		}
		err := osm.RecordDecision("test-source", decision)
		if err != nil {
			t.Fatalf("Unexpected error recording decision: %v", err)
		}
	}

	state := osm.GetState("test-source")
	// History should be trimmed to maxHistorySize (default 100)
	if len(state.DecisionHistory) > 100 {
		t.Errorf("Expected history trimmed to maxHistorySize (100), got %d", len(state.DecisionHistory))
	}
}

func TestOptimizationStateManager_UpdatePerformanceData(t *testing.T) {
	osm := NewOptimizationStateManager()

	perfData := &PerformanceData{
		Source:                 "test-source",
		AverageLatency:         150 * time.Millisecond,
		PeakLatency:            500 * time.Millisecond,
		TotalProcessed:         1000,
		ThroughputEventsPerSec: 50.0,
		LastUpdated:            time.Now(),
	}

	osm.UpdatePerformanceData("test-source", perfData)

	state := osm.GetState("test-source")
	if state.PerformanceData == nil {
		t.Fatal("Expected non-nil performance data")
	}

	if state.PerformanceData.TotalProcessed != 1000 {
		t.Errorf("Expected 1000 total processed, got %d", state.PerformanceData.TotalProcessed)
	}
}

func TestOptimizationStateManager_UpdateActiveRules(t *testing.T) {
	osm := NewOptimizationStateManager()

	rules := []config.DynamicFilterRule{
		{
			Condition: "severity == 'low'",
			Action:    "filter",
		},
		{
			Condition: "namespace == 'default'",
			Action:    "allow",
		},
	}

	osm.UpdateActiveRules("test-source", rules)

	state := osm.GetState("test-source")
	if len(state.ActiveRules) != 2 {
		t.Errorf("Expected 2 active rules, got %d", len(state.ActiveRules))
	}
}

func TestOptimizationStateManager_UpdateMetrics(t *testing.T) {
	osm := NewOptimizationStateManager()

	metrics := map[string]float64{
		"cpu_usage":    75.5,
		"memory_usage": 60.2,
		"queue_depth":  10.0,
	}

	osm.UpdateMetrics("test-source", metrics)

	state := osm.GetState("test-source")
	if state.Metrics["cpu_usage"] != 75.5 {
		t.Errorf("Expected cpu_usage 75.5, got %f", state.Metrics["cpu_usage"])
	}

	if state.Metrics["memory_usage"] != 60.2 {
		t.Errorf("Expected memory_usage 60.2, got %f", state.Metrics["memory_usage"])
	}
}

func TestOptimizationStateManager_GetDecisionHistory(t *testing.T) {
	osm := NewOptimizationStateManager()

	// Record multiple decisions
	for i := 0; i < 5; i++ {
		decision := OptimizationDecision{
			Type:       "strategy_change",
			NewValue:   "filter_first",
			Confidence: 0.5 + float64(i)*0.1,
			Reason:     "Test decision",
		}
		osm.RecordDecision("test-source", decision)
	}

	history := osm.GetDecisionHistory("test-source", 0) // Get all
	if len(history) != 5 {
		t.Errorf("Expected 5 decisions in history, got %d", len(history))
	}

	limitedHistory := osm.GetDecisionHistory("test-source", 3) // Get last 3
	if len(limitedHistory) != 3 {
		t.Errorf("Expected 3 decisions in limited history, got %d", len(limitedHistory))
	}
}

func TestOptimizationStateManager_GetAllStates(t *testing.T) {
	osm := NewOptimizationStateManager()

	osm.GetState("source1")
	osm.GetState("source2")
	osm.GetState("source3")

	allStates := osm.GetAllStates()
	if len(allStates) != 3 {
		t.Errorf("Expected 3 states, got %d", len(allStates))
	}

	if allStates["source1"] == nil {
		t.Error("Expected state for source1")
	}

	if allStates["source2"] == nil {
		t.Error("Expected state for source2")
	}

	if allStates["source3"] == nil {
		t.Error("Expected state for source3")
	}
}
