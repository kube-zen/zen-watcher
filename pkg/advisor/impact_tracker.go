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

package advisor

import (
	"sync"
	"time"
)

// ImpactTracker tracks the impact of applied optimizations
type ImpactTracker struct {
	impacts map[string]*ImpactMetrics // source -> metrics
	mu      sync.RWMutex
}

// NewImpactTracker creates a new impact tracker
func NewImpactTracker() *ImpactTracker {
	return &ImpactTracker{
		impacts: make(map[string]*ImpactMetrics),
	}
}

// RecordSuggestion records that a suggestion was generated
func (it *ImpactTracker) RecordSuggestion(suggestion Suggestion) {
	it.mu.Lock()
	defer it.mu.Unlock()

	impact := it.getOrCreateImpact(suggestion.Source)
	// Suggestions generated are tracked but don't affect metrics until applied
	_ = impact
}

// RecordApplication records that a suggestion was applied
func (it *ImpactTracker) RecordApplication(suggestion Suggestion) {
	it.mu.Lock()
	defer it.mu.Unlock()

	impact := it.getOrCreateImpact(suggestion.Source)
	impact.OptimizationsApplied++
	impact.LastOptimizedAt = time.Now()

	// Estimate impact based on reduction percentage
	if suggestion.Reduction > 0 {
		// This is a simplified calculation - actual impact would be measured from metrics
		impact.ReductionPercent = suggestion.Reduction
		impact.MostEffective = suggestion.Title
	}
}

// GetImpact returns impact metrics for a source
func (it *ImpactTracker) GetImpact(source string) *ImpactMetrics {
	it.mu.RLock()
	defer it.mu.RUnlock()

	if impact, exists := it.impacts[source]; exists {
		// Return a copy to avoid race conditions
		return &ImpactMetrics{
			Source:               impact.Source,
			OptimizationsApplied: impact.OptimizationsApplied,
			ObservationsReduced:  impact.ObservationsReduced,
			ReductionPercent:     impact.ReductionPercent,
			CPUSavingsMinutes:    impact.CPUSavingsMinutes,
			LastOptimizedAt:      impact.LastOptimizedAt,
			MostEffective:        impact.MostEffective,
		}
	}

	return &ImpactMetrics{
		Source: source,
	}
}

// UpdateImpact updates impact metrics based on actual measurements
func (it *ImpactTracker) UpdateImpact(source string, observationsReduced int64, cpuSavingsMinutes float64) {
	it.mu.Lock()
	defer it.mu.Unlock()

	impact := it.getOrCreateImpact(source)
	impact.ObservationsReduced = observationsReduced
	impact.CPUSavingsMinutes = cpuSavingsMinutes

	// Calculate reduction percent if we have baseline
	// This would require tracking before/after metrics
}

// getOrCreateImpact gets or creates impact metrics for a source
func (it *ImpactTracker) getOrCreateImpact(source string) *ImpactMetrics {
	if impact, exists := it.impacts[source]; exists {
		return impact
	}

	impact := &ImpactMetrics{
		Source: source,
	}
	it.impacts[source] = impact
	return impact
}

// GetAllImpacts returns all impact metrics
func (it *ImpactTracker) GetAllImpacts() map[string]*ImpactMetrics {
	it.mu.RLock()
	defer it.mu.RUnlock()

	result := make(map[string]*ImpactMetrics)
	for k, v := range it.impacts {
		// Return copies
		result[k] = &ImpactMetrics{
			Source:               v.Source,
			OptimizationsApplied: v.OptimizationsApplied,
			ObservationsReduced:  v.ObservationsReduced,
			ReductionPercent:     v.ReductionPercent,
			CPUSavingsMinutes:    v.CPUSavingsMinutes,
			LastOptimizedAt:      v.LastOptimizedAt,
			MostEffective:        v.MostEffective,
		}
	}
	return result
}

