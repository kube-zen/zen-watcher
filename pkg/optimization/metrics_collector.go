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

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsWindow represents a sliding window of metrics
type MetricsWindow struct {
	windowSize   time.Duration
	entries      []MetricsEntry
	mu           sync.RWMutex
	maxEntries   int
}

// MetricsEntry represents a single metrics entry in the window
type MetricsEntry struct {
	Timestamp time.Time
	Metrics   map[string]float64
}

// NewMetricsWindow creates a new metrics window
func NewMetricsWindow(windowSize time.Duration) *MetricsWindow {
	return &MetricsWindow{
		windowSize: windowSize,
		entries:    make([]MetricsEntry, 0),
		maxEntries: 1000, // Maximum entries to keep in memory
	}
}

// AddMetric adds a metric to the window
func (mw *MetricsWindow) AddMetric(name string, value float64) {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	now := time.Now()
	
	// Remove old entries outside window
	mw.cleanup(now)

	// Add new entry
	entry := MetricsEntry{
		Timestamp: now,
		Metrics:   map[string]float64{name: value},
	}
	
	mw.entries = append(mw.entries, entry)

	// Trim if too many entries
	if len(mw.entries) > mw.maxEntries {
		mw.entries = mw.entries[len(mw.entries)-mw.maxEntries:]
	}
}

// cleanup removes entries older than the window
func (mw *MetricsWindow) cleanup(now time.Time) {
	cutoff := now.Add(-mw.windowSize)
	
	// Find first entry within window
	startIdx := 0
	for i, entry := range mw.entries {
		if entry.Timestamp.After(cutoff) {
			startIdx = i
			break
		}
	}

	// Remove old entries
	if startIdx > 0 {
		mw.entries = mw.entries[startIdx:]
	}
}

// GetWindowMetrics returns aggregated metrics for the current window
func (mw *MetricsWindow) GetWindowMetrics() map[string]float64 {
	mw.mu.RLock()
	defer mw.mu.RUnlock()

	result := make(map[string]float64)
	counts := make(map[string]int)

	cutoff := time.Now().Add(-mw.windowSize)

	// Aggregate metrics in window
	for _, entry := range mw.entries {
		if entry.Timestamp.Before(cutoff) {
			continue
		}

		for name, value := range entry.Metrics {
			result[name] += value
			counts[name]++
		}
	}

	// Calculate averages where appropriate
	// (For counters, keep sum; for rates, calculate average)
	avgMetrics := []string{"processing_latency", "cpu_usage", "memory_usage"}
	for _, name := range avgMetrics {
		if count, ok := counts[name]; ok && count > 0 {
			result[name+"_avg"] = result[name] / float64(count)
		}
	}

	return result
}

// GetMetric returns a specific metric value from the window
func (mw *MetricsWindow) GetMetric(name string) float64 {
	metrics := mw.GetWindowMetrics()
	return metrics[name]
}

// PerSourceMetricsCollector collects and aggregates metrics per source
type PerSourceMetricsCollector struct {
	source             string
	collectionInterval time.Duration
	window             *MetricsWindow
	
	// Counters
	eventsProcessed    int64
	eventsFiltered     int64
	eventsDeduped      int64
	
	// Quality metrics
	filterEffectiveness float64
	dedupEffectiveness  float64
	
	// Prometheus metrics
	promEventsProcessed    prometheus.Counter
	promEventsFiltered     prometheus.Counter
	promEventsDeduped      prometheus.Counter
	promProcessingLatency  prometheus.Histogram
	promFilterEffectiveness prometheus.Gauge
	promDedupEffectiveness  prometheus.Gauge
	promObservationsPerMin  prometheus.Gauge
	
	mu sync.RWMutex
}

// NewPerSourceMetricsCollector creates a new per-source metrics collector
func NewPerSourceMetricsCollector(source string) *PerSourceMetricsCollector {
	labels := prometheus.Labels{"source": source}
	
	return &PerSourceMetricsCollector{
		source:             source,
		collectionInterval: 30 * time.Second,
		window:             NewMetricsWindow(10 * time.Minute),
		
		// Initialize Prometheus metrics
		promEventsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "zen_watcher",
			Subsystem:   "optimization",
			Name:        "source_events_processed_total",
			Help:        "Total number of events processed per source",
			ConstLabels: labels,
		}),
		promEventsFiltered: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "zen_watcher",
			Subsystem:   "optimization",
			Name:        "source_events_filtered_total",
			Help:        "Total number of events filtered per source",
			ConstLabels: labels,
		}),
		promEventsDeduped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "zen_watcher",
			Subsystem:   "optimization",
			Name:        "source_events_deduped_total",
			Help:        "Total number of events deduplicated per source",
			ConstLabels: labels,
		}),
		promProcessingLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   "zen_watcher",
			Subsystem:   "optimization",
			Name:        "source_processing_latency_seconds",
			Help:        "Processing latency per source in seconds",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}),
		promFilterEffectiveness: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "zen_watcher",
			Subsystem:   "optimization",
			Name:        "filter_effectiveness_ratio",
			Help:        "Filter effectiveness ratio per source (0.0-1.0)",
			ConstLabels: labels,
		}),
		promDedupEffectiveness: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "zen_watcher",
			Subsystem:   "optimization",
			Name:        "deduplication_rate_ratio",
			Help:        "Deduplication rate ratio per source (0.0-1.0)",
			ConstLabels: labels,
		}),
		promObservationsPerMin: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "zen_watcher",
			Subsystem:   "optimization",
			Name:        "observations_per_minute",
			Help:        "Observations per minute per source",
			ConstLabels: labels,
		}),
	}
}

// RecordProcessing records a processing event
func (c *PerSourceMetricsCollector) RecordProcessing(
	processingTime time.Duration,
	err error,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventsProcessed++
	c.window.AddMetric("events_processed", 1)
	c.window.AddMetric("processing_latency", float64(processingTime.Milliseconds()))

	// Update Prometheus metrics
	c.promEventsProcessed.Inc()
	c.promProcessingLatency.Observe(processingTime.Seconds())

	if err != nil {
		c.window.AddMetric("errors", 1)
	}

	c.updateDerivedMetrics()
}

// RecordFiltered records a filtered event
func (c *PerSourceMetricsCollector) RecordFiltered(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventsFiltered++
	c.window.AddMetric("events_filtered", 1)
	c.window.AddMetric("filter_reason_"+reason, 1)

	// Update Prometheus metrics
	c.promEventsFiltered.Inc()

	c.updateDerivedMetrics()
}

// RecordDeduped records a deduplicated event
func (c *PerSourceMetricsCollector) RecordDeduped() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventsDeduped++
	c.window.AddMetric("events_deduped", 1)

	// Update Prometheus metrics
	c.promEventsDeduped.Inc()

	c.updateDerivedMetrics()
}

// updateDerivedMetrics updates computed metrics
func (c *PerSourceMetricsCollector) updateDerivedMetrics() {
	processed := c.eventsProcessed
	filtered := c.eventsFiltered
	deduped := c.eventsDeduped

	if processed > 0 {
		c.filterEffectiveness = float64(filtered) / float64(processed)
		c.dedupEffectiveness = float64(deduped) / float64(processed)
		
		// Update Prometheus gauges
		c.promFilterEffectiveness.Set(c.filterEffectiveness)
		c.promDedupEffectiveness.Set(c.dedupEffectiveness)
	}
}

// GetOptimizationMetrics returns optimization metrics for decision making
func (c *PerSourceMetricsCollector) GetOptimizationMetrics() *OptimizationMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	windowMetrics := c.window.GetWindowMetrics()

	// Calculate observations per minute
	processed := c.window.GetMetric("events_processed")
	observationsPerMinute := (processed / 10.0) * 6.0 // Assuming 10-minute window
	
	// Update Prometheus gauge
	c.promObservationsPerMin.Set(observationsPerMinute)

	return &OptimizationMetrics{
		Source:                c.source,
		EventsProcessed:       int64(c.eventsProcessed),
		EventsFiltered:        int64(c.eventsFiltered),
		EventsDeduped:         int64(c.eventsDeduped),
		ProcessingLatency:     int64(windowMetrics["processing_latency_avg"]),
		DeduplicationRate:     c.dedupEffectiveness,
		FilterEffectiveness:   c.filterEffectiveness,
		ObservationsPerMinute: observationsPerMinute,
		LowSeverityPercent:    0.0, // Would be populated from actual event data
	}
}

// Describe implements prometheus.Collector
func (c *PerSourceMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.promEventsProcessed.Describe(ch)
	c.promEventsFiltered.Describe(ch)
	c.promEventsDeduped.Describe(ch)
	c.promProcessingLatency.Describe(ch)
	c.promFilterEffectiveness.Describe(ch)
	c.promDedupEffectiveness.Describe(ch)
	c.promObservationsPerMin.Describe(ch)
}

// Collect implements prometheus.Collector
func (c *PerSourceMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	c.promEventsProcessed.Collect(ch)
	c.promEventsFiltered.Collect(ch)
	c.promEventsDeduped.Collect(ch)
	c.promProcessingLatency.Collect(ch)
	c.promFilterEffectiveness.Collect(ch)
	c.promDedupEffectiveness.Collect(ch)
	c.promObservationsPerMin.Collect(ch)
}

// Reset resets all metrics
func (c *PerSourceMetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventsProcessed = 0
	c.eventsFiltered = 0
	c.eventsDeduped = 0
	c.filterEffectiveness = 0.0
	c.dedupEffectiveness = 0.0
	c.window = NewMetricsWindow(10 * time.Minute)
}

