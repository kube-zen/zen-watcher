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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// HAMetrics holds HA-specific metrics for auto-scaling
type HAMetrics struct {
	CPUUsagePerReplica    *prometheus.GaugeVec
	MemoryUsagePerReplica *prometheus.GaugeVec
	EventsPerSecond       prometheus.Gauge
	QueueDepth            prometheus.Gauge
	ResponseTimePerEvent  *prometheus.HistogramVec
	ReplicaLoad           *prometheus.GaugeVec
	ScalingDecisions      *prometheus.CounterVec
}

// NewHAMetrics creates and registers HA-specific metrics
func NewHAMetrics() *HAMetrics {
	cpuUsagePerReplica := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_ha_cpu_usage_percent",
			Help: "CPU usage percentage per replica",
		},
		[]string{"replica_id"},
	)

	memoryUsagePerReplica := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_ha_memory_usage_bytes",
			Help: "Memory usage in bytes per replica",
		},
		[]string{"replica_id"},
	)

	eventsPerSecond := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "zen_watcher_ha_events_per_second",
			Help: "Events processed per second (aggregated across replicas)",
		},
	)

	queueDepth := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "zen_watcher_ha_queue_depth",
			Help: "Current queue depth (pending events)",
		},
	)

	responseTimePerEvent := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_ha_response_time_seconds",
			Help:    "Response time per event in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"replica_id"},
	)

	replicaLoad := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_ha_replica_load",
			Help: "Current load factor per replica (0.0-1.0)",
		},
		[]string{"replica_id"},
	)

	scalingDecisions := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_ha_scaling_decisions_total",
			Help: "Total number of scaling decisions made",
		},
		[]string{"action", "reason"}, // action: scale_up, scale_down, no_action
	)

	// Register all metrics
	prometheus.MustRegister(
		cpuUsagePerReplica,
		memoryUsagePerReplica,
		eventsPerSecond,
		queueDepth,
		responseTimePerEvent,
		replicaLoad,
		scalingDecisions,
	)

	return &HAMetrics{
		CPUUsagePerReplica:    cpuUsagePerReplica,
		MemoryUsagePerReplica: memoryUsagePerReplica,
		EventsPerSecond:       eventsPerSecond,
		QueueDepth:            queueDepth,
		ResponseTimePerEvent:  responseTimePerEvent,
		ReplicaLoad:           replicaLoad,
		ScalingDecisions:      scalingDecisions,
	}
}
