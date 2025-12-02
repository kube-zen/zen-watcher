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
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ConfigMapLoader watches a ConfigMap and reloads filter configuration dynamically
type ConfigMapLoader struct {
	clientSet          kubernetes.Interface
	filter             *filter.Filter
	configMapName      string
	configMapNamespace string
	configMapKey       string
	lastGoodConfig     *filter.FilterConfig
	mu                 sync.RWMutex
}

// NewConfigMapLoader creates a new ConfigMap loader that watches for changes
func NewConfigMapLoader(
	clientSet kubernetes.Interface,
	filter *filter.Filter,
) *ConfigMapLoader {
	configMapName := os.Getenv("FILTER_CONFIGMAP_NAME")
	if configMapName == "" {
		configMapName = "zen-watcher-filter"
	}

	configMapNamespace := os.Getenv("FILTER_CONFIGMAP_NAMESPACE")
	if configMapNamespace == "" {
		configMapNamespace = os.Getenv("WATCH_NAMESPACE")
		if configMapNamespace == "" {
			configMapNamespace = "zen-system"
		}
	}

	configMapKey := os.Getenv("FILTER_CONFIGMAP_KEY")
	if configMapKey == "" {
		configMapKey = "filter.json"
	}

	return &ConfigMapLoader{
		clientSet:          clientSet,
		filter:             filter,
		configMapName:      configMapName,
		configMapNamespace: configMapNamespace,
		configMapKey:       configMapKey,
	}
}

// Start starts watching the ConfigMap for changes
func (cml *ConfigMapLoader) Start(ctx context.Context) error {
	logger.Info("Starting ConfigMap watcher for filter config",
		logger.Fields{
			Component: "config",
			Operation: "configmap_watcher_start",
			Namespace: cml.configMapNamespace,
			Additional: map[string]interface{}{
				"configmap_name": cml.configMapName,
			},
		})

	// Load initial config (use context to respect cancellation)
	initialConfig, err := cml.loadConfigWithContext(ctx)
	if err != nil {
		logger.Warn("Failed to load initial filter config, will retry on ConfigMap creation",
			logger.Fields{
				Component: "config",
				Operation: "configmap_load_initial",
				Namespace: cml.configMapNamespace,
				Error:     err,
			})
		// Continue - we'll watch for ConfigMap creation
	} else {
		cml.updateFilter(initialConfig)
		cml.setLastGoodConfig(initialConfig)
		logger.Info("Loaded initial filter configuration from ConfigMap",
			logger.Fields{
				Component: "config",
				Operation: "configmap_load_initial",
				Namespace: cml.configMapNamespace,
				Additional: map[string]interface{}{
					"configmap_name": cml.configMapName,
				},
			})
	}

	// Create informer factory for the specific namespace
	factory := informers.NewSharedInformerFactoryWithOptions(
		cml.clientSet,
		0, // resync period - 0 means no resync, only watch for changes
		informers.WithNamespace(cml.configMapNamespace),
	)

	// Get ConfigMap informer
	configMapInformer := factory.Core().V1().ConfigMaps().Informer()

	// Add event handlers
	configMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm, ok := obj.(*corev1.ConfigMap)
			if !ok || cm.Name != cml.configMapName {
				return
			}
			logger.Info("ConfigMap added",
				logger.Fields{
					Component: "config",
					Operation: "configmap_added",
					Namespace: cm.Namespace,
					Additional: map[string]interface{}{
						"configmap_name": cm.Name,
					},
				})
			cml.handleConfigMapChange(cm)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cm, ok := newObj.(*corev1.ConfigMap)
			if !ok || cm.Name != cml.configMapName {
				return
			}
			logger.Info("ConfigMap updated",
				logger.Fields{
					Component: "config",
					Operation: "configmap_updated",
					Namespace: cm.Namespace,
					Additional: map[string]interface{}{
						"configmap_name": cm.Name,
					},
				})
			cml.handleConfigMapChange(cm)
		},
		DeleteFunc: func(obj interface{}) {
			// Handle DeletedFinalStateUnknown (tombstone) from informer cache
			cm, ok := obj.(*corev1.ConfigMap)
			if !ok {
				// Try to extract from tombstone
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				cm, ok = tombstone.Obj.(*corev1.ConfigMap)
				if !ok {
					return
				}
			}
			if cm.Name != cml.configMapName {
				return
			}
			logger.Info("ConfigMap deleted, keeping last good config",
				logger.Fields{
					Component: "config",
					Operation: "configmap_deleted",
					Namespace: cm.Namespace,
					Additional: map[string]interface{}{
						"configmap_name": cm.Name,
					},
				})
			// Keep last good config - don't reset to default
		},
	})

	// Start the informer
	factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), configMapInformer.HasSynced) {
		return fmt.Errorf("failed to sync ConfigMap informer cache")
	}

	logger.Info("ConfigMap watcher started and synced",
		logger.Fields{
			Component: "config",
			Operation: "configmap_watcher_synced",
			Namespace: cml.configMapNamespace,
			Additional: map[string]interface{}{
				"configmap_name": cml.configMapName,
			},
		})

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// handleConfigMapChange processes ConfigMap changes
func (cml *ConfigMapLoader) handleConfigMapChange(cm *corev1.ConfigMap) {
	// Extract filter.json from ConfigMap
	filterJSON, found := cm.Data[cml.configMapKey]
	if !found {
		logger.Warn("Filter key not found in ConfigMap, keeping last good config",
			logger.Fields{
				Component: "config",
				Operation: "configmap_reload",
				Namespace: cm.Namespace,
				Reason:    "key_not_found",
				Additional: map[string]interface{}{
					"configmap_name": cm.Name,
					"key":            cml.configMapKey,
				},
			})
		return
	}

	// Parse JSON
	var config filter.FilterConfig
	if err := json.Unmarshal([]byte(filterJSON), &config); err != nil {
		logger.Error("Failed to parse filter config from ConfigMap, keeping last good config",
			logger.Fields{
				Component: "config",
				Operation: "configmap_reload",
				Namespace: cm.Namespace,
				Error:     err,
				Additional: map[string]interface{}{
					"configmap_name": cm.Name,
				},
			})
		return
	}

	// Validate config has sources map (prevent nil map panic)
	if config.Sources == nil {
		config.Sources = make(map[string]filter.SourceFilter)
	}

	// Store config
	cml.setLastGoodConfig(&config)
	
	// Update filter with new config
	// Note: If ObservationFilterLoader is active, it will merge ConfigMap + CRD configs
	// For backward compatibility without ObservationFilterLoader, we still update directly
	cml.updateFilter(&config)
	logger.Info("Reloaded filter configuration from ConfigMap",
		logger.Fields{
			Component: "config",
			Operation: "configmap_reload",
			Namespace: cm.Namespace,
			Additional: map[string]interface{}{
				"configmap_name": cm.Name,
			},
		})
}

// loadConfig loads the current ConfigMap configuration (uses context.Background for backward compatibility)
func (cml *ConfigMapLoader) loadConfig() (*filter.FilterConfig, error) {
	return cml.loadConfigWithContext(context.Background())
}

// loadConfigWithContext loads the current ConfigMap configuration with context support
func (cml *ConfigMapLoader) loadConfigWithContext(ctx context.Context) (*filter.FilterConfig, error) {
	cm, err := cml.clientSet.CoreV1().ConfigMaps(cml.configMapNamespace).Get(
		ctx,
		cml.configMapName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("ConfigMap not found: %w", err)
	}

	// Extract filter.json from ConfigMap
	filterJSON, found := cm.Data[cml.configMapKey]
	if !found {
		return nil, fmt.Errorf("key '%s' not found in ConfigMap", cml.configMapKey)
	}

	// Parse JSON
	var config filter.FilterConfig
	if err := json.Unmarshal([]byte(filterJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to parse filter config: %w", err)
	}

	// Validate config has sources map (prevent nil map panic)
	if config.Sources == nil {
		config.Sources = make(map[string]filter.SourceFilter)
	}

	return &config, nil
}

// updateFilter updates the filter with new configuration (thread-safe)
func (cml *ConfigMapLoader) updateFilter(config *filter.FilterConfig) {
	if cml.filter == nil {
		return
	}
	cml.filter.UpdateConfig(config)
}

// setLastGoodConfig stores the last known good configuration
func (cml *ConfigMapLoader) setLastGoodConfig(config *filter.FilterConfig) {
	cml.mu.Lock()
	defer cml.mu.Unlock()
	cml.lastGoodConfig = config
}

// GetLastGoodConfig returns the last known good configuration
func (cml *ConfigMapLoader) GetLastGoodConfig() *filter.FilterConfig {
	cml.mu.RLock()
	defer cml.mu.RUnlock()
	return cml.lastGoodConfig
}
