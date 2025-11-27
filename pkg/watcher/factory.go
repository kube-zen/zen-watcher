package watcher

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// NewProcessors creates all event processors with centralized observation creator
func NewProcessors(
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	eventsTotal *prometheus.CounterVec,
	eventProcessingDuration *prometheus.HistogramVec,
) (*EventProcessor, *WebhookProcessor, *ObservationCreator) {
	// Create centralized observation creator - this is the ONLY place where Observations are created
	observationCreator := NewObservationCreator(dynClient, eventGVR, eventsTotal)

	eventProcessor := NewEventProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration, observationCreator)
	webhookProcessor := NewWebhookProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration, observationCreator)
	return eventProcessor, webhookProcessor, observationCreator
}
