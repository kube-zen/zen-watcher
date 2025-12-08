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
	"github.com/prometheus/client_golang/prometheus"
)

// TrafficAnalyzer monitors events per second and calculates traffic patterns
type TrafficAnalyzer struct {
	eventCounter  prometheus.Counter
	windowHistory []TrafficSample
	historyMu     sync.RWMutex
	currentWindow time.Duration
	windowMu      sync.RWMutex
	haConfig      *config.DedupOptimizationConfig
	lowThreshold  float64 // events/sec for low traffic
	highThreshold float64 // events/sec for high traffic
}

// TrafficSample represents a traffic measurement at a point in time
type TrafficSample struct {
	Timestamp    time.Time
	EventsPerSec float64
	WindowSize   time.Duration
}

// HADedupOptimizer manages dynamic dedup window optimization for HA deployments
type HADedupOptimizer struct {
	analyzer     *TrafficAnalyzer
	haConfig     *config.DedupOptimizationConfig
	updateTicker *time.Ticker
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewTrafficAnalyzer creates a new traffic analyzer
func NewTrafficAnalyzer(haConfig *config.DedupOptimizationConfig, eventCounter prometheus.Counter) *TrafficAnalyzer {
	lowThreshold := 50.0   // < 50 events/sec = low traffic
	highThreshold := 500.0 // > 500 events/sec = high traffic

	return &TrafficAnalyzer{
		eventCounter:  eventCounter,
		windowHistory: make([]TrafficSample, 0, 100),
		haConfig:      haConfig,
		lowThreshold:  lowThreshold,
		highThreshold: highThreshold,
	}
}

// RecordEvent records an event for traffic analysis
func (ta *TrafficAnalyzer) RecordEvent() {
	if ta.eventCounter != nil {
		ta.eventCounter.Inc()
	}
}

// CalculateEventsPerSecond calculates the average events per second over the last window
func (ta *TrafficAnalyzer) CalculateEventsPerSecond(window time.Duration) float64 {
	ta.historyMu.RLock()
	defer ta.historyMu.RUnlock()

	cutoff := time.Now().Add(-window)
	var totalEvents float64
	var count int

	for _, sample := range ta.windowHistory {
		if sample.Timestamp.After(cutoff) {
			totalEvents += sample.EventsPerSec
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalEvents / float64(count)
}

// GetOptimalWindow determines the optimal dedup window based on current traffic
func (ta *TrafficAnalyzer) GetOptimalWindow() time.Duration {
	ta.windowMu.RLock()
	defer ta.windowMu.RUnlock()

	if !ta.haConfig.Enabled || !ta.haConfig.AdaptiveWindows {
		// Return default if HA optimization is disabled
		return 60 * time.Second
	}

	// Calculate average events per second over last 5 minutes
	avgEventsPerSec := ta.CalculateEventsPerSecond(5 * time.Minute)

	// Determine window based on traffic thresholds
	var optimalWindow time.Duration
	if avgEventsPerSec < ta.lowThreshold {
		// Low traffic: use longer window
		optimalWindow, _ = ta.haConfig.GetLowTrafficWindow()
	} else if avgEventsPerSec > ta.highThreshold {
		// High traffic: use shorter window
		optimalWindow, _ = ta.haConfig.GetHighTrafficWindow()
	} else {
		// Medium traffic: use medium window (average of low and high)
		lowWindow, _ := ta.haConfig.GetLowTrafficWindow()
		highWindow, _ := ta.haConfig.GetHighTrafficWindow()
		optimalWindow = (lowWindow + highWindow) / 2
	}

	return optimalWindow
}

// UpdateWindow updates the current dedup window based on traffic analysis
func (ta *TrafficAnalyzer) UpdateWindow() {
	newWindow := ta.GetOptimalWindow()

	ta.windowMu.Lock()
	oldWindow := ta.currentWindow
	ta.currentWindow = newWindow
	ta.windowMu.Unlock()

	// Record the change in history
	ta.historyMu.Lock()
	ta.windowHistory = append(ta.windowHistory, TrafficSample{
		Timestamp:    time.Now(),
		EventsPerSec: ta.CalculateEventsPerSecond(1 * time.Minute),
		WindowSize:   newWindow,
	})
	// Keep only last 100 samples
	if len(ta.windowHistory) > 100 {
		ta.windowHistory = ta.windowHistory[len(ta.windowHistory)-100:]
	}
	ta.historyMu.Unlock()

	// Log window change if significant
	if oldWindow != 0 && (newWindow != oldWindow) {
		// Window changed - this will be logged by the caller
	}
}

// GetCurrentWindow returns the current dedup window
func (ta *TrafficAnalyzer) GetCurrentWindow() time.Duration {
	ta.windowMu.RLock()
	defer ta.windowMu.RUnlock()
	return ta.currentWindow
}

// NewHADedupOptimizer creates a new HA dedup optimizer
func NewHADedupOptimizer(haConfig *config.DedupOptimizationConfig, eventCounter prometheus.Counter) *HADedupOptimizer {
	if haConfig == nil || !haConfig.Enabled {
		return nil
	}

	analyzer := NewTrafficAnalyzer(haConfig, eventCounter)

	return &HADedupOptimizer{
		analyzer: analyzer,
		haConfig: haConfig,
		stopChan: make(chan struct{}),
	}
}

// Start begins the continuous optimization loop
func (opt *HADedupOptimizer) Start(updateInterval time.Duration) {
	if opt == nil {
		return
	}

	opt.updateTicker = time.NewTicker(updateInterval)
	opt.wg.Add(1)

	go func() {
		defer opt.wg.Done()
		for {
			select {
			case <-opt.updateTicker.C:
				opt.analyzer.UpdateWindow()
			case <-opt.stopChan:
				return
			}
		}
	}()
}

// Stop stops the optimization loop
func (opt *HADedupOptimizer) Stop() {
	if opt == nil {
		return
	}

	if opt.updateTicker != nil {
		opt.updateTicker.Stop()
	}
	close(opt.stopChan)
	opt.wg.Wait()
}

// GetOptimalWindow returns the current optimal dedup window
func (opt *HADedupOptimizer) GetOptimalWindow() time.Duration {
	if opt == nil || opt.analyzer == nil {
		return 60 * time.Second // default
	}
	return opt.analyzer.GetOptimalWindow()
}

// RecordEvent records an event for traffic analysis
func (opt *HADedupOptimizer) RecordEvent() {
	if opt != nil && opt.analyzer != nil {
		opt.analyzer.RecordEvent()
	}
}
