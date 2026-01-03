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

package processor

import (
	"context"
	"testing"
	"time"

	sdkdedup "github.com/kube-zen/zen-sdk/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

// setupBenchmarkProcessor creates a processor for benchmarking
func setupBenchmarkProcessor() (*Processor, dynamic.Interface) {
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

	// Create observation creator with nil metrics for benchmarking
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
	return proc, dynamicClient
}

// BenchmarkPipeline_HighVolumeLowSeverity benchmarks high-volume events with 85% LOW severity
func BenchmarkPipeline_HighVolumeLowSeverity(b *testing.B) {
	proc, _ := setupBenchmarkProcessor()
	ctx := context.Background()

	sourceConfig := &generic.SourceConfig{
		Source: "benchmark-source",
		Processing: &generic.ProcessingConfig{
			Order: "filter_first", // Expected strategy for high LOW severity
		},
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "vulnerability",
			Priority: map[string]float64{
				"LOW":  0.3,
				"HIGH": 0.8,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 85% LOW severity, 15% HIGH
		severity := "LOW"
		if i%20 == 0 {
			severity = "HIGH"
		}

		rawEvent := &generic.RawEvent{
			Source:    "benchmark-source",
			Timestamp: time.Now(),
			RawData: map[string]interface{}{
				"severity": severity,
				"id":       i,
			},
		}

		_ = proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	}
}

// BenchmarkPipeline_HighDeduplicationRate benchmarks events with 60% duplication rate
func BenchmarkPipeline_HighDeduplicationRate(b *testing.B) {
	proc, _ := setupBenchmarkProcessor()
	ctx := context.Background()

	sourceConfig := &generic.SourceConfig{
		Source: "benchmark-source",
		Processing: &generic.ProcessingConfig{
			Order: "dedup_first", // Expected strategy for high dedup rate
		},
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "vulnerability",
			Priority: map[string]float64{
				"HIGH": 0.8,
			},
		},
	}

	// Create base event that will be duplicated
	baseEvent := &generic.RawEvent{
		Source:    "benchmark-source",
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": "HIGH",
			"id":       "duplicate-id",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 60% duplicates (same content), 40% unique
		event := baseEvent
		if i%10 < 4 {
			// Unique event
			event = &generic.RawEvent{
				Source:    "benchmark-source",
				Timestamp: time.Now(),
				RawData: map[string]interface{}{
					"severity": "HIGH",
					"id":       i,
				},
			}
		}

		_ = proc.ProcessEvent(ctx, event, sourceConfig)
	}
}

// BenchmarkPipeline_BalancedSeverity benchmarks balanced severity distribution
func BenchmarkPipeline_BalancedSeverity(b *testing.B) {
	proc, _ := setupBenchmarkProcessor()
	ctx := context.Background()

	sourceConfig := &generic.SourceConfig{
		Source: "benchmark-source",
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "vulnerability",
			Priority: map[string]float64{
				"LOW":    0.3,
				"MEDIUM": 0.5,
				"HIGH":   0.8,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 40% LOW, 30% MEDIUM, 30% HIGH
		severity := "LOW"
		switch i % 10 {
		case 0, 1, 2, 3:
			severity = "LOW"
		case 4, 5, 6:
			severity = "MEDIUM"
		case 7, 8, 9:
			severity = "HIGH"
		}

		rawEvent := &generic.RawEvent{
			Source:    "benchmark-source",
			Timestamp: time.Now(),
			RawData: map[string]interface{}{
				"severity": severity,
				"id":       i,
			},
		}

		_ = proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	}
}

// BenchmarkPipeline_FilterFirst benchmarks filter_first strategy
func BenchmarkPipeline_FilterFirst(b *testing.B) {
	proc, _ := setupBenchmarkProcessor()
	ctx := context.Background()

	sourceConfig := &generic.SourceConfig{
		Source: "benchmark-source",
		Processing: &generic.ProcessingConfig{
			Order: "filter_first",
		},
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "vulnerability",
			Priority: map[string]float64{
				"HIGH": 0.8,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rawEvent := &generic.RawEvent{
			Source:    "benchmark-source",
			Timestamp: time.Now(),
			RawData: map[string]interface{}{
				"severity": "HIGH",
				"id":       i,
			},
		}

		_ = proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	}
}

// BenchmarkPipeline_DedupFirst benchmarks dedup_first strategy
func BenchmarkPipeline_DedupFirst(b *testing.B) {
	proc, _ := setupBenchmarkProcessor()
	ctx := context.Background()

	sourceConfig := &generic.SourceConfig{
		Source: "benchmark-source",
		Processing: &generic.ProcessingConfig{
			Order: "dedup_first",
		},
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "vulnerability",
			Priority: map[string]float64{
				"HIGH": 0.8,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rawEvent := &generic.RawEvent{
			Source:    "benchmark-source",
			Timestamp: time.Now(),
			RawData: map[string]interface{}{
				"severity": "HIGH",
				"id":       i,
			},
		}

		_ = proc.ProcessEvent(ctx, rawEvent, sourceConfig)
	}
}
