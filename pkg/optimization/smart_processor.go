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
	"context"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
)

// RawEvent represents a raw event from a source
type RawEvent struct {
	Source    string
	RawData   map[string]interface{}
	Timestamp time.Time
}

// SmartProcessor processes events using intelligent optimization strategies
type SmartProcessor struct {
	strategyDecider     *StrategyDecider
	metricsCollectors   map[string]*PerSourceMetricsCollector
	performanceTrackers map[string]*PerformanceTracker
	mu                  sync.RWMutex // Protects maps from concurrent access
}

// NewSmartProcessor creates a new SmartProcessor
func NewSmartProcessor() *SmartProcessor {
	return &SmartProcessor{
		strategyDecider:     NewStrategyDecider(),
		metricsCollectors:   make(map[string]*PerSourceMetricsCollector),
		performanceTrackers: make(map[string]*PerformanceTracker),
	}
}

// GetOrCreateMetricsCollector gets or creates a metrics collector for a source
func (sp *SmartProcessor) GetOrCreateMetricsCollector(source string) *PerSourceMetricsCollector {
	sp.mu.RLock()
	if collector, exists := sp.metricsCollectors[source]; exists {
		sp.mu.RUnlock()
		return collector
	}
	sp.mu.RUnlock()

	sp.mu.Lock()
	defer sp.mu.Unlock()
	// Double-check after acquiring write lock
	if collector, exists := sp.metricsCollectors[source]; exists {
		return collector
	}

	collector := NewPerSourceMetricsCollector(source)
	sp.metricsCollectors[source] = collector
	return collector
}

// GetOrCreatePerformanceTracker gets or creates a performance tracker for a source
func (sp *SmartProcessor) GetOrCreatePerformanceTracker(source string) *PerformanceTracker {
	sp.mu.RLock()
	if tracker, exists := sp.performanceTrackers[source]; exists {
		sp.mu.RUnlock()
		return tracker
	}
	sp.mu.RUnlock()

	sp.mu.Lock()
	defer sp.mu.Unlock()
	// Double-check after acquiring write lock
	if tracker, exists := sp.performanceTrackers[source]; exists {
		return tracker
	}

	tracker := NewPerformanceTracker(source)
	sp.performanceTrackers[source] = tracker
	return tracker
}

// DetermineOptimalStrategy determines the optimal processing strategy for a source
func (sp *SmartProcessor) DetermineOptimalStrategy(
	raw *RawEvent,
	sourceConfig *generic.SourceConfig,
) ProcessingStrategy {
	// Get current metrics for this source
	collector := sp.GetOrCreateMetricsCollector(raw.Source)
	metrics := collector.GetOptimizationMetrics()

	// Use StrategyDecider to determine optimal strategy
	return sp.strategyDecider.DetermineStrategy(metrics, sourceConfig)
}

// ProcessEvent processes an event using the optimal strategy determined by the SmartProcessor
// This is a high-level interface - actual processing would call into the observation creator
func (sp *SmartProcessor) ProcessEvent(
	ctx context.Context,
	raw *RawEvent,
	sourceConfig *generic.SourceConfig,
) error {
	startTime := time.Now()

	// Get components for this source
	collector := sp.GetOrCreateMetricsCollector(raw.Source)
	tracker := sp.GetOrCreatePerformanceTracker(raw.Source)

	// Determine optimal strategy
	strategy := sp.DetermineOptimalStrategy(raw, sourceConfig)
	tracker.RecordStrategyDecision(raw.Source, strategy)

	// Note: Actual filtering and deduplication are handled by the processor pipeline,
	// not by SmartProcessor. This function is for metrics collection only.

	// Record processing metrics
	processingTime := time.Since(startTime)
	collector.RecordProcessing(processingTime, nil)
	tracker.RecordEvent(processingTime)

	return nil
}

// GetSourceMetrics returns metrics for a source
func (sp *SmartProcessor) GetSourceMetrics(source string) *OptimizationMetrics {
	collector := sp.GetOrCreateMetricsCollector(source)
	return collector.GetOptimizationMetrics()
}

// GetAllMetricsCollectors returns all metrics collectors
func (sp *SmartProcessor) GetAllMetricsCollectors() map[string]*PerSourceMetricsCollector {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	result := make(map[string]*PerSourceMetricsCollector)
	for k, v := range sp.metricsCollectors {
		result[k] = v
	}
	return result
}
