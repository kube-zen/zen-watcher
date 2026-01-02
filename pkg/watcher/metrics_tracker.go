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

package watcher

import "github.com/prometheus/client_golang/prometheus"

// MetricsTracker groups all Prometheus metrics for observation creation
type MetricsTracker struct {
	EventsTotal              *prometheus.CounterVec
	ObservationsCreated     *prometheus.CounterVec
	ObservationsFiltered    *prometheus.CounterVec
	ObservationsDeduped      prometheus.Counter
	ObservationsCreateErrors *prometheus.CounterVec
}

// NewMetricsTracker creates a new MetricsTracker
func NewMetricsTracker(
	eventsTotal *prometheus.CounterVec,
	observationsCreated *prometheus.CounterVec,
	observationsFiltered *prometheus.CounterVec,
	observationsDeduped prometheus.Counter,
	observationsCreateErrors *prometheus.CounterVec,
) *MetricsTracker {
	return &MetricsTracker{
		EventsTotal:              eventsTotal,
		ObservationsCreated:      observationsCreated,
		ObservationsFiltered:     observationsFiltered,
		ObservationsDeduped:       observationsDeduped,
		ObservationsCreateErrors: observationsCreateErrors,
	}
}

