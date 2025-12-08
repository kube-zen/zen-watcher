// Copyright 2024 The Zen Watcher Authors
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
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
)

// OptimizationDecision represents a decision made by the optimization system
type OptimizationDecision struct {
	Timestamp     time.Time
	Type          string      // strategy_change, rule_update, threshold_adjustment
	PreviousValue interface{}
	NewValue      interface{}
	Confidence    float64
	Reason        string
	ImpactMetrics map[string]float64
}

// OptimizationState tracks the optimization state for a source
type OptimizationState struct {
	Source          string
	CurrentStrategy string
	LastDecision    time.Time
	DecisionHistory []OptimizationDecision
	PerformanceData *PerformanceData
	ActiveRules     []config.DynamicFilterRule
	Metrics         map[string]float64
	LastUpdated     time.Time
	mu              sync.RWMutex
}

// OptimizationStateManager manages optimization state for all sources
type OptimizationStateManager struct {
	states         map[string]*OptimizationState
	persistence    StatePersistence // Optional persistence layer
	mu             sync.RWMutex
	maxHistorySize int
}

// StatePersistence interface for persisting optimization state
type StatePersistence interface {
	Save(source string, state *OptimizationState) error
	Load(source string) (*OptimizationState, error)
	Delete(source string) error
}

// NewOptimizationStateManager creates a new optimization state manager
func NewOptimizationStateManager() *OptimizationStateManager {
	return &OptimizationStateManager{
		states:         make(map[string]*OptimizationState),
		maxHistorySize: 100, // Keep last 100 decisions per source
	}
}

// NewOptimizationStateManagerWithPersistence creates a new state manager with persistence
func NewOptimizationStateManagerWithPersistence(persistence StatePersistence) *OptimizationStateManager {
	return &OptimizationStateManager{
		states:         make(map[string]*OptimizationState),
		persistence:    persistence,
		maxHistorySize: 100,
	}
}

// GetState gets or creates optimization state for a source
func (osm *OptimizationStateManager) GetState(source string) *OptimizationState {
	osm.mu.Lock()
	defer osm.mu.Unlock()

	if state, exists := osm.states[source]; exists {
		return state
	}

	// Try loading from persistence if available
	if osm.persistence != nil {
		if state, err := osm.persistence.Load(source); err == nil && state != nil {
			osm.states[source] = state
			return state
		}
	}

	// Create new state
	state := &OptimizationState{
		Source:          source,
		CurrentStrategy: "filter_first", // Default
		DecisionHistory: make([]OptimizationDecision, 0),
		ActiveRules:     make([]config.DynamicFilterRule, 0),
		Metrics:         make(map[string]float64),
		LastUpdated:     time.Now(),
	}

	osm.states[source] = state
	return state
}

// RecordDecision records an optimization decision
func (osm *OptimizationStateManager) RecordDecision(source string, decision OptimizationDecision) error {
	state := osm.GetState(source)

	state.mu.Lock()
	defer state.mu.Unlock()

	decision.Timestamp = time.Now()
	state.DecisionHistory = append(state.DecisionHistory, decision)
	state.LastDecision = decision.Timestamp
	state.LastUpdated = time.Now()

	// Trim history if too large
	if len(state.DecisionHistory) > osm.maxHistorySize {
		state.DecisionHistory = state.DecisionHistory[len(state.DecisionHistory)-osm.maxHistorySize:]
	}

	// Update current strategy if it's a strategy change
	if decision.Type == "strategy_change" {
		if newStrategy, ok := decision.NewValue.(string); ok {
			state.CurrentStrategy = newStrategy
		}
	}

	// Persist if persistence is available
	if osm.persistence != nil {
		if err := osm.persistence.Save(source, state); err != nil {
			return err
		}
	}

	return nil
}

// UpdatePerformanceData updates performance data for a source
func (osm *OptimizationStateManager) UpdatePerformanceData(source string, data *PerformanceData) {
	state := osm.GetState(source)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.PerformanceData = data
	state.LastUpdated = time.Now()
}

// UpdateActiveRules updates active rules for a source
func (osm *OptimizationStateManager) UpdateActiveRules(source string, rules []config.DynamicFilterRule) {
	state := osm.GetState(source)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.ActiveRules = rules
	state.LastUpdated = time.Now()
}

// UpdateMetrics updates metrics for a source
func (osm *OptimizationStateManager) UpdateMetrics(source string, metrics map[string]float64) {
	state := osm.GetState(source)

	state.mu.Lock()
	defer state.mu.Unlock()

	for k, v := range metrics {
		state.Metrics[k] = v
	}
	state.LastUpdated = time.Now()
}

// GetDecisionHistory returns the decision history for a source
func (osm *OptimizationStateManager) GetDecisionHistory(source string, limit int) []OptimizationDecision {
	state := osm.GetState(source)

	state.mu.RLock()
	defer state.mu.RUnlock()

	history := state.DecisionHistory
	if limit > 0 && limit < len(history) {
		start := len(history) - limit
		history = history[start:]
	}

	return history
}

// GetAllStates returns all optimization states
func (osm *OptimizationStateManager) GetAllStates() map[string]*OptimizationState {
	osm.mu.RLock()
	defer osm.mu.RUnlock()

	result := make(map[string]*OptimizationState)
	for k, v := range osm.states {
		result[k] = v
	}
	return result
}

