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

package helpers

import (
	"testing"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	sdkdedup "github.com/kube-zen/zen-sdk/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// DefaultObservationGVR is the default GVR for observations in tests
var DefaultObservationGVR = schema.GroupVersionResource{
	Group:    "zen.kube-zen.io",
	Version:  "v1",
	Resource: "observations",
}

// SetupTestEnv creates a fake dynamic client for testing
func SetupTestEnv(t *testing.T) dynamic.Interface {
	t.Helper()
	scheme := runtime.NewScheme()
	// Register observations resource for List operations
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		DefaultObservationGVR: "ObservationList",
	})
}

// SetupTestEnvWithGVR creates a fake dynamic client with custom GVR
func SetupTestEnvWithGVR(t *testing.T, gvr schema.GroupVersionResource) dynamic.Interface {
	t.Helper()
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: gvr.Resource + "List",
	})
}

// SetupPipeline creates a complete pipeline with fake clients
func SetupPipeline(t *testing.T, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource) *processor.Processor {
	t.Helper()

	// Create filter
	filterConfig := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}
	f := filter.NewFilter(filterConfig)

	// Create deduper
	deduper := sdkdedup.NewDeduper(60, 10000) // windowSeconds=60, maxSize=10000

	// Create observation creator
	creator := watcher.NewObservationCreator(
		dynamicClient,
		gvr,
		nil, nil, nil, nil, nil, // metrics
		f, // filter
	)

	// Create processor
	return processor.NewProcessor(f, deduper, creator)
}

// SetupPipelineWithDefaults creates a pipeline with default observation GVR
func SetupPipelineWithDefaults(t *testing.T, dynamicClient dynamic.Interface) *processor.Processor {
	t.Helper()
	return SetupPipeline(t, dynamicClient, DefaultObservationGVR)
}

// CreateTestSourceConfig creates a test SourceConfig
func CreateTestSourceConfig(source string) *generic.SourceConfig {
	return &generic.SourceConfig{
		Source:   source,
		Ingester: "informer",
		Normalization: &generic.NormalizationConfig{
			Domain: "security",
			Type:   "test-event",
			Priority: map[string]float64{
				"HIGH":   0.8,
				"MEDIUM": 0.5,
				"LOW":    0.2,
			},
		},
	}
}

// CreateTestSourceConfigWithProcessing creates a test SourceConfig with processing order
func CreateTestSourceConfigWithProcessing(source, order string) *generic.SourceConfig {
	config := CreateTestSourceConfig(source)
	config.Processing = &generic.ProcessingConfig{
		Order: order, // "filter_first" or "dedup_first"
	}
	return config
}

