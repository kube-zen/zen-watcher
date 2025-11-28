package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for zen-watcher
type Metrics struct {
	EventsTotal             *prometheus.CounterVec
	ObservationsCreated     *prometheus.CounterVec
	ObservationsFiltered    *prometheus.CounterVec
	ObservationsDeduped     prometheus.Counter
	ToolsActive             *prometheus.GaugeVec
	InformerCacheSync       *prometheus.GaugeVec
	EventProcessingDuration *prometheus.HistogramVec
	WebhookRequests         *prometheus.CounterVec
	WebhookDropped          *prometheus.CounterVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	eventsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_events_total",
			Help: "Total number of Observations created by source",
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

	// Register all metrics
	prometheus.MustRegister(eventsTotal)
	prometheus.MustRegister(observationsCreated)
	prometheus.MustRegister(observationsFiltered)
	prometheus.MustRegister(observationsDeduped)
	prometheus.MustRegister(toolsActive)
	prometheus.MustRegister(informerCacheSync)
	prometheus.MustRegister(eventProcessingDuration)
	prometheus.MustRegister(webhookRequests)
	prometheus.MustRegister(webhookDropped)

	return &Metrics{
		EventsTotal:             eventsTotal,
		ObservationsCreated:     observationsCreated,
		ObservationsFiltered:    observationsFiltered,
		ObservationsDeduped:     observationsDeduped,
		ToolsActive:             toolsActive,
		InformerCacheSync:       informerCacheSync,
		EventProcessingDuration: eventProcessingDuration,
		WebhookRequests:         webhookRequests,
		WebhookDropped:          webhookDropped,
	}
}
