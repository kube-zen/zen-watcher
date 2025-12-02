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

package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	// ObservationFilterGVR is the GroupVersionResource for ObservationFilter CRDs
	ObservationFilterGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "observationfilters",
	}
)

// ObservationFilterLoader watches ObservationFilter CRDs and reloads filter configuration dynamically
type ObservationFilterLoader struct {
	dynClient       dynamic.Interface
	filter          *filter.Filter
	configMapLoader *ConfigMapLoader // Reference to ConfigMap loader for merging
	factory         dynamicinformer.DynamicSharedInformerFactory
	lastGoodConfig  *filter.FilterConfig
	mu              sync.RWMutex
	watchNamespace  string
}

// NewObservationFilterLoader creates a new ObservationFilter loader that watches for changes
func NewObservationFilterLoader(
	dynClient dynamic.Interface,
	filter *filter.Filter,
	configMapLoader *ConfigMapLoader,
) *ObservationFilterLoader {
	watchNamespace := ""
	if ns := strings.TrimSpace(observationFilterNamespace()); ns != "" {
		watchNamespace = ns
	}

	return &ObservationFilterLoader{
		dynClient:       dynClient,
		filter:          filter,
		configMapLoader: configMapLoader,
		watchNamespace:  watchNamespace,
	}
}

// observationFilterNamespace returns the namespace to watch for ObservationFilters
// Defaults to all namespaces (empty string) or can be set via OBSERVATION_FILTER_NAMESPACE env var
func observationFilterNamespace() string {
	if ns := strings.TrimSpace(os.Getenv("OBSERVATION_FILTER_NAMESPACE")); ns != "" {
		return ns
	}
	// Default to watching all namespaces for ObservationFilters
	// This allows namespace-scoped filters for multi-tenant scenarios
	return ""
}

// Start starts watching ObservationFilter CRDs for changes
func (ofl *ObservationFilterLoader) Start(ctx context.Context) error {
	logger.Info("Starting ObservationFilter CRD watcher for filter config",
		logger.Fields{
			Component: "config",
			Operation: "observationfilter_watcher_start",
			Additional: map[string]interface{}{
				"namespace": ofl.watchNamespace,
				"gvr":       ObservationFilterGVR.String(),
			},
		})

	// Create dynamic informer factory
	if ofl.watchNamespace != "" {
		// Watch specific namespace
		ofl.factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			ofl.dynClient,
			0, // resync period - 0 means no resync, only watch for changes
			ofl.watchNamespace,
			nil, // no label selector
		)
	} else {
		// Watch all namespaces
		ofl.factory = dynamicinformer.NewDynamicSharedInformerFactory(
			ofl.dynClient,
			0, // resync period
		)
	}

	// Get informer for ObservationFilter CRDs
	informer := ofl.factory.ForResource(ObservationFilterGVR).Informer()

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			of := obj.(*unstructured.Unstructured)
			ofl.handleObservationFilterChange(of)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			of := newObj.(*unstructured.Unstructured)
			ofl.handleObservationFilterChange(of)
		},
		DeleteFunc: func(obj interface{}) {
			of, ok := obj.(*unstructured.Unstructured)
			if !ok {
				// Handle deletedFinalStateUnknown
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				of, ok = tombstone.Obj.(*unstructured.Unstructured)
				if !ok {
					return
				}
			}
			ofl.handleObservationFilterChange(of)
		},
	})

	// Start informer
	ofl.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync ObservationFilter informer cache")
	}

	logger.Info("ObservationFilter watcher started and synced",
		logger.Fields{
			Component: "config",
			Operation: "observationfilter_watcher_synced",
		})

	// Load initial config from all ObservationFilters
	initialConfig, err := ofl.loadAllObservationFilters(ctx)
	if err != nil {
		logger.Warn("Failed to load initial ObservationFilter configs, will retry on CRD creation",
			logger.Fields{
				Component: "config",
				Operation: "observationfilter_load_initial",
				Error:     err,
			})
	} else {
		ofl.updateFilter(initialConfig)
		ofl.setLastGoodConfig(initialConfig)
	}

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// handleObservationFilterChange processes ObservationFilter CRD changes
func (ofl *ObservationFilterLoader) handleObservationFilterChange(of *unstructured.Unstructured) {
	// Reload all ObservationFilters and merge with ConfigMap
	ctx := context.Background()
	config, err := ofl.loadAllObservationFilters(ctx)
	if err != nil {
		logger.Error("Failed to reload ObservationFilter configs, keeping last good config",
			logger.Fields{
				Component:    "config",
				Operation:    "observationfilter_reload",
				Namespace:    of.GetNamespace(),
				ResourceName: of.GetName(),
				Error:        err,
			})
		return
	}

	ofl.updateFilter(config)
	ofl.setLastGoodConfig(config)
	logger.Info("Reloaded filter configuration from ObservationFilters",
		logger.Fields{
			Component: "config",
			Operation: "observationfilter_reload",
			Additional: map[string]interface{}{
				"filter_count": len(config.Sources),
			},
		})
}

// loadAllObservationFilters loads all ObservationFilter CRDs and converts them to FilterConfig
func (ofl *ObservationFilterLoader) loadAllObservationFilters(ctx context.Context) (*filter.FilterConfig, error) {
	var listOptions metav1.ListOptions

	var observationFilters *unstructured.UnstructuredList
	var err error

	if ofl.watchNamespace != "" {
		// List from specific namespace
		observationFilters, err = ofl.dynClient.Resource(ObservationFilterGVR).
			Namespace(ofl.watchNamespace).
			List(ctx, listOptions)
	} else {
		// List from all namespaces
		observationFilters, err = ofl.dynClient.Resource(ObservationFilterGVR).
			List(ctx, listOptions)
	}

	if err != nil {
		// CRD might not exist yet or might not have any resources
		return &filter.FilterConfig{
			Sources: make(map[string]filter.SourceFilter),
		}, nil // Return empty config, not error
	}

	// Convert ObservationFilter CRDs to FilterConfig
	config := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}

	for _, of := range observationFilters.Items {
		sourceFilter := ofl.convertObservationFilterToSourceFilter(&of)
		if sourceFilter == nil {
			continue // Skip invalid filters
		}

		// Get target source
		targetSource, found, _ := unstructured.NestedString(of.Object, "spec", "targetSource")
		if !found || targetSource == "" {
			logger.Warn("ObservationFilter missing targetSource, skipping",
				logger.Fields{
					Component:    "config",
					Operation:    "observationfilter_convert",
					Namespace:    of.GetNamespace(),
					ResourceName: of.GetName(),
				})
			continue
		}

		targetSource = strings.ToLower(targetSource)

		// If multiple ObservationFilters target the same source, merge them using FilterConfig merger
		if existing, exists := config.Sources[targetSource]; exists {
			// Create temporary configs for merging
			tempConfig1 := &filter.FilterConfig{
				Sources: map[string]filter.SourceFilter{
					targetSource: existing,
				},
			}
			tempConfig2 := &filter.FilterConfig{
				Sources: map[string]filter.SourceFilter{
					targetSource: *sourceFilter,
				},
			}
			mergedConfig := filter.MergeFilterConfigs(tempConfig1, tempConfig2)
			if merged, exists := mergedConfig.Sources[targetSource]; exists {
				config.Sources[targetSource] = merged
			}
		} else {
			config.Sources[targetSource] = *sourceFilter
		}
	}

	return config, nil
}

// convertObservationFilterToSourceFilter converts an ObservationFilter CRD to a SourceFilter
func (ofl *ObservationFilterLoader) convertObservationFilterToSourceFilter(of *unstructured.Unstructured) *filter.SourceFilter {
	spec, found, _ := unstructured.NestedMap(of.Object, "spec")
	if !found || spec == nil {
		return nil
	}

	result := &filter.SourceFilter{}

	// MinSeverity
	if val, found, _ := unstructured.NestedString(of.Object, "spec", "minSeverity"); found && val != "" {
		result.MinSeverity = strings.ToUpper(val)
	}

	// IncludeSeverity
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "includeSeverity"); found {
		upper := make([]string, len(val))
		for i, v := range val {
			upper[i] = strings.ToUpper(v)
		}
		result.IncludeSeverity = upper
	}

	// ExcludeEventTypes
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "excludeEventTypes"); found {
		result.ExcludeEventTypes = val
	}

	// IncludeEventTypes
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "includeEventTypes"); found {
		result.IncludeEventTypes = val
	}

	// ExcludeNamespaces
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "excludeNamespaces"); found {
		result.ExcludeNamespaces = val
	}

	// IncludeNamespaces
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "includeNamespaces"); found {
		result.IncludeNamespaces = val
	}

	// ExcludeKinds
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "excludeKinds"); found {
		result.ExcludeKinds = val
	}

	// IncludeKinds
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "includeKinds"); found {
		result.IncludeKinds = val
	}

	// ExcludeCategories
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "excludeCategories"); found {
		result.ExcludeCategories = val
	}

	// IncludeCategories
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "includeCategories"); found {
		result.IncludeCategories = val
	}

	// ExcludeRules
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "excludeRules"); found {
		result.ExcludeRules = val
	}

	// IgnoreKinds
	if val, found, _ := unstructured.NestedStringSlice(of.Object, "spec", "ignoreKinds"); found {
		result.IgnoreKinds = val
	}

	// Enabled
	if val, found, _ := unstructured.NestedBool(of.Object, "spec", "enabled"); found {
		result.Enabled = &val
	}

	return result
}

// updateFilter updates the merged filter (ConfigMap + ObservationFilters)
func (ofl *ObservationFilterLoader) updateFilter(observationFilterConfig *filter.FilterConfig) {
	if ofl.configMapLoader == nil || ofl.filter == nil {
		return
	}

	// Get ConfigMap config
	configMapConfig := ofl.configMapLoader.GetLastGoodConfig()
	if configMapConfig == nil {
		configMapConfig = &filter.FilterConfig{
			Sources: make(map[string]filter.SourceFilter),
		}
	}

	// Merge: ConfigMap first, then ObservationFilters on top
	merged := filter.MergeFilterConfigs(configMapConfig, observationFilterConfig)

	// Update the filter
	ofl.filter.UpdateConfig(merged)

	logger.Debug("Updated filter with merged config (ConfigMap + ObservationFilters)",
		logger.Fields{
			Component: "config",
			Operation: "filter_update_merged",
			Additional: map[string]interface{}{
				"configmap_sources": len(configMapConfig.Sources),
				"crd_sources":       len(observationFilterConfig.Sources),
				"merged_sources":    len(merged.Sources),
			},
		})
}

// setLastGoodConfig stores the last known good ObservationFilter configuration
func (ofl *ObservationFilterLoader) setLastGoodConfig(config *filter.FilterConfig) {
	ofl.mu.Lock()
	defer ofl.mu.Unlock()
	ofl.lastGoodConfig = config
}

// GetLastGoodConfig returns the last known good ObservationFilter configuration
func (ofl *ObservationFilterLoader) GetLastGoodConfig() *filter.FilterConfig {
	ofl.mu.RLock()
	defer ofl.mu.RUnlock()
	return ofl.lastGoodConfig
}
