package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Events metrics
	EventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_events_total",
			Help: "Total number of events collected by category, source, and severity",
		},
		[]string{"category", "source", "event_type", "severity"},
	)

	EventsWritten = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_events_written_total",
			Help: "Total number of events successfully written to CRDs",
		},
		[]string{"category", "source"},
	)

	EventsFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_events_failures_total",
			Help: "Total number of failed event writes",
		},
		[]string{"category", "source", "reason"},
	)

	// Active events gauge
	ActiveEvents = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_active_events",
			Help: "Number of currently active (unresolved) events",
		},
		[]string{"category", "severity"},
	)

	// Watcher metrics
	WatcherStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_watcher_status",
			Help: "Status of each watcher (1=enabled, 0=disabled)",
		},
		[]string{"watcher"},
	)

	WatcherErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_watcher_errors_total",
			Help: "Total number of watcher errors",
		},
		[]string{"watcher", "error_type"},
	)

	WatcherScrapeDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_scrape_duration_seconds",
			Help:    "Duration of watcher scrape operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"watcher"},
	)

	// CRD operations
	CRDOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_crd_operations_total",
			Help: "Total number of CRD operations",
		},
		[]string{"operation", "status"},
	)

	CRDOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_crd_operation_duration_seconds",
			Help:    "Duration of CRD operations",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation"},
	)

	// API metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"endpoint", "method", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"endpoint", "method"},
	)

	// Resource metrics
	GoRoutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zen_watcher_goroutines",
			Help: "Number of goroutines",
		},
	)

	// Build info
	BuildInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_build_info",
			Help: "Build information",
		},
	)

	// Health metrics
	HealthStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zen_watcher_health_status",
			Help: "Overall health status (1=healthy, 0=unhealthy)",
		},
	)

	ReadinessStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zen_watcher_readiness_status",
			Help: "Readiness status (1=ready, 0=not ready)",
		},
	)

	// Tool-specific metrics
	ToolEventsCollected = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_tool_events_collected_total",
			Help: "Number of events collected per tool",
		},
		[]string{"tool"},
	)

	ToolLastScrapeTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_tool_last_scrape_timestamp_seconds",
			Help: "Timestamp of last successful scrape per tool",
		},
		[]string{"tool"},
	)

	// Event processing metrics
	EventProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_event_processing_duration_seconds",
			Help:    "Duration of event processing",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"source"},
	)

	EventQueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zen_watcher_event_queue_depth",
			Help: "Current depth of event processing queue",
		},
		[]string{"source"},
	)

	// Kubernetes API client metrics
	K8sAPIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_k8s_api_requests_total",
			Help: "Total number of Kubernetes API requests",
		},
		[]string{"resource", "verb", "status"},
	)

	K8sAPIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_k8s_api_request_duration_seconds",
			Help:    "Duration of Kubernetes API requests",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"resource", "verb"},
	)
)

// SetBuildInfo sets the build information metric
}

// SetHealthStatus sets the health status metric
func SetHealthStatus(healthy bool) {
	if healthy {
		HealthStatus.Set(1)
	} else {
		HealthStatus.Set(0)
	}
}

// SetReadinessStatus sets the readiness status metric
func SetReadinessStatus(ready bool) {
	if ready {
		ReadinessStatus.Set(1)
	} else {
		ReadinessStatus.Set(0)
	}
}

// RecordEvent records an event metric
func RecordEvent(category, source, eventType, severity string) {
	EventsTotal.WithLabelValues(category, source, eventType, severity).Inc()
}

// RecordEventWritten records a successful event write
func RecordEventWritten(category, source string) {
	EventsWritten.WithLabelValues(category, source).Inc()
}

// RecordEventFailure records a failed event write
func RecordEventFailure(category, source, reason string) {
	EventsFailures.WithLabelValues(category, source, reason).Inc()
}

// UpdateActiveEvents updates the active events gauge
func UpdateActiveEvents(category, severity string, count float64) {
	ActiveEvents.WithLabelValues(category, severity).Set(count)
}

// SetWatcherStatus sets the watcher status
func SetWatcherStatus(watcher string, enabled bool) {
	if enabled {
		WatcherStatus.WithLabelValues(watcher).Set(1)
	} else {
		WatcherStatus.WithLabelValues(watcher).Set(0)
	}
}

// RecordWatcherError records a watcher error
func RecordWatcherError(watcher, errorType string) {
	WatcherErrors.WithLabelValues(watcher, errorType).Inc()
}

// RecordHTTPRequest records an HTTP request
func RecordHTTPRequest(endpoint, method, status string) {
	HTTPRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
}


