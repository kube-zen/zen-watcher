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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for zen-watcher
type Metrics struct {
	// Core event metrics
	EventsTotal              *prometheus.CounterVec
	ObservationsCreated      *prometheus.CounterVec
	ObservationsFiltered     *prometheus.CounterVec
	ObservationsDeduped      prometheus.Counter
	ObservationsDeleted      *prometheus.CounterVec
	ObservationsCreateErrors *prometheus.CounterVec

	// Filter metrics (NEW)
	FilterDecisions      *prometheus.CounterVec
	FilterReloadTotal    *prometheus.CounterVec
	FilterLastReload     *prometheus.GaugeVec
	FilterPoliciesActive *prometheus.GaugeVec

	// ObservationMapping / CRD adapter metrics (NEW)
	ObservationMappingsActive *prometheus.GaugeVec
	ObservationMappingsEvents *prometheus.CounterVec
	CRDAdapterErrors          *prometheus.CounterVec

	// Adapter lifecycle metrics (NEW)
	AdapterRunsTotal *prometheus.CounterVec

	// Webhook metrics (enhanced)
	WebhookRequests   *prometheus.CounterVec
	WebhookDropped    *prometheus.CounterVec
	WebhookQueueUsage *prometheus.GaugeVec // NEW

	// Dedup metrics (enhanced - NEW)
	DedupCacheUsage *prometheus.GaugeVec
	DedupEvictions  *prometheus.CounterVec

	// GC metrics
	GCRunsTotal      prometheus.Counter
	GCDuration       *prometheus.HistogramVec
	GCErrors         *prometheus.CounterVec
	ObservationsLive *prometheus.GaugeVec // NEW

	// Performance & health metrics
	ToolsActive             *prometheus.GaugeVec
	InformerCacheSync       *prometheus.GaugeVec
	EventProcessingDuration *prometheus.HistogramVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	eventsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_events_total",
			Help: "Total number of events that resulted in Observation CRD creation (after filtering and deduplication), grouped by source, category, and severity",
		},
		[]string{"source", "category", "severity"},
	)

	toolsActive := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_tools_active",
			Help: "Number of security tools currently detected (1=active, 0=inactive)",
		},
		[]string{"tool"},
	)

	informerCacheSync := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_informer_cache_synced",
			Help: "Informer cache sync status (1=synced, 0=not synced)",
		},
		[]string{"resource"},
	)

	eventProcessingDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_event_processing_duration_seconds",
			Help:    "Time taken to process and create an Observation",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"source", "processor_type"},
	)

	webhookRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_webhook_requests_total",
			Help: "Total number of webhook requests received",
		},
		[]string{"endpoint", "status"},
	)

	webhookDropped := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_webhook_events_dropped_total",
			Help: "Total number of webhook events dropped due to channel full (backpressure)",
		},
		[]string{"endpoint"},
	)

	observationsCreated := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_observations_created_total",
			Help: "Total number of Observation CRDs successfully created",
		},
		[]string{"source"},
	)

	observationsFiltered := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_observations_filtered_total",
			Help: "Total number of observations filtered out by source-level filtering rules",
		},
		[]string{"source", "reason"},
	)

	observationsDeduped := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "zen_watcher_observations_deduped_total",
			Help: "Total number of observations skipped due to deduplication (within sliding window)",
		},
	)

	observationsDeleted := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_observations_deleted_total",
			Help: "Total number of Observations deleted by garbage collector",
		},
		[]string{"source", "reason"},
	)

	gcRunsTotal := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "zen_watcher_gc_runs_total",
			Help: "Total number of garbage collection runs",
		},
	)

	gcDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_gc_duration_seconds",
			Help:    "Time taken to run garbage collection",
			Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
		},
		[]string{"operation"},
	)

	observationsCreateErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_observations_create_errors_total",
			Help: "Total number of errors encountered while creating Observation CRDs",
		},
		[]string{"source", "error_type"},
	)

	gcErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_gc_errors_total",
			Help: "Total number of errors encountered during garbage collection",
		},
		[]string{"operation", "error_type"},
	)

	// NEW: Filter metrics
	filterDecisions := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_filter_decisions_total",
			Help: "Total filter decisions by action and reason",
		},
		[]string{"source", "action", "reason"},
	)

	filterReloadTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_filter_reload_total",
			Help: "Total filter config reloads",
		},
		[]string{"source", "result"},
	)

	filterLastReload := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_filter_last_reload_timestamp_seconds",
			Help: "Timestamp of last successful filter reload",
		},
		[]string{"source"},
	)

	filterPoliciesActive := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_filter_policies_active",
			Help: "Number of active filter policies",
		},
		[]string{"type"},
	)

	// NEW: ObservationMapping metrics
	observationMappingsActive := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_observation_mappings_active",
			Help: "Active ObservationMapping CRDs",
		},
		[]string{"mapping", "group", "version", "kind", "namespace_scope"},
	)

	observationMappingsEvents := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_observation_mappings_events_total",
			Help: "Events processed by ObservationMapping",
		},
		[]string{"mapping", "result"},
	)

	crdAdapterErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_crd_adapter_errors_total",
			Help: "CRD adapter errors",
		},
		[]string{"mapping", "stage", "error_type"},
	)

	// NEW: Adapter lifecycle metrics
	adapterRunsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_adapter_runs_total",
			Help: "Adapter run iterations",
		},
		[]string{"adapter", "outcome"},
	)

	// NEW: Enhanced webhook metrics
	webhookQueueUsage := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_webhook_queue_usage_ratio",
			Help: "Webhook queue utilization ratio",
		},
		[]string{"endpoint"},
	)

	// NEW: Dedup cache metrics
	dedupCacheUsage := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_dedup_cache_usage_ratio",
			Help: "Dedup cache utilization ratio",
		},
		[]string{"source"},
	)

	dedupEvictions := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_dedup_evictions_total",
			Help: "Dedup cache evictions",
		},
		[]string{"source"},
	)

	// NEW: GC footprint metric
	observationsLive := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_observations_live",
			Help: "Current live Observation CRs in etcd",
		},
		[]string{"source"},
	)

	// Register all metrics
	prometheus.MustRegister(eventsTotal)
	prometheus.MustRegister(observationsCreated)
	prometheus.MustRegister(observationsFiltered)
	prometheus.MustRegister(observationsDeduped)
	prometheus.MustRegister(observationsDeleted)
	prometheus.MustRegister(observationsCreateErrors)
	prometheus.MustRegister(gcRunsTotal)
	prometheus.MustRegister(gcDuration)
	prometheus.MustRegister(gcErrors)
	prometheus.MustRegister(toolsActive)
	prometheus.MustRegister(informerCacheSync)
	prometheus.MustRegister(eventProcessingDuration)
	prometheus.MustRegister(webhookRequests)
	prometheus.MustRegister(webhookDropped)

	// Register NEW metrics
	prometheus.MustRegister(filterDecisions)
	prometheus.MustRegister(filterReloadTotal)
	prometheus.MustRegister(filterLastReload)
	prometheus.MustRegister(filterPoliciesActive)
	prometheus.MustRegister(observationMappingsActive)
	prometheus.MustRegister(observationMappingsEvents)
	prometheus.MustRegister(crdAdapterErrors)
	prometheus.MustRegister(adapterRunsTotal)
	prometheus.MustRegister(webhookQueueUsage)
	prometheus.MustRegister(dedupCacheUsage)
	prometheus.MustRegister(dedupEvictions)
	prometheus.MustRegister(observationsLive)

	return &Metrics{
		// Core event metrics
		EventsTotal:              eventsTotal,
		ObservationsCreated:      observationsCreated,
		ObservationsFiltered:     observationsFiltered,
		ObservationsDeduped:      observationsDeduped,
		ObservationsDeleted:      observationsDeleted,
		ObservationsCreateErrors: observationsCreateErrors,

		// Filter metrics (NEW)
		FilterDecisions:      filterDecisions,
		FilterReloadTotal:    filterReloadTotal,
		FilterLastReload:     filterLastReload,
		FilterPoliciesActive: filterPoliciesActive,

		// ObservationMapping / CRD adapter metrics (NEW)
		ObservationMappingsActive: observationMappingsActive,
		ObservationMappingsEvents: observationMappingsEvents,
		CRDAdapterErrors:          crdAdapterErrors,

		// Adapter lifecycle metrics (NEW)
		AdapterRunsTotal: adapterRunsTotal,

		// Webhook metrics
		WebhookRequests:   webhookRequests,
		WebhookDropped:    webhookDropped,
		WebhookQueueUsage: webhookQueueUsage, // NEW

		// Dedup metrics (NEW)
		DedupCacheUsage: dedupCacheUsage,
		DedupEvictions:  dedupEvictions,

		// GC metrics
		GCRunsTotal:      gcRunsTotal,
		GCDuration:       gcDuration,
		GCErrors:         gcErrors,
		ObservationsLive: observationsLive, // NEW

		// Performance & health metrics
		ToolsActive:             toolsActive,
		InformerCacheSync:       informerCacheSync,
		EventProcessingDuration: eventProcessingDuration,
	}
}
