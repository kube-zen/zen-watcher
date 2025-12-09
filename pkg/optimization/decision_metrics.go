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

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// decisionMetrics holds Prometheus metrics for optimization decisions
	decisionMetrics     *DecisionMetrics
	decisionMetricsOnce sync.Once
)

// DecisionMetrics holds Prometheus metrics for optimization decisions
type DecisionMetrics struct {
	decisionTotal      *prometheus.CounterVec
	decisionDuration   *prometheus.HistogramVec
	decisionConfidence *prometheus.GaugeVec
	stateVersion       prometheus.Gauge
}

// RegisterDecisionMetrics registers decision metrics with Prometheus
// This should be called during application startup
func RegisterDecisionMetrics() {
	metrics := getDecisionMetrics()
	prometheus.MustRegister(metrics)
}

// getDecisionMetrics returns the singleton DecisionMetrics instance
func getDecisionMetrics() *DecisionMetrics {
	decisionMetricsOnce.Do(func() {
		decisionMetrics = &DecisionMetrics{
			decisionTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "zen_watcher",
					Subsystem: "optimization",
					Name:      "decision_total",
					Help:      "Total number of optimization decisions made",
				},
				[]string{"type", "result", "source"},
			),
			decisionDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "zen_watcher",
					Subsystem: "optimization",
					Name:      "decision_duration_seconds",
					Help:      "Time taken to make optimization decisions",
					Buckets:   prometheus.DefBuckets,
				},
				[]string{"type", "source"},
			),
			decisionConfidence: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: "zen_watcher",
					Subsystem: "optimization",
					Name:      "decision_confidence",
					Help:      "Confidence level of optimization decisions (0.0-1.0)",
				},
				[]string{"type", "source"},
			),
			stateVersion: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Namespace: "zen_watcher",
					Subsystem: "optimization",
					Name:      "state_version",
					Help:      "Current version of optimization state",
				},
			),
		}
	})
	return decisionMetrics
}

// RecordDecision records a decision metric
func RecordDecision(decisionType, result, source string, duration time.Duration, confidence float64) {
	metrics := getDecisionMetrics()
	metrics.decisionTotal.WithLabelValues(decisionType, result, source).Inc()
	metrics.decisionDuration.WithLabelValues(decisionType, source).Observe(duration.Seconds())
	metrics.decisionConfidence.WithLabelValues(decisionType, source).Set(confidence)
}

// UpdateStateVersion updates the state version gauge
func UpdateStateVersion(version int) {
	metrics := getDecisionMetrics()
	metrics.stateVersion.Set(float64(version))
}

// Describe implements prometheus.Collector
func (dm *DecisionMetrics) Describe(ch chan<- *prometheus.Desc) {
	dm.decisionTotal.Describe(ch)
	dm.decisionDuration.Describe(ch)
	dm.decisionConfidence.Describe(ch)
	dm.stateVersion.Describe(ch)
}

// Collect implements prometheus.Collector
func (dm *DecisionMetrics) Collect(ch chan<- prometheus.Metric) {
	dm.decisionTotal.Collect(ch)
	dm.decisionDuration.Collect(ch)
	dm.decisionConfidence.Collect(ch)
	dm.stateVersion.Collect(ch)
}
