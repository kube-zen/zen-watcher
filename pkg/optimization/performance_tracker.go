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
)

// PerformanceData tracks performance metrics for a source
type PerformanceData struct {
	Source                 string
	AverageLatency         time.Duration
	PeakLatency            time.Duration
	TotalProcessed         int64
	ThroughputEventsPerSec float64
	LastUpdated            time.Time
}

// PerformanceTracker tracks performance metrics per source
type PerformanceTracker struct {
	source    string
	startTime time.Time
	endTime   time.Time
	active    bool

	// Latency tracking
	latencies    []time.Duration
	maxLatencies int

	// Counters
	totalProcessed int64

	mu sync.RWMutex
}

// NewPerformanceTracker creates a new performance tracker
func NewPerformanceTracker(source string) *PerformanceTracker {
	return &PerformanceTracker{
		source:       source,
		maxLatencies: 1000, // Keep last 1000 latency measurements
		latencies:    make([]time.Duration, 0, 1000),
	}
}

// StartProcessing marks the start of a processing batch
func (pt *PerformanceTracker) StartProcessing() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.startTime = time.Now()
	pt.active = true
}

// EndProcessing marks the end of a processing batch
func (pt *PerformanceTracker) EndProcessing() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.endTime = time.Now()
	pt.active = false
}

// RecordEvent records processing of a single event
func (pt *PerformanceTracker) RecordEvent(processingTime time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.totalProcessed++

	// Track latency
	if len(pt.latencies) >= pt.maxLatencies {
		// Remove oldest
		pt.latencies = pt.latencies[1:]
	}
	pt.latencies = append(pt.latencies, processingTime)
}

// GetAverageLatency returns the average processing latency
func (pt *PerformanceTracker) GetAverageLatency() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if len(pt.latencies) == 0 {
		return 0
	}

	var sum time.Duration
	for _, lat := range pt.latencies {
		sum += lat
	}

	return sum / time.Duration(len(pt.latencies))
}

// GetPeakLatency returns the peak processing latency
func (pt *PerformanceTracker) GetPeakLatency() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if len(pt.latencies) == 0 {
		return 0
	}

	var peak time.Duration
	for _, lat := range pt.latencies {
		if lat > peak {
			peak = lat
		}
	}

	return peak
}

// GetThroughput returns events processed per second
func (pt *PerformanceTracker) GetThroughput() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if !pt.active || pt.startTime.IsZero() {
		return 0
	}

	duration := time.Since(pt.startTime)
	if duration.Seconds() == 0 {
		return 0
	}

	return float64(pt.totalProcessed) / duration.Seconds()
}

// GetPerformanceData returns performance data for the source
func (pt *PerformanceTracker) GetPerformanceData() *PerformanceData {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return &PerformanceData{
		Source:                 pt.source,
		AverageLatency:         pt.GetAverageLatency(),
		PeakLatency:            pt.GetPeakLatency(),
		TotalProcessed:         pt.totalProcessed,
		ThroughputEventsPerSec: pt.GetThroughput(),
		LastUpdated:            time.Now(),
	}
}

// RecordStrategyDecision records which strategy was used
func (pt *PerformanceTracker) RecordStrategyDecision(source string, strategy ProcessingStrategy) {
	// This would be used for tracking strategy effectiveness
	// Implementation can be extended to track strategy-specific metrics
}

// Reset resets all performance tracking
func (pt *PerformanceTracker) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.startTime = time.Time{}
	pt.endTime = time.Time{}
	pt.active = false
	pt.totalProcessed = 0
	pt.latencies = pt.latencies[:0]
}
