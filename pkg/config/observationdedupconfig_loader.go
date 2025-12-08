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

package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	// ObservationDedupConfigGVR is the GroupVersionResource for ObservationDedupConfig CRDs
	ObservationDedupConfigGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "observationdedupconfigs",
	}
)

// ObservationDedupConfigLoader watches ObservationDedupConfig CRDs and reloads dedup configuration dynamically
type ObservationDedupConfigLoader struct {
	dynClient      dynamic.Interface
	deduper        *dedup.Deduper
	factory        dynamicinformer.DynamicSharedInformerFactory
	lastGoodConfig map[string]int // source -> windowSeconds
	mu             sync.RWMutex
	watchNamespace string
	defaultWindow  int
}

// NewObservationDedupConfigLoader creates a new ObservationDedupConfig loader that watches for changes
func NewObservationDedupConfigLoader(
	dynClient dynamic.Interface,
	deduper *dedup.Deduper,
	defaultWindow int,
) *ObservationDedupConfigLoader {
	watchNamespace := ""
	if ns := strings.TrimSpace(observationDedupConfigNamespace()); ns != "" {
		watchNamespace = ns
	}

	return &ObservationDedupConfigLoader{
		dynClient:      dynClient,
		deduper:        deduper,
		watchNamespace: watchNamespace,
		defaultWindow:  defaultWindow,
		lastGoodConfig: make(map[string]int),
	}
}

// observationDedupConfigNamespace returns the namespace to watch for ObservationDedupConfigs
// Defaults to all namespaces (empty string) or can be set via OBSERVATION_DEDUP_CONFIG_NAMESPACE env var
func observationDedupConfigNamespace() string {
	if ns := strings.TrimSpace(os.Getenv("OBSERVATION_DEDUP_CONFIG_NAMESPACE")); ns != "" {
		return ns
	}
	// Default to watching all namespaces for ObservationDedupConfigs
	// This allows namespace-scoped dedup configs for multi-tenant scenarios
	return ""
}

// Start starts watching ObservationDedupConfig CRDs for changes
func (odcl *ObservationDedupConfigLoader) Start(ctx context.Context) error {
	logger.Info("Starting ObservationDedupConfig CRD watcher for dedup config",
		logger.Fields{
			Component: "config",
			Operation: "observationdedupconfig_watcher_start",
			Additional: map[string]interface{}{
				"namespace":    odcl.watchNamespace,
				"gvr":          ObservationDedupConfigGVR.String(),
				"default_window": odcl.defaultWindow,
			},
		})

	// Create dynamic informer factory
	if odcl.watchNamespace != "" {
		// Watch specific namespace
		odcl.factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			odcl.dynClient,
			0, // resync period - 0 means no resync, only watch for changes
			odcl.watchNamespace,
			nil, // no label selector
		)
	} else {
		// Watch all namespaces
		odcl.factory = dynamicinformer.NewDynamicSharedInformerFactory(
			odcl.dynClient,
			0, // resync period
		)
	}

	// Get informer for ObservationDedupConfig CRDs
	informer := odcl.factory.ForResource(ObservationDedupConfigGVR).Informer()

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			odc := obj.(*unstructured.Unstructured)
			odcl.handleObservationDedupConfigChange(odc)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			odc := newObj.(*unstructured.Unstructured)
			odcl.handleObservationDedupConfigChange(odc)
		},
		DeleteFunc: func(obj interface{}) {
			odc, ok := obj.(*unstructured.Unstructured)
			if !ok {
				// Handle deletedFinalStateUnknown
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				odc, ok = tombstone.Obj.(*unstructured.Unstructured)
				if !ok {
					return
				}
			}
			odcl.handleObservationDedupConfigChange(odc)
		},
	})

	// Start informer
	odcl.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync ObservationDedupConfig informer cache")
	}

	logger.Info("ObservationDedupConfig watcher started and synced",
		logger.Fields{
			Component: "config",
			Operation: "observationdedupconfig_watcher_synced",
		})

	// Load initial config from all ObservationDedupConfigs
	initialConfig, err := odcl.loadAllObservationDedupConfigs(ctx)
	if err != nil {
		logger.Warn("Failed to load initial ObservationDedupConfig configs, will retry on CRD creation",
			logger.Fields{
				Component: "config",
				Operation: "observationdedupconfig_load_initial",
				Error:     err,
			})
	} else {
		odcl.updateDeduper(initialConfig)
		odcl.setLastGoodConfig(initialConfig)
	}

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// handleObservationDedupConfigChange processes ObservationDedupConfig CRD changes
func (odcl *ObservationDedupConfigLoader) handleObservationDedupConfigChange(odc *unstructured.Unstructured) {
	// Reload all ObservationDedupConfigs
	ctx := context.Background()
	config, err := odcl.loadAllObservationDedupConfigs(ctx)
	if err != nil {
		logger.Error("Failed to reload ObservationDedupConfig configs, keeping last good config",
			logger.Fields{
				Component:    "config",
				Operation:    "observationdedupconfig_reload",
				Namespace:    odc.GetNamespace(),
				ResourceName: odc.GetName(),
				Error:        err,
			})
		return
	}

	odcl.updateDeduper(config)
	odcl.setLastGoodConfig(config)
	logger.Info("Reloaded dedup configuration from ObservationDedupConfigs",
		logger.Fields{
			Component: "config",
			Operation: "observationdedupconfig_reload",
			Additional: map[string]interface{}{
				"config_count": len(config),
			},
		})
}

// loadAllObservationDedupConfigs loads all ObservationDedupConfig CRDs and converts them to source->window map
func (odcl *ObservationDedupConfigLoader) loadAllObservationDedupConfigs(ctx context.Context) (map[string]int, error) {
	var listOptions metav1.ListOptions

	var observationDedupConfigs *unstructured.UnstructuredList
	var err error

	if odcl.watchNamespace != "" {
		// List from specific namespace
		observationDedupConfigs, err = odcl.dynClient.Resource(ObservationDedupConfigGVR).
			Namespace(odcl.watchNamespace).
			List(ctx, listOptions)
	} else {
		// List from all namespaces
		observationDedupConfigs, err = odcl.dynClient.Resource(ObservationDedupConfigGVR).
			List(ctx, listOptions)
	}

	if err != nil {
		// CRD might not exist yet or might not have any resources
		return make(map[string]int), nil // Return empty config, not error
	}

	// Convert ObservationDedupConfig CRDs to source->window map
	config := make(map[string]int)

	for _, odc := range observationDedupConfigs.Items {
		targetSource, found, _ := unstructured.NestedString(odc.Object, "spec", "targetSource")
		if !found || targetSource == "" {
			logger.Warn("ObservationDedupConfig missing targetSource, skipping",
				logger.Fields{
					Component:    "config",
					Operation:    "observationdedupconfig_convert",
					Namespace:    odc.GetNamespace(),
					ResourceName: odc.GetName(),
				})
			continue
		}

		targetSource = strings.ToLower(targetSource)

		// Check if enabled (default: true)
		enabled := true
		if val, found, _ := unstructured.NestedBool(odc.Object, "spec", "enabled"); found {
			enabled = val
		}

		if !enabled {
			logger.Debug("ObservationDedupConfig disabled, skipping",
				logger.Fields{
					Component:    "config",
					Operation:    "observationdedupconfig_convert",
					Namespace:    odc.GetNamespace(),
					ResourceName: odc.GetName(),
					Source:       targetSource,
				})
			continue
		}

		// Get windowSeconds
		windowSeconds := odcl.defaultWindow // Default to configured default
		if val, found, _ := unstructured.NestedInt64(odc.Object, "spec", "windowSeconds"); found && val > 0 {
			windowSeconds = int(val)
		}

		// If multiple ObservationDedupConfigs target the same source, use the one with smaller window (more restrictive)
		// This is a design choice - you could also use the last one or raise an error
		if existing, exists := config[targetSource]; exists {
			if windowSeconds < existing {
				logger.Debug("Multiple ObservationDedupConfigs for same source, using smaller window",
					logger.Fields{
						Component:    "config",
						Operation:    "observationdedupconfig_merge",
						Source:       targetSource,
						Additional: map[string]interface{}{
							"existing_window": existing,
							"new_window":      windowSeconds,
							"selected":        windowSeconds,
						},
					})
				config[targetSource] = windowSeconds
			} else {
				logger.Debug("Multiple ObservationDedupConfigs for same source, keeping existing window",
					logger.Fields{
						Component:    "config",
						Operation:    "observationdedupconfig_merge",
						Source:       targetSource,
						Additional: map[string]interface{}{
							"existing_window": existing,
							"new_window":      windowSeconds,
							"selected":        existing,
						},
					})
			}
		} else {
			config[targetSource] = windowSeconds
		}
	}

	return config, nil
}

// updateDeduper updates the deduper with new source window configuration
func (odcl *ObservationDedupConfigLoader) updateDeduper(config map[string]int) {
	if odcl.deduper == nil {
		return
	}

	// Update the deduper with new configuration
	odcl.deduper.UpdateSourceWindows(config, odcl.defaultWindow)

	logger.Debug("Updated deduper with new source window configuration",
		logger.Fields{
			Component: "config",
			Operation: "deduper_update",
			Additional: map[string]interface{}{
				"source_count": len(config),
				"default_window": odcl.defaultWindow,
			},
		})
}

// setLastGoodConfig stores the last known good ObservationDedupConfig configuration
func (odcl *ObservationDedupConfigLoader) setLastGoodConfig(config map[string]int) {
	odcl.mu.Lock()
	defer odcl.mu.Unlock()
	odcl.lastGoodConfig = make(map[string]int)
	for k, v := range config {
		odcl.lastGoodConfig[k] = v
	}
}

// GetLastGoodConfig returns the last known good ObservationDedupConfig configuration
func (odcl *ObservationDedupConfigLoader) GetLastGoodConfig() map[string]int {
	odcl.mu.RLock()
	defer odcl.mu.RUnlock()
	result := make(map[string]int)
	for k, v := range odcl.lastGoodConfig {
		result[k] = v
	}
	return result
}

