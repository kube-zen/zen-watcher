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
	observationsCreateErrors *prometheus.CounterVec,
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
		observationsCreateErrors,
		filter,
	)

	eventProcessor := NewEventProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration, observationCreator)
	webhookProcessor := NewWebhookProcessor(dynClient, eventGVR, eventsTotal, eventProcessingDuration, observationCreator)
	return eventProcessor, webhookProcessor, observationCreator
}
