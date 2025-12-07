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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	// ObservationSourceConfigGVR is the GroupVersionResource for ObservationSourceConfig CRDs
	ObservationSourceConfigGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "observationsourceconfigs",
	}
)

// SourceConfig represents the configuration for a source
type SourceConfig struct {
	Source      string
	DedupWindow time.Duration
	DedupStrategy string
	DedupFields []string
	FilterMinPriority float64
	FilterExcludeNamespaces []string
	FilterIncludeTypes []string
	TTLDefault time.Duration
	TTLMin     time.Duration
	TTLMax     time.Duration
	RateLimitMaxPerMinute int
	RateLimitBurst int
	TypeDetection string
	TypeField     string
	StaticType    string
	ProcessingOrder string // auto, filter_first, dedup_first
	AutoOptimize bool
}

// SourceConfigLoader watches ObservationSourceConfig CRDs and caches configuration
type SourceConfigLoader struct {
	dynClient      dynamic.Interface
	factory        dynamicinformer.DynamicSharedInformerFactory
	configCache    map[string]*SourceConfig // source -> config
	mu             sync.RWMutex
	watchNamespace string
}

// NewSourceConfigLoader creates a new SourceConfig loader that watches for changes
func NewSourceConfigLoader(dynClient dynamic.Interface) *SourceConfigLoader {
	watchNamespace := ""
	if ns := strings.TrimSpace(os.Getenv("OBSERVATION_SOURCE_CONFIG_NAMESPACE")); ns != "" {
		watchNamespace = ns
	}

	return &SourceConfigLoader{
		dynClient:      dynClient,
		configCache:    make(map[string]*SourceConfig),
		watchNamespace: watchNamespace,
	}
}

// Start starts watching ObservationSourceConfig CRDs for changes
func (scl *SourceConfigLoader) Start(ctx context.Context) error {
	logger.Info("Starting ObservationSourceConfig CRD watcher",
		logger.Fields{
			Component: "config",
			Operation: "observationsourceconfig_watcher_start",
			Additional: map[string]interface{}{
				"namespace": scl.watchNamespace,
				"gvr":       ObservationSourceConfigGVR.String(),
			},
		})

	// Create dynamic informer factory
	if scl.watchNamespace != "" {
		scl.factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			scl.dynClient,
			0,
			scl.watchNamespace,
			nil,
		)
	} else {
		scl.factory = dynamicinformer.NewDynamicSharedInformerFactory(
			scl.dynClient,
			0,
		)
	}

	// Get informer for ObservationSourceConfig CRDs
	informer := scl.factory.ForResource(ObservationSourceConfigGVR).Informer()

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			osc := obj.(*unstructured.Unstructured)
			scl.handleConfigChange(osc)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			osc := newObj.(*unstructured.Unstructured)
			scl.handleConfigChange(osc)
		},
		DeleteFunc: func(obj interface{}) {
			osc, ok := obj.(*unstructured.Unstructured)
			if !ok {
				if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					osc, ok = tombstone.Obj.(*unstructured.Unstructured)
					if !ok {
						return
					}
				} else {
					return
				}
			}
			scl.handleConfigChange(osc)
		},
	})

	// Start informer
	scl.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync ObservationSourceConfig informer cache")
	}

	logger.Info("ObservationSourceConfig watcher started and synced",
		logger.Fields{
			Component: "config",
			Operation: "observationsourceconfig_watcher_synced",
		})

	// Load initial configs
	scl.reloadAllConfigs(ctx)

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// handleConfigChange processes ObservationSourceConfig CRD changes
func (scl *SourceConfigLoader) handleConfigChange(osc *unstructured.Unstructured) {
	ctx := context.Background()
	scl.reloadAllConfigs(ctx)
	logger.Debug("Reloaded source config from ObservationSourceConfig",
		logger.Fields{
			Component:    "config",
			Operation:    "observationsourceconfig_reload",
			Namespace:    osc.GetNamespace(),
			ResourceName: osc.GetName(),
		})
}

// reloadAllConfigs reloads all ObservationSourceConfig CRDs and updates cache
func (scl *SourceConfigLoader) reloadAllConfigs(ctx context.Context) {
	var listOptions metav1.ListOptions

	var configs *unstructured.UnstructuredList
	var err error

	if scl.watchNamespace != "" {
		configs, err = scl.dynClient.Resource(ObservationSourceConfigGVR).
			Namespace(scl.watchNamespace).
			List(ctx, listOptions)
	} else {
		configs, err = scl.dynClient.Resource(ObservationSourceConfigGVR).
			List(ctx, listOptions)
	}

	if err != nil {
		logger.Debug("Failed to list ObservationSourceConfigs, using defaults",
			logger.Fields{
				Component: "config",
				Operation: "observationsourceconfig_list",
				Error:     err,
			})
		return
	}

	scl.mu.Lock()
	defer scl.mu.Unlock()

	// Clear cache and rebuild
	newCache := make(map[string]*SourceConfig)

	for _, osc := range configs.Items {
		sourceConfig := scl.convertToSourceConfig(&osc)
		if sourceConfig == nil {
			continue
		}

		// If multiple configs for same source, merge (use most restrictive)
		if existing, exists := newCache[sourceConfig.Source]; exists {
			newCache[sourceConfig.Source] = scl.mergeSourceConfigs(existing, sourceConfig)
		} else {
			newCache[sourceConfig.Source] = sourceConfig
		}
	}

	scl.configCache = newCache
}

// convertToSourceConfig converts an ObservationSourceConfig CRD to SourceConfig
func (scl *SourceConfigLoader) convertToSourceConfig(osc *unstructured.Unstructured) *SourceConfig {
	source, found, _ := unstructured.NestedString(osc.Object, "spec", "source")
	if !found || source == "" {
		return nil
	}

	config := &SourceConfig{
		Source: strings.ToLower(source),
	}

	// Get defaults for this source
	defaults := GetDefaultSourceConfig(config.Source)

	// Parse dedup configuration
	if dedupObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "dedup"); found {
		if windowStr, ok := dedupObj["window"].(string); ok {
			if window, err := time.ParseDuration(windowStr); err == nil {
				config.DedupWindow = window
			} else {
				config.DedupWindow = defaults.DedupWindow
			}
		} else {
			config.DedupWindow = defaults.DedupWindow
		}

		if strategy, ok := dedupObj["strategy"].(string); ok {
			config.DedupStrategy = strategy
		} else {
			config.DedupStrategy = "fingerprint"
		}

		if fields, ok := dedupObj["fields"].([]interface{}); ok {
			config.DedupFields = make([]string, 0, len(fields))
			for _, f := range fields {
				if fieldStr, ok := f.(string); ok {
					config.DedupFields = append(config.DedupFields, fieldStr)
				}
			}
		}
	} else {
		config.DedupWindow = defaults.DedupWindow
		config.DedupStrategy = "fingerprint"
	}

	// Parse filter configuration
	if filterObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "filter"); found {
		if minPriority, ok := filterObj["minPriority"].(float64); ok {
			config.FilterMinPriority = minPriority
		} else {
			config.FilterMinPriority = defaults.FilterMinPriority
		}

		if excludeNamespaces, ok := filterObj["excludeNamespaces"].([]interface{}); ok {
			config.FilterExcludeNamespaces = make([]string, 0, len(excludeNamespaces))
			for _, ns := range excludeNamespaces {
				if nsStr, ok := ns.(string); ok {
					config.FilterExcludeNamespaces = append(config.FilterExcludeNamespaces, nsStr)
				}
			}
		}

		if includeTypes, ok := filterObj["includeTypes"].([]interface{}); ok {
			config.FilterIncludeTypes = make([]string, 0, len(includeTypes))
			for _, t := range includeTypes {
				if tStr, ok := t.(string); ok {
					config.FilterIncludeTypes = append(config.FilterIncludeTypes, tStr)
				}
			}
		}
	} else {
		config.FilterMinPriority = defaults.FilterMinPriority
	}

	// Parse TTL configuration
	if ttlObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "ttl"); found {
		if defaultTTL, ok := ttlObj["default"].(string); ok {
			if ttl, err := time.ParseDuration(defaultTTL); err == nil {
				config.TTLDefault = ttl
			} else {
				config.TTLDefault = defaults.TTLDefault
			}
		} else {
			config.TTLDefault = defaults.TTLDefault
		}

		if minTTL, ok := ttlObj["min"].(string); ok {
			time.ParseDuration(minTTL) // Store if needed
		}

		if maxTTL, ok := ttlObj["max"].(string); ok {
			time.ParseDuration(maxTTL) // Store if needed
		}
	} else {
		config.TTLDefault = defaults.TTLDefault
	}

	// Parse rate limit configuration
	if rateLimitObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "rateLimit"); found {
		if maxPerMin, ok := rateLimitObj["maxPerMinute"].(int64); ok {
			config.RateLimitMaxPerMinute = int(maxPerMin)
		} else {
			config.RateLimitMaxPerMinute = defaults.RateLimitMax
		}

		if burst, ok := rateLimitObj["burst"].(int64); ok {
			config.RateLimitBurst = int(burst)
		} else {
			config.RateLimitBurst = defaults.RateLimitMax * 2
		}
	} else {
		config.RateLimitMaxPerMinute = defaults.RateLimitMax
		config.RateLimitBurst = defaults.RateLimitMax * 2
	}

	// Parse processing configuration
	if processingObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "processing"); found {
		if order, ok := processingObj["order"].(string); ok {
			config.ProcessingOrder = order
		} else {
			config.ProcessingOrder = "auto"
		}

		if autoOptimize, ok := processingObj["autoOptimize"].(bool); ok {
			config.AutoOptimize = autoOptimize
		} else {
			config.AutoOptimize = true // Default to true
		}
	} else {
		config.ProcessingOrder = "auto"
		config.AutoOptimize = true
	}

	return config
}

// mergeSourceConfigs merges two source configs (uses most restrictive values)
func (scl *SourceConfigLoader) mergeSourceConfigs(c1, c2 *SourceConfig) *SourceConfig {
	result := *c1

	// Use smaller dedup window (more restrictive)
	if c2.DedupWindow < result.DedupWindow {
		result.DedupWindow = c2.DedupWindow
	}

	// Use higher min priority (more restrictive)
	if c2.FilterMinPriority > result.FilterMinPriority {
		result.FilterMinPriority = c2.FilterMinPriority
	}

	// Merge exclude namespaces (union)
	excludeMap := make(map[string]bool)
	for _, ns := range result.FilterExcludeNamespaces {
		excludeMap[ns] = true
	}
	for _, ns := range c2.FilterExcludeNamespaces {
		excludeMap[ns] = true
	}
	result.FilterExcludeNamespaces = make([]string, 0, len(excludeMap))
	for ns := range excludeMap {
		result.FilterExcludeNamespaces = append(result.FilterExcludeNamespaces, ns)
	}

	// Merge include types (intersection - more restrictive)
	if len(c2.FilterIncludeTypes) > 0 {
		if len(result.FilterIncludeTypes) == 0 {
			result.FilterIncludeTypes = c2.FilterIncludeTypes
		} else {
			// Intersection logic
			typeMap := make(map[string]bool)
			for _, t := range c2.FilterIncludeTypes {
				typeMap[t] = true
			}
			intersection := make([]string, 0)
			for _, t := range result.FilterIncludeTypes {
				if typeMap[t] {
					intersection = append(intersection, t)
				}
			}
			result.FilterIncludeTypes = intersection
		}
	}

	return &result
}

// GetSourceConfig returns the configuration for a source (with defaults fallback)
func (scl *SourceConfigLoader) GetSourceConfig(source string) *SourceConfig {
	scl.mu.RLock()
	defer scl.mu.RUnlock()

	if config, exists := scl.configCache[strings.ToLower(source)]; exists {
		return config
	}

	// Return defaults if no config exists
	defaults := GetDefaultSourceConfig(source)
	return &SourceConfig{
		Source:            strings.ToLower(source),
		DedupWindow:       defaults.DedupWindow,
		DedupStrategy:     "fingerprint",
		FilterMinPriority: defaults.FilterMinPriority,
		TTLDefault:        defaults.TTLDefault,
		RateLimitMaxPerMinute: defaults.RateLimitMax,
		RateLimitBurst:    defaults.RateLimitMax * 2,
		ProcessingOrder:   "auto",
		AutoOptimize:      true,
	}
}

// GetAllSourceConfigs returns all cached source configurations
func (scl *SourceConfigLoader) GetAllSourceConfigs() map[string]*SourceConfig {
	scl.mu.RLock()
	defer scl.mu.RUnlock()

	result := make(map[string]*SourceConfig)
	for k, v := range scl.configCache {
		result[k] = v
	}
	return result
}

