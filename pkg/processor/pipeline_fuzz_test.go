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

//go:build go1.18
// +build go1.18

package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-sdk/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

// FuzzProcessEvent fuzzes the ProcessEvent function with random event payloads
func FuzzProcessEvent(f *testing.F) {
	// Seed corpus with valid examples
	seedEvents := []string{
		`{"source":"trivy","severity":"HIGH","id":"test-1"}`,
		`{"source":"falco","priority":"critical","rule":"test-rule"}`,
		`{"source":"kyverno","severity":"MEDIUM","policy":"test-policy"}`,
	}

	for _, seed := range seedEvents {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, eventJSON string) {
		// Parse JSON (may fail, that's OK for fuzzing)
		var eventData map[string]interface{}
		if err := json.Unmarshal([]byte(eventJSON), &eventData); err != nil {
			t.Skip() // Invalid JSON, skip
		}

		// Setup processor
		observationGVR := schema.GroupVersionResource{
			Group:    "zen.kube-zen.io",
			Version:  "v1",
			Resource: "observations",
		}
		dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, map[schema.GroupVersionResource]string{
			observationGVR: "ObservationList",
		})
		filterConfig := &filter.FilterConfig{Sources: make(map[string]filter.SourceFilter)}
		f := filter.NewFilter(filterConfig)
		deduper := sdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000
		creator := watcher.NewObservationCreator(
			dynamicClient,
			observationGVR,
			nil, // eventsTotal
			nil, // observationsCreated
			nil, // observationsFiltered
			nil, // observationsDeduped
			nil, // observationsCreateErrors
			f,   // filter
		)
		proc := NewProcessor(f, deduper, creator)

		// Create raw event
		rawEvent := &generic.RawEvent{
			Source:    "fuzz-source",
			Timestamp: time.Now(), // Use current time
			RawData:   eventData,
		}

		sourceConfig := &generic.SourceConfig{
			Source: "fuzz-source",
			Normalization: &generic.NormalizationConfig{
				Domain: "security",
				Type:   "fuzz-event",
				Priority: map[string]float64{
					"HIGH":   0.8,
					"MEDIUM": 0.5,
					"LOW":    0.3,
				},
			},
		}

		// Process event - should not panic
		ctx := context.Background()
		_ = proc.ProcessEvent(ctx, rawEvent, sourceConfig)
		// We don't check for errors - fuzzing is about finding panics
	})
}

// FuzzProcessEvent_ExtremeSizes fuzzes with extreme payload sizes
func FuzzProcessEvent_ExtremeSizes(f *testing.F) {
	f.Add(1)     // Small
	f.Add(100)   // Medium
	f.Add(10000) // Large

	f.Fuzz(func(t *testing.T, size int) {
		// Limit size to prevent OOM
		if size > 100000 {
			t.Skip()
		}

		// Generate event with specified size
		eventData := make(map[string]interface{})
		eventData["source"] = "fuzz-source"
		eventData["data"] = make([]byte, size)

		// Setup processor
		observationGVR := schema.GroupVersionResource{
			Group:    "zen.kube-zen.io",
			Version:  "v1",
			Resource: "observations",
		}
		dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, map[schema.GroupVersionResource]string{
			observationGVR: "ObservationList",
		})
		filterConfig := &filter.FilterConfig{Sources: make(map[string]filter.SourceFilter)}
		filter := filter.NewFilter(filterConfig)
		deduper := sdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000
		creator := watcher.NewObservationCreator(
			dynamicClient,
			observationGVR,
			nil,    // eventsTotal
			nil,    // observationsCreated
			nil,    // observationsFiltered
			nil,    // observationsDeduped
			nil,    // observationsCreateErrors
			filter, // filter
		)
		proc := NewProcessor(filter, deduper, creator)

		rawEvent := &generic.RawEvent{
			Source:  "fuzz-source",
			RawData: eventData,
		}

		sourceConfig := &generic.SourceConfig{
			Source: "fuzz-source",
			Normalization: &generic.NormalizationConfig{
				Domain: "security",
				Type:   "fuzz-event",
			},
		}

		ctx := context.Background()
		_ = proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	})
}

// FuzzProcessEvent_HighCardinalityLabels fuzzes with high-cardinality label sets
func FuzzProcessEvent_HighCardinalityLabels(f *testing.F) {
	f.Fuzz(func(t *testing.T, numLabels int) {
		// Limit to prevent excessive memory usage
		if numLabels > 1000 {
			t.Skip()
		}

		eventData := make(map[string]interface{})
		eventData["source"] = "fuzz-source"
		labels := make(map[string]interface{})
		for i := 0; i < numLabels; i++ {
			labels[fmt.Sprintf("label-%d", i)] = fmt.Sprintf("value-%d", i)
		}
		eventData["labels"] = labels

		// Setup processor
		observationGVR := schema.GroupVersionResource{
			Group:    "zen.kube-zen.io",
			Version:  "v1",
			Resource: "observations",
		}
		dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, map[schema.GroupVersionResource]string{
			observationGVR: "ObservationList",
		})
		filterConfig := &filter.FilterConfig{Sources: make(map[string]filter.SourceFilter)}
		filter := filter.NewFilter(filterConfig)
		deduper := sdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000
		creator := watcher.NewObservationCreator(
			dynamicClient,
			observationGVR,
			nil,    // eventsTotal
			nil,    // observationsCreated
			nil,    // observationsFiltered
			nil,    // observationsDeduped
			nil,    // observationsCreateErrors
			filter, // filter
		)
		proc := NewProcessor(filter, deduper, creator)

		rawEvent := &generic.RawEvent{
			Source:  "fuzz-source",
			RawData: eventData,
		}

		sourceConfig := &generic.SourceConfig{
			Source: "fuzz-source",
			Normalization: &generic.NormalizationConfig{
				Domain: "security",
				Type:   "fuzz-event",
			},
		}

		ctx := context.Background()
		_ = proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	})
}
