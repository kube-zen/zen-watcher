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
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
)

// Event represents an event being processed
type Event struct {
	Source    string
	Priority  float64
	Severity  string
	Category  string
	Namespace string
	Type      string
	RawData   map[string]interface{}
}

// FilterMetrics tracks filter performance
type FilterMetrics struct {
	TotalProcessed int64
	TotalAllowed   int64
	TotalFiltered  int64
	FalsePositives int64
	FalseNegatives int64
	LastUpdated    time.Time
}

// AdaptiveFilter provides adaptive filtering with learning capabilities
type AdaptiveFilter struct {
	source          string
	rules           []config.DynamicFilterRule
	metrics         *FilterMetrics
	learningEnabled bool
	adaptationRate  float64
	mu              sync.RWMutex
}

// NewAdaptiveFilter creates a new AdaptiveFilter
func NewAdaptiveFilter(source string, filterConfig config.FilterConfigAdvanced) *AdaptiveFilter {
	return &AdaptiveFilter{
		source:          source,
		rules:           filterConfig.DynamicRules,
		metrics:         &FilterMetrics{LastUpdated: time.Now()},
		learningEnabled: filterConfig.AdaptiveEnabled,
		adaptationRate:  filterConfig.LearningRate,
	}
}

// Allow determines if an event should be allowed through the filter
func (f *AdaptiveFilter) Allow(event *Event) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.metrics.TotalProcessed++

	// Apply static rules first
	if !f.applyStaticRules(event) {
		f.metrics.TotalFiltered++
		return false
	}

	// Apply dynamic rules if learning is enabled
	if f.learningEnabled {
		f.adaptiveRuleAdjustment(event)
	}

	f.metrics.TotalAllowed++
	return true
}

// applyStaticRules applies static filter rules (minPriority, excludeNamespaces, includeTypes)
func (f *AdaptiveFilter) applyStaticRules(event *Event) bool {
	// Note: This is a simplified version. In a full implementation,
	// the actual filter configuration from config.FilterConfig would be used.
	// For now, we assume the filter has access to the full FilterConfig.

	// Priority check would be done here
	// Namespace exclusion would be done here
	// Type inclusion would be done here

	return true // Placeholder - actual implementation would use FilterConfig
}

// adaptiveRuleAdjustment adjusts adaptive rules based on event patterns
func (f *AdaptiveFilter) adaptiveRuleAdjustment(event *Event) {
	// This would implement adaptive learning logic
	// For now, it's a placeholder for the adaptive adjustment mechanism

	// Example: Track patterns and adjust rules based on:
	// - Event frequency
	// - Priority distribution
	// - Namespace patterns
	// - Type patterns
}

// GetMetrics returns current filter metrics
func (f *AdaptiveFilter) GetMetrics() *FilterMetrics {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &FilterMetrics{
		TotalProcessed: f.metrics.TotalProcessed,
		TotalAllowed:   f.metrics.TotalAllowed,
		TotalFiltered:  f.metrics.TotalFiltered,
		FalsePositives: f.metrics.FalsePositives,
		FalseNegatives: f.metrics.FalseNegatives,
		LastUpdated:    f.metrics.LastUpdated,
	}
}

// UpdateConfig updates the filter configuration
func (f *AdaptiveFilter) UpdateConfig(filterConfig config.FilterConfigAdvanced) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.rules = filterConfig.DynamicRules
	f.learningEnabled = filterConfig.AdaptiveEnabled
	f.adaptationRate = filterConfig.LearningRate

	return nil
}

// Reset resets filter metrics
func (f *AdaptiveFilter) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.metrics = &FilterMetrics{LastUpdated: time.Now()}
}
