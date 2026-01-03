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

package filter

import (
	"fmt"
	"sync"

	sdkfilter "github.com/kube-zen/zen-sdk/pkg/filter"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Type aliases for compatibility
type FilterConfig = sdkfilter.FilterConfig
type SourceFilter = sdkfilter.SourceFilter
type GlobalNamespaceFilter = sdkfilter.GlobalNamespaceFilter

// Package-level logger to avoid repeated allocations
var (
	filterLogger = sdklog.NewLogger("zen-watcher-filter")
)

// Filter wraps zen-sdk Filter with zen-watcher metrics support
// This wrapper maintains backward compatibility while using zen-sdk internally
type Filter struct {
	mu        sync.RWMutex
	config    *FilterConfig
	metrics   *metrics.Metrics  // Optional metrics
	sdkFilter *sdkfilter.Filter // Internal zen-sdk filter
}

// NewFilter creates a new filter with the given configuration
func NewFilter(config *FilterConfig) *Filter {
	return NewFilterWithMetrics(config, nil)
}

// NewFilterWithMetrics creates a new filter with metrics support
func NewFilterWithMetrics(config *FilterConfig, m *metrics.Metrics) *Filter {
	var metricsAdapter sdkfilter.FilterMetrics
	if m != nil {
		metricsAdapter = NewMetricsAdapter(m)
	}
	return &Filter{
		config:    config,
		metrics:   m,
		sdkFilter: sdkfilter.NewFilterWithMetrics(config, metricsAdapter),
	}
}

// UpdateConfig updates the filter configuration atomically (thread-safe)
func (f *Filter) UpdateConfig(config *FilterConfig) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// Validate config - ensure Sources map is initialized
	if config != nil && config.Sources == nil {
		config.Sources = make(map[string]SourceFilter)
	}
	f.config = config
	// Update internal SDK filter
	if f.sdkFilter != nil {
		var metricsAdapter sdkfilter.FilterMetrics
		if f.metrics != nil {
			metricsAdapter = NewMetricsAdapter(f.metrics)
		}
		f.sdkFilter = sdkfilter.NewFilterWithMetrics(config, metricsAdapter)
	}
	filterLogger.Debug("Filter configuration updated dynamically",
		sdklog.Operation("config_update"))
}

// GetConfig returns a copy of the current filter configuration (thread-safe read)
// This is exported for use by external components that need to merge configs
func (f *Filter) GetConfig() *FilterConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.config == nil {
		return &FilterConfig{
			Sources: make(map[string]SourceFilter),
		}
	}
	// Return a shallow copy (caller should deep copy if modifying)
	return f.config
}

// Allow checks if an observation should be allowed based on filter rules
// Returns true if the observation should be processed, false if it should be filtered out
// This is called BEFORE normalization and deduplication
// Delegates to zen-sdk filter
func (f *Filter) Allow(observation *unstructured.Unstructured) bool {
	if f == nil {
		return true
	}
	f.mu.RLock()
	sdkFilter := f.sdkFilter
	f.mu.RUnlock()
	if sdkFilter == nil {
		return true
	}
	return sdkFilter.Allow(observation)
}

// AllowWithReason checks if an observation should be allowed and returns the reason if filtered
// Returns (true, "") if allowed, (false, reason) if filtered
// Delegates to zen-sdk filter but maintains custom metrics tracking
func (f *Filter) AllowWithReason(observation *unstructured.Unstructured) (bool, string) {
	if f == nil {
		return true, ""
	}
	f.mu.RLock()
	sdkFilter := f.sdkFilter
	metrics := f.metrics
	f.mu.RUnlock()
	if sdkFilter == nil {
		return true, ""
	}
	// Use SDK filter for actual filtering logic
	// Note: SDK filter doesn't have AllowWithReason, so we use Allow and track metrics separately
	allowed := sdkFilter.Allow(observation)
	if !allowed {
		// Extract source for metrics
		sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
		source := "unknown"
		if sourceVal != nil {
			if str, ok := sourceVal.(string); ok {
				source = str
			} else {
				source = fmt.Sprintf("%v", sourceVal)
			}
		}
		if metrics != nil {
			metrics.FilterDecisions.WithLabelValues(source, "filter", "sdk_filtered").Inc()
		}
		return false, "sdk_filtered"
	}
	return true, ""
}
