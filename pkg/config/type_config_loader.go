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

	"github.com/kube-zen/zen-watcher/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	// ObservationTypeConfigGVR is the GroupVersionResource for ObservationTypeConfig CRDs
	ObservationTypeConfigGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "observationtypeconfigs",
	}
)

// FieldMappingTypeConfig represents a field mapping from raw data to labels
// (renamed to avoid conflict with FieldMapping in ingester_loader.go)
type FieldMappingTypeConfig struct {
	From      string // JSONPath in raw data
	To        string // Label name
	Transform string // Optional transform function
}

// ResourceExtractionConfig represents how to extract Kubernetes resources
type ResourceExtractionConfig struct {
	Strategy string              // jsonpath, k8s_owner, or manual
	JSONPath string              // JSONPath expression
	K8SOwner map[string]string   // Field paths for K8s owner
	Manual   []map[string]string // Manually specified resources
}

// TypeConfig represents the configuration for an observation type
type TypeConfig struct {
	Type               string
	Domain             string
	PriorityMap        map[string]float64 // Source value -> priority (0.0-1.0)
	FieldMappings      []FieldMappingTypeConfig
	Templates          map[string]string // title, description go templates
	ResourceExtraction ResourceExtractionConfig
}

// TypeConfigLoader watches ObservationTypeConfig CRDs and caches configuration
type TypeConfigLoader struct {
	dynClient      dynamic.Interface
	factory        dynamicinformer.DynamicSharedInformerFactory
	configCache    map[string]*TypeConfig // type -> config
	mu             sync.RWMutex
	watchNamespace string
}

// NewTypeConfigLoader creates a new TypeConfig loader that watches for changes
func NewTypeConfigLoader(dynClient dynamic.Interface) *TypeConfigLoader {
	watchNamespace := ""
	if ns := strings.TrimSpace(os.Getenv("OBSERVATION_TYPE_CONFIG_NAMESPACE")); ns != "" {
		watchNamespace = ns
	}

	return &TypeConfigLoader{
		dynClient:      dynClient,
		configCache:    make(map[string]*TypeConfig),
		watchNamespace: watchNamespace,
	}
}

// Start starts watching ObservationTypeConfig CRDs for changes
func (tcl *TypeConfigLoader) Start(ctx context.Context) error {
	logger.Info("Starting ObservationTypeConfig CRD watcher",
		logger.Fields{
			Component: "config",
			Operation: "observationtypeconfig_watcher_start",
			Additional: map[string]interface{}{
				"namespace": tcl.watchNamespace,
				"gvr":       ObservationTypeConfigGVR.String(),
			},
		})

	// Create dynamic informer factory
	if tcl.watchNamespace != "" {
		tcl.factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			tcl.dynClient,
			0,
			tcl.watchNamespace,
			nil,
		)
	} else {
		tcl.factory = dynamicinformer.NewDynamicSharedInformerFactory(
			tcl.dynClient,
			0,
		)
	}

	// Get informer for ObservationTypeConfig CRDs
	informer := tcl.factory.ForResource(ObservationTypeConfigGVR).Informer()

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			otc := obj.(*unstructured.Unstructured)
			tcl.handleConfigChange(otc)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			otc := newObj.(*unstructured.Unstructured)
			tcl.handleConfigChange(otc)
		},
		DeleteFunc: func(obj interface{}) {
			otc, ok := obj.(*unstructured.Unstructured)
			if !ok {
				if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					otc, ok = tombstone.Obj.(*unstructured.Unstructured)
					if !ok {
						return
					}
				} else {
					return
				}
			}
			tcl.handleConfigChange(otc)
		},
	})

	// Start informer
	tcl.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync ObservationTypeConfig informer cache")
	}

	logger.Info("ObservationTypeConfig watcher started and synced",
		logger.Fields{
			Component: "config",
			Operation: "observationtypeconfig_watcher_synced",
		})

	// Load initial configs
	tcl.reloadAllConfigs(ctx)

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// handleConfigChange processes ObservationTypeConfig CRD changes
func (tcl *TypeConfigLoader) handleConfigChange(otc *unstructured.Unstructured) {
	ctx := context.Background()
	tcl.reloadAllConfigs(ctx)
	logger.Debug("Reloaded type config from ObservationTypeConfig",
		logger.Fields{
			Component:    "config",
			Operation:    "observationtypeconfig_reload",
			Namespace:    otc.GetNamespace(),
			ResourceName: otc.GetName(),
		})
}

// reloadAllConfigs reloads all ObservationTypeConfig CRDs and updates cache
func (tcl *TypeConfigLoader) reloadAllConfigs(ctx context.Context) {
	var listOptions metav1.ListOptions

	var configs *unstructured.UnstructuredList
	var err error

	if tcl.watchNamespace != "" {
		configs, err = tcl.dynClient.Resource(ObservationTypeConfigGVR).
			Namespace(tcl.watchNamespace).
			List(ctx, listOptions)
	} else {
		configs, err = tcl.dynClient.Resource(ObservationTypeConfigGVR).
			List(ctx, listOptions)
	}

	if err != nil {
		logger.Debug("Failed to list ObservationTypeConfigs, using defaults",
			logger.Fields{
				Component: "config",
				Operation: "observationtypeconfig_list",
				Error:     err,
			})
		return
	}

	tcl.mu.Lock()
	defer tcl.mu.Unlock()

	// Clear cache and rebuild
	newCache := make(map[string]*TypeConfig)

	for _, otc := range configs.Items {
		typeConfig := tcl.convertToTypeConfig(&otc)
		if typeConfig == nil {
			continue
		}

		// If multiple configs for same type, last one wins (could implement merging if needed)
		newCache[typeConfig.Type] = typeConfig
	}

	tcl.configCache = newCache
}

// convertToTypeConfig converts an ObservationTypeConfig CRD to TypeConfig
func (tcl *TypeConfigLoader) convertToTypeConfig(otc *unstructured.Unstructured) *TypeConfig {
	obsType, found, _ := unstructured.NestedString(otc.Object, "spec", "type")
	if !found || obsType == "" {
		return nil
	}

	domain, found, _ := unstructured.NestedString(otc.Object, "spec", "domain")
	if !found {
		domain = "security" // Default
	}

	config := &TypeConfig{
		Type:        strings.ToLower(obsType),
		Domain:      domain,
		PriorityMap: make(map[string]float64),
		Templates:   make(map[string]string),
	}

	// Get defaults for this type
	defaults := GetDefaultTypeConfig(config.Type)
	if config.Domain == "security" && defaults.Domain != "" {
		config.Domain = defaults.Domain
	}

	// Parse priority mapping
	if priorityMap, found, _ := unstructured.NestedMap(otc.Object, "spec", "priority"); found {
		for k, v := range priorityMap {
			if priorityVal, ok := v.(float64); ok && priorityVal >= 0.0 && priorityVal <= 1.0 {
				config.PriorityMap[k] = priorityVal
			}
		}
	}

	// Parse field mappings
	if fieldMappings, found, _ := unstructured.NestedSlice(otc.Object, "spec", "fieldMapping"); found {
		config.FieldMappings = make([]FieldMappingTypeConfig, 0, len(fieldMappings))
		for _, fm := range fieldMappings {
			if fmMap, ok := fm.(map[string]interface{}); ok {
				mapping := FieldMappingTypeConfig{}
				if from, ok := fmMap["from"].(string); ok {
					mapping.From = from
				}
				if to, ok := fmMap["to"].(string); ok {
					mapping.To = to
				}
				if transform, ok := fmMap["transform"].(string); ok {
					mapping.Transform = transform
				}
				if mapping.From != "" && mapping.To != "" {
					config.FieldMappings = append(config.FieldMappings, mapping)
				}
			}
		}
	}

	// Parse templates
	if templates, found, _ := unstructured.NestedMap(otc.Object, "spec", "templates"); found {
		if title, ok := templates["title"].(string); ok {
			config.Templates["title"] = title
		}
		if description, ok := templates["description"].(string); ok {
			config.Templates["description"] = description
		}
	}

	// Parse resource extraction
	if resourceExtraction, found, _ := unstructured.NestedMap(otc.Object, "spec", "resourceExtraction"); found {
		config.ResourceExtraction = ResourceExtractionConfig{}

		if strategy, ok := resourceExtraction["strategy"].(string); ok {
			config.ResourceExtraction.Strategy = strategy
		}

		if jsonpath, ok := resourceExtraction["jsonpath"].(string); ok {
			config.ResourceExtraction.JSONPath = jsonpath
		}

		if k8sOwner, ok := resourceExtraction["k8sOwner"].(map[string]interface{}); ok {
			config.ResourceExtraction.K8SOwner = make(map[string]string)
			if apiVersion, ok := k8sOwner["apiVersionField"].(string); ok {
				config.ResourceExtraction.K8SOwner["apiVersion"] = apiVersion
			}
			if kind, ok := k8sOwner["kindField"].(string); ok {
				config.ResourceExtraction.K8SOwner["kind"] = kind
			}
			if name, ok := k8sOwner["nameField"].(string); ok {
				config.ResourceExtraction.K8SOwner["name"] = name
			}
			if ns, ok := k8sOwner["namespaceField"].(string); ok {
				config.ResourceExtraction.K8SOwner["namespace"] = ns
			}
		}

		if manual, ok := resourceExtraction["manual"].([]interface{}); ok {
			config.ResourceExtraction.Manual = make([]map[string]string, 0, len(manual))
			for _, m := range manual {
				if mMap, ok := m.(map[string]interface{}); ok {
					resource := make(map[string]string)
					if apiVersion, ok := mMap["apiVersion"].(string); ok {
						resource["apiVersion"] = apiVersion
					}
					if kind, ok := mMap["kind"].(string); ok {
						resource["kind"] = kind
					}
					if name, ok := mMap["name"].(string); ok {
						resource["name"] = name
					}
					if ns, ok := mMap["namespace"].(string); ok {
						resource["namespace"] = ns
					}
					if len(resource) > 0 {
						config.ResourceExtraction.Manual = append(config.ResourceExtraction.Manual, resource)
					}
				}
			}
		}
	}

	return config
}

// GetTypeConfig returns the configuration for an observation type (with defaults fallback)
func (tcl *TypeConfigLoader) GetTypeConfig(obsType string) *TypeConfig {
	tcl.mu.RLock()
	defer tcl.mu.RUnlock()

	if config, exists := tcl.configCache[strings.ToLower(obsType)]; exists {
		return config
	}

	// Return defaults if no config exists
	defaults := GetDefaultTypeConfig(obsType)
	return &TypeConfig{
		Type:               strings.ToLower(obsType),
		Domain:             defaults.Domain,
		PriorityMap:        make(map[string]float64),
		FieldMappings:      []FieldMappingTypeConfig{},
		Templates:          make(map[string]string),
		ResourceExtraction: ResourceExtractionConfig{},
	}
}

// GetAllTypeConfigs returns all cached type configurations
func (tcl *TypeConfigLoader) GetAllTypeConfigs() map[string]*TypeConfig {
	tcl.mu.RLock()
	defer tcl.mu.RUnlock()

	result := make(map[string]*TypeConfig)
	for k, v := range tcl.configCache {
		result[k] = v
	}
	return result
}
