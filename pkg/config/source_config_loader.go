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

// ThresholdConfig represents a threshold configuration for monitoring
type ThresholdConfig struct {
	Warning    float64
	Critical   float64
	Window     time.Duration
	Action     string // warn, alert, optimize
	Description string
}

// ProcessingConfig represents processing optimization configuration
type ProcessingConfig struct {
	Order            string                      // auto, filter_first, dedup_first
	AutoOptimize     bool                        // Enable auto-optimization
	Thresholds       map[string]*ThresholdConfig // Per-metric thresholds
	AnalysisInterval time.Duration               // How often to analyze performance
	ConfidenceThreshold float64                  // Minimum confidence for auto-optimization (0.0-1.0)
}

// FilterConfig represents advanced filtering configuration
type FilterConfig struct {
	MinPriority        float64
	ExcludeNamespaces  []string
	IncludeTypes       []string
	DynamicRules       []DynamicFilterRule
	AdaptiveEnabled    bool
	LearningRate       float64
}

// DynamicFilterRule represents a dynamic filter rule
type DynamicFilterRule struct {
	ID        string
	Priority  int
	Enabled   bool
	Condition string // JSONPath condition
	Action    string // include, exclude, modify
	TTL       time.Duration
	Metrics   map[string]float64
}

// DeduplicationConfig represents intelligent deduplication configuration
type DeduplicationConfig struct {
	Window      time.Duration
	Strategy    string   // fingerprint, content, hybrid, adaptive
	Adaptive    bool     // Enable adaptive deduplication
	Fields      []string // Fields to consider
	MinChange   float64  // Minimum change threshold to trigger adaptation
	LearningRate float64
}

// RateLimitConfig represents rate limiting and throttling configuration
type RateLimitConfig struct {
	MaxPerMinute   int
	Burst          int
	Adaptive       bool           // Enable adaptive rate limiting
	CooldownPeriod time.Duration
	Targets        map[string]int // Per-severity or per-type targets
}

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
	
	// Enhanced optimization configuration (backward compatible - flat fields remain)
	Processing    ProcessingConfig
	Filter        FilterConfig
	Deduplication DeduplicationConfig
	RateLimit     RateLimitConfig
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
				config.Deduplication.Window = window
			} else {
				config.DedupWindow = defaults.DedupWindow
				config.Deduplication.Window = defaults.DedupWindow
			}
		} else {
			config.DedupWindow = defaults.DedupWindow
			config.Deduplication.Window = defaults.DedupWindow
		}

		if strategy, ok := dedupObj["strategy"].(string); ok {
			config.DedupStrategy = strategy
			config.Deduplication.Strategy = strategy
		} else {
			config.DedupStrategy = "fingerprint"
			config.Deduplication.Strategy = "fingerprint"
		}

		if fields, ok := dedupObj["fields"].([]interface{}); ok {
			config.DedupFields = make([]string, 0, len(fields))
			config.Deduplication.Fields = make([]string, 0, len(fields))
			for _, f := range fields {
				if fieldStr, ok := f.(string); ok {
					config.DedupFields = append(config.DedupFields, fieldStr)
					config.Deduplication.Fields = append(config.Deduplication.Fields, fieldStr)
				}
			}
		}

		// Parse adaptive deduplication settings
		if adaptive, ok := dedupObj["adaptive"].(bool); ok {
			config.Deduplication.Adaptive = adaptive
		}
		if minChange, ok := dedupObj["minChange"].(float64); ok {
			config.Deduplication.MinChange = minChange
		}
		if learningRate, ok := dedupObj["learningRate"].(float64); ok {
			config.Deduplication.LearningRate = learningRate
		}
	} else {
		config.DedupWindow = defaults.DedupWindow
		config.DedupStrategy = "fingerprint"
		config.Deduplication.Window = defaults.DedupWindow
		config.Deduplication.Strategy = "fingerprint"
	}

	// Parse filter configuration
	if filterObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "filter"); found {
		if minPriority, ok := filterObj["minPriority"].(float64); ok {
			config.FilterMinPriority = minPriority
			config.Filter.MinPriority = minPriority
		} else {
			config.FilterMinPriority = defaults.FilterMinPriority
			config.Filter.MinPriority = defaults.FilterMinPriority
		}

		if excludeNamespaces, ok := filterObj["excludeNamespaces"].([]interface{}); ok {
			config.FilterExcludeNamespaces = make([]string, 0, len(excludeNamespaces))
			config.Filter.ExcludeNamespaces = make([]string, 0, len(excludeNamespaces))
			for _, ns := range excludeNamespaces {
				if nsStr, ok := ns.(string); ok {
					config.FilterExcludeNamespaces = append(config.FilterExcludeNamespaces, nsStr)
					config.Filter.ExcludeNamespaces = append(config.Filter.ExcludeNamespaces, nsStr)
				}
			}
		}

		if includeTypes, ok := filterObj["includeTypes"].([]interface{}); ok {
			config.FilterIncludeTypes = make([]string, 0, len(includeTypes))
			config.Filter.IncludeTypes = make([]string, 0, len(includeTypes))
			for _, t := range includeTypes {
				if tStr, ok := t.(string); ok {
					config.FilterIncludeTypes = append(config.FilterIncludeTypes, tStr)
					config.Filter.IncludeTypes = append(config.Filter.IncludeTypes, tStr)
				}
			}
		}

		// Parse adaptive filtering settings
		if adaptiveEnabled, ok := filterObj["adaptiveEnabled"].(bool); ok {
			config.Filter.AdaptiveEnabled = adaptiveEnabled
		}
		if learningRate, ok := filterObj["learningRate"].(float64); ok {
			config.Filter.LearningRate = learningRate
		}

		// Parse dynamic rules
		if dynamicRules, ok := filterObj["dynamicRules"].([]interface{}); ok {
			config.Filter.DynamicRules = make([]DynamicFilterRule, 0, len(dynamicRules))
			for _, ruleObj := range dynamicRules {
				if ruleMap, ok := ruleObj.(map[string]interface{}); ok {
					rule := DynamicFilterRule{
						Metrics: make(map[string]float64),
					}
					if id, ok := ruleMap["id"].(string); ok {
						rule.ID = id
					}
					if priority, ok := ruleMap["priority"].(int64); ok {
						rule.Priority = int(priority)
					}
					if enabled, ok := ruleMap["enabled"].(bool); ok {
						rule.Enabled = enabled
					}
					if condition, ok := ruleMap["condition"].(string); ok {
						rule.Condition = condition
					}
					if action, ok := ruleMap["action"].(string); ok {
						rule.Action = action
					}
					if ttlStr, ok := ruleMap["ttl"].(string); ok {
						if ttl, err := time.ParseDuration(ttlStr); err == nil {
							rule.TTL = ttl
						}
					}
					if metricsObj, ok := ruleMap["metrics"].(map[string]interface{}); ok {
						for k, v := range metricsObj {
							if val, ok := v.(float64); ok {
								rule.Metrics[k] = val
							}
						}
					}
					config.Filter.DynamicRules = append(config.Filter.DynamicRules, rule)
				}
			}
		}
	} else {
		config.FilterMinPriority = defaults.FilterMinPriority
		config.Filter.MinPriority = defaults.FilterMinPriority
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
			config.RateLimit.MaxPerMinute = int(maxPerMin)
		} else {
			config.RateLimitMaxPerMinute = defaults.RateLimitMax
			config.RateLimit.MaxPerMinute = defaults.RateLimitMax
		}

		if burst, ok := rateLimitObj["burst"].(int64); ok {
			config.RateLimitBurst = int(burst)
			config.RateLimit.Burst = int(burst)
		} else {
			config.RateLimitBurst = defaults.RateLimitMax * 2
			config.RateLimit.Burst = defaults.RateLimitMax * 2
		}

		// Parse adaptive rate limiting settings
		if adaptive, ok := rateLimitObj["adaptive"].(bool); ok {
			config.RateLimit.Adaptive = adaptive
		}
		if cooldownStr, ok := rateLimitObj["cooldownPeriod"].(string); ok {
			if cooldown, err := time.ParseDuration(cooldownStr); err == nil {
				config.RateLimit.CooldownPeriod = cooldown
			}
		}
		if targetsObj, ok := rateLimitObj["targets"].(map[string]interface{}); ok {
			config.RateLimit.Targets = make(map[string]int)
			for k, v := range targetsObj {
				if val, ok := v.(int64); ok {
					config.RateLimit.Targets[k] = int(val)
				}
			}
		}
	} else {
		config.RateLimitMaxPerMinute = defaults.RateLimitMax
		config.RateLimitBurst = defaults.RateLimitMax * 2
		config.RateLimit.MaxPerMinute = defaults.RateLimitMax
		config.RateLimit.Burst = defaults.RateLimitMax * 2
	}

	// Parse processing configuration (enhanced with nested structure)
	if processingObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "processing"); found {
		if order, ok := processingObj["order"].(string); ok {
			config.ProcessingOrder = order
			config.Processing.Order = order
		} else {
			config.ProcessingOrder = "auto"
			config.Processing.Order = "auto"
		}

		if autoOptimize, ok := processingObj["autoOptimize"].(bool); ok {
			config.AutoOptimize = autoOptimize
			config.Processing.AutoOptimize = autoOptimize
		} else {
			config.AutoOptimize = true // Default to true
			config.Processing.AutoOptimize = true
		}

		// Parse analysis interval
		if intervalStr, ok := processingObj["analysisInterval"].(string); ok {
			if interval, err := time.ParseDuration(intervalStr); err == nil {
				config.Processing.AnalysisInterval = interval
			} else {
				config.Processing.AnalysisInterval = 15 * time.Minute // Default
			}
		} else {
			config.Processing.AnalysisInterval = 15 * time.Minute
		}

		// Parse confidence threshold
		if confidence, ok := processingObj["confidenceThreshold"].(float64); ok {
			config.Processing.ConfidenceThreshold = confidence
		} else {
			config.Processing.ConfidenceThreshold = 0.7 // Default 70%
		}

		// Parse thresholds (if present)
		if thresholdsObj, ok := processingObj["thresholds"].(map[string]interface{}); ok {
			config.Processing.Thresholds = make(map[string]*ThresholdConfig)
			// Thresholds will be parsed in detail below
		}
	} else {
		config.ProcessingOrder = "auto"
		config.AutoOptimize = true
		config.Processing.Order = "auto"
		config.Processing.AutoOptimize = true
		config.Processing.AnalysisInterval = 15 * time.Minute
		config.Processing.ConfidenceThreshold = 0.7
	}

	// Parse thresholds configuration from top-level thresholds field
	if thresholdsObj, found, _ := unstructured.NestedMap(osc.Object, "spec", "thresholds"); found {
		if config.Processing.Thresholds == nil {
			config.Processing.Thresholds = make(map[string]*ThresholdConfig)
		}

		// Parse observationsPerMinute threshold
		if obsPerMinObj, ok := thresholdsObj["observationsPerMinute"].(map[string]interface{}); ok {
			thresh := &ThresholdConfig{
				Action: "alert",
				Description: "Observation rate per minute",
			}
			if warning, ok := obsPerMinObj["warning"].(int64); ok {
				thresh.Warning = float64(warning)
			}
			if critical, ok := obsPerMinObj["critical"].(int64); ok {
				thresh.Critical = float64(critical)
			}
			if action, ok := obsPerMinObj["action"].(string); ok {
				thresh.Action = action
			}
			config.Processing.Thresholds["observationsPerMinute"] = thresh
		}

		// Parse lowSeverityPercent threshold
		if lowSevObj, ok := thresholdsObj["lowSeverityPercent"].(map[string]interface{}); ok {
			thresh := &ThresholdConfig{
				Action: "filter",
				Description: "Low severity event percentage",
			}
			if warning, ok := lowSevObj["warning"].(float64); ok {
				thresh.Warning = warning
			}
			if critical, ok := lowSevObj["critical"].(float64); ok {
				thresh.Critical = critical
			}
			if action, ok := lowSevObj["action"].(string); ok {
				thresh.Action = action
			}
			config.Processing.Thresholds["lowSeverityPercent"] = thresh
		}

		// Parse dedupEffectiveness threshold
		if dedupEffObj, ok := thresholdsObj["dedupEffectiveness"].(map[string]interface{}); ok {
			thresh := &ThresholdConfig{
				Action: "optimize",
				Description: "Deduplication effectiveness",
			}
			if warning, ok := dedupEffObj["warning"].(float64); ok {
				thresh.Warning = warning
			}
			if critical, ok := dedupEffObj["critical"].(float64); ok {
				thresh.Critical = critical
			}
			if action, ok := dedupEffObj["action"].(string); ok {
				thresh.Action = action
			}
			config.Processing.Thresholds["dedupEffectiveness"] = thresh
		}
	}

	// Populate enhanced Filter config from existing filter data
	config.Filter.MinPriority = config.FilterMinPriority
	config.Filter.ExcludeNamespaces = config.FilterExcludeNamespaces
	config.Filter.IncludeTypes = config.FilterIncludeTypes
	config.Filter.AdaptiveEnabled = false // Default, can be enabled via CRD later
	config.Filter.LearningRate = 0.1      // Default learning rate

	// Populate enhanced Deduplication config from existing dedup data
	config.Deduplication.Window = config.DedupWindow
	config.Deduplication.Strategy = config.DedupStrategy
	config.Deduplication.Fields = config.DedupFields
	config.Deduplication.Adaptive = false // Default, can be enabled via CRD later
	config.Deduplication.MinChange = 0.05 // Default 5% minimum change
	config.Deduplication.LearningRate = 0.1

	// Populate enhanced RateLimit config from existing rate limit data
	config.RateLimit.MaxPerMinute = config.RateLimitMaxPerMinute
	config.RateLimit.Burst = config.RateLimitBurst
	config.RateLimit.Adaptive = false           // Default, can be enabled via CRD later
	config.RateLimit.CooldownPeriod = 5 * time.Minute // Default cooldown
	config.RateLimit.Targets = make(map[string]int)

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

