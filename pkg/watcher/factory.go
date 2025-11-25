package watcher

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// NewProcessors creates all event processors
func NewProcessors(
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	eventsTotal *prometheus.CounterVec,
	eventProcessingDuration *prometheus.HistogramVec,
) (*EventProcessor, *WebhookProcessor) {
	eventProcessor := NewEventProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration)
	webhookProcessor := NewWebhookProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration)
	return eventProcessor, webhookProcessor
}
