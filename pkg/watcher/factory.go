package watcher

import (
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// NewProcessors creates all event processors with centralized observation creator and filter
func NewProcessors(
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	eventsTotal *prometheus.CounterVec,
	observationsCreated *prometheus.CounterVec,
	observationsFiltered *prometheus.CounterVec,
	observationsDeduped prometheus.Counter,
	eventProcessingDuration *prometheus.HistogramVec,
	filter *filter.Filter,
) (*EventProcessor, *WebhookProcessor, *ObservationCreator) {
	// Create centralized observation creator with filter - this is the ONLY place where Observations are created
	// Flow: filter() → normalize() → dedup() → create Observation CRD + update metrics + log
	observationCreator := NewObservationCreator(
		dynClient,
		eventGVR,
		eventsTotal,
		observationsCreated,
		observationsFiltered,
		observationsDeduped,
		filter,
	)

	eventProcessor := NewEventProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration, observationCreator)
	webhookProcessor := NewWebhookProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration, observationCreator)
	return eventProcessor, webhookProcessor, observationCreator
}
