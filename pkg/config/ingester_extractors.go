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
	"fmt"
	"strconv"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// extractDestinations extracts and validates destination configurations
func extractDestinations(destinations []interface{}, logger *sdklog.Logger) []DestinationConfig {
	result := make([]DestinationConfig, 0, len(destinations))
	for _, dest := range destinations {
		if destMap, ok := dest.(map[string]interface{}); ok {
			destType := getString(destMap, "type")
			destValue := getString(destMap, "value")

			if destType == "crd" {
				gvr := resolveDestinationGVR(destMap, destValue, logger)
				if gvr.Resource != "" {
					result = append(result, DestinationConfig{
						Type:  destType,
						Value: destValue,
						GVR:   gvr,
					})
				}
			}
		}
	}
	return result
}

// resolveDestinationGVR resolves GVR from destination map
func resolveDestinationGVR(destMap map[string]interface{}, destValue string, logger *sdklog.Logger) schema.GroupVersionResource {
	var gvr schema.GroupVersionResource
	if gvrMap, ok := destMap["gvr"].(map[string]interface{}); ok {
		group := getString(gvrMap, "group")
		version := getString(gvrMap, "version")
		resource := getString(gvrMap, "resource")

		if version != "" && resource != "" {
			if err := ValidateGVRConfig(group, version, resource); err != nil {
				configLogger.Warn("Invalid GVR in destination configuration",
					sdklog.Operation("ingester_convert"),
					sdklog.String("group", group),
					sdklog.String("version", version),
					sdklog.String("resource", resource),
					sdklog.Error(err))
				return gvr
			}
			gvr = schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource,
			}
		} else if destValue != "" {
			gvr = ResolveDestinationGVR(destValue)
		} else {
			configLogger.Warn("Destination has neither gvr nor value",
				sdklog.Operation("ingester_convert"))
		}
	} else if destValue != "" {
		gvr = ResolveDestinationGVR(destValue)
	} else {
		logger.Warn("Destination has neither gvr nor value",
			sdklog.Operation("ingester_convert"))
	}
	return gvr
}

// extractNormalization extracts normalization configuration
func extractNormalization(spec map[string]interface{}) *NormalizationConfig {
	norm, ok := spec["normalization"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &NormalizationConfig{
		Domain:   getString(norm, "domain"),
		Type:     getString(norm, "type"),
		Priority: make(map[string]float64),
	}
	if priority, ok := norm["priority"].(map[string]interface{}); ok {
		for k, v := range priority {
			if f, ok := v.(float64); ok {
				config.Priority[k] = f
			}
		}
	}
	if fieldMapping, ok := norm["fieldMapping"].([]interface{}); ok {
		for _, fm := range fieldMapping {
			if fmMap, ok := fm.(map[string]interface{}); ok {
				config.FieldMapping = append(config.FieldMapping, FieldMapping{
					From:      getString(fmMap, "from"),
					To:        getString(fmMap, "to"),
					Transform: getString(fmMap, "transform"),
				})
			}
		}
	}
	return config
}

// extractProcessingConfig extracts processing configuration (filter and dedup)
func extractProcessingConfig(spec map[string]interface{}) (*ProcessingConfig, *FilterConfig, *DedupConfig) {
	var processingConfig *ProcessingConfig
	var filterConfig *FilterConfig
	var dedupConfig *DedupConfig

	if processing, ok := spec["processing"].(map[string]interface{}); ok {
		processingConfig = &ProcessingConfig{
			Order: getString(processing, "order"),
		}
		filterConfig = extractFilterFromProcessing(processing)
		dedupConfig = extractDedupFromProcessing(processing)
	}

	// Fallback to legacy locations
	if dedupConfig == nil {
		dedupConfig = extractDedupFromLegacy(spec)
	}
	if filterConfig == nil {
		filterConfig = extractFilterFromLegacy(spec)
	}

	return processingConfig, filterConfig, dedupConfig
}

// extractFilterFromProcessing extracts filter config from processing.filter
func extractFilterFromProcessing(processing map[string]interface{}) *FilterConfig {
	filter, ok := processing["filter"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &FilterConfig{}
	if expression, ok := filter["expression"].(string); ok && expression != "" {
		config.Expression = expression
	}
	if minPriority, ok := filter["minPriority"].(float64); ok {
		config.MinPriority = minPriority
	}
	if includeNS, ok := filter["includeNamespaces"].([]interface{}); ok {
		for _, ns := range includeNS {
			if nsStr, ok := ns.(string); ok {
				config.IncludeNamespaces = append(config.IncludeNamespaces, nsStr)
			}
		}
	}
	if excludeNS, ok := filter["excludeNamespaces"].([]interface{}); ok {
		for _, ns := range excludeNS {
			if nsStr, ok := ns.(string); ok {
				config.ExcludeNamespaces = append(config.ExcludeNamespaces, nsStr)
			}
		}
	}
	return config
}

// extractDedupFromProcessing extracts dedup config from processing.dedup
func extractDedupFromProcessing(processing map[string]interface{}) *DedupConfig {
	dedup, ok := processing["dedup"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &DedupConfig{
		Enabled: true,
	}
	if enabled, ok := dedup["enabled"].(bool); ok {
		config.Enabled = enabled
	}
	config.Window = getString(dedup, "window")
	config.Strategy = getString(dedup, "strategy")
	if config.Strategy == "" {
		config.Strategy = "fingerprint"
	}
	if fields, ok := dedup["fields"].([]interface{}); ok {
		for _, f := range fields {
			if fStr, ok := f.(string); ok {
				config.Fields = append(config.Fields, fStr)
			}
		}
	}
	if maxEvents, ok := dedup["maxEventsPerWindow"].(float64); ok {
		config.MaxEventsPerWindow = int(maxEvents)
	}
	return config
}

// extractDedupFromLegacy extracts dedup config from legacy location
func extractDedupFromLegacy(spec map[string]interface{}) *DedupConfig {
	dedup, ok := spec["deduplication"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &DedupConfig{
		Enabled: getBool(dedup, "enabled"),
	}
	config.Window = getString(dedup, "window")
	config.Strategy = getString(dedup, "strategy")
	if config.Strategy == "" {
		config.Strategy = "fingerprint"
	}
	if fields, ok := dedup["fields"].([]interface{}); ok {
		for _, f := range fields {
			if fStr, ok := f.(string); ok {
				config.Fields = append(config.Fields, fStr)
			}
		}
	}
	return config
}

// extractFilterFromLegacy extracts filter config from legacy location
func extractFilterFromLegacy(spec map[string]interface{}) *FilterConfig {
	filter, ok := spec["filters"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &FilterConfig{}
	if expression, ok := filter["expression"].(string); ok && expression != "" {
		config.Expression = expression
	}
	if minPriority, ok := filter["minPriority"].(float64); ok {
		config.MinPriority = minPriority
	}
	if includeNS, ok := filter["includeNamespaces"].([]interface{}); ok {
		for _, ns := range includeNS {
			if nsStr, ok := ns.(string); ok {
				config.IncludeNamespaces = append(config.IncludeNamespaces, nsStr)
			}
		}
	}
	if excludeNS, ok := filter["excludeNamespaces"].([]interface{}); ok {
		for _, ns := range excludeNS {
			if nsStr, ok := ns.(string); ok {
				config.ExcludeNamespaces = append(config.ExcludeNamespaces, nsStr)
			}
		}
	}
	return config
}

// extractOptimizationConfig extracts optimization configuration
func extractOptimizationConfig(spec map[string]interface{}) *OptimizationConfig {
	optimization, ok := spec["optimization"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &OptimizationConfig{
		Order:      getString(optimization, "order"),
		Processing: make(map[string]*ProcessingThreshold),
	}
	if thresholds, ok := optimization["thresholds"].(map[string]interface{}); ok {
		config.Thresholds = extractOptimizationThresholds(thresholds)
	}
	if processing, ok := optimization["processing"].(map[string]interface{}); ok {
		for key, val := range processing {
			if pMap, ok := val.(map[string]interface{}); ok {
				pt := &ProcessingThreshold{
					Action:      getString(pMap, "action"),
					Description: getString(pMap, "description"),
				}
				if w, ok := pMap["warning"].(float64); ok {
					pt.Warning = w
				}
				if c, ok := pMap["critical"].(float64); ok {
					pt.Critical = c
				}
				config.Processing[key] = pt
			}
		}
	}
	return config
}

// extractOptimizationThresholds extracts optimization thresholds
func extractOptimizationThresholds(thresholds map[string]interface{}) *OptimizationThresholds {
	result := &OptimizationThresholds{}
	if dedupEff, ok := thresholds["dedupEffectiveness"].(map[string]interface{}); ok {
		result.DedupEffectiveness = &ThresholdRange{}
		if w, ok := dedupEff["warning"].(float64); ok {
			result.DedupEffectiveness.Warning = w
		}
		if c, ok := dedupEff["critical"].(float64); ok {
			result.DedupEffectiveness.Critical = c
		}
	}
	if lowSev, ok := thresholds["lowSeverityPercent"].(map[string]interface{}); ok {
		result.LowSeverityPercent = &ThresholdRange{}
		if w, ok := lowSev["warning"].(float64); ok {
			result.LowSeverityPercent.Warning = w
		}
		if c, ok := lowSev["critical"].(float64); ok {
			result.LowSeverityPercent.Critical = c
		}
	}
	if obsPerMin, ok := thresholds["observationsPerMinute"].(map[string]interface{}); ok {
		result.ObservationsPerMinute = &ThresholdRange{}
		if w, ok := obsPerMin["warning"].(float64); ok {
			result.ObservationsPerMinute.Warning = w
		}
		if c, ok := obsPerMin["critical"].(float64); ok {
			result.ObservationsPerMinute.Critical = c
		}
	}
	if custom, ok := thresholds["custom"].([]interface{}); ok {
		for _, c := range custom {
			if cMap, ok := c.(map[string]interface{}); ok {
				ct := CustomThreshold{
					Name:     getString(cMap, "name"),
					Field:    getString(cMap, "field"),
					Operator: getString(cMap, "operator"),
					Value:    getString(cMap, "value"),
					Message:  getString(cMap, "message"),
				}
				result.Custom = append(result.Custom, ct)
			}
		}
	}
	return result
}

// Helper functions for extracting values from unstructured maps
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getSpecKeys returns all keys in the spec map for debugging
// nolint:unused // Kept for future use
func getSpecKeys(spec map[string]interface{}) []string {
	keys := make([]string, 0, len(spec))
	for k := range spec {
		keys = append(keys, k)
	}
	return keys
}

// extractSourceConfig extracts source-specific configuration for multi-source ingester
func extractSourceConfig(config *IngesterConfig, sourceMap map[string]interface{}, sourceType string, logger *sdklog.Logger, namespace, name, sourceName string) {
	switch sourceType {
	case "informer":
		config.Informer = extractInformerConfigFromMap(sourceMap)
	case "logs":
		config.Logs = extractLogsConfigFromMap(sourceMap)
	case "webhook":
		config.Webhook = extractWebhookConfigFromMap(sourceMap)
	default:
		logger.Warn("Unknown source type",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", namespace),
			sdklog.String("name", name),
			sdklog.String("sourceName", sourceName),
			sdklog.String("sourceType", sourceType))
	}
}

// extractInformerConfigFromMap extracts informer config from a map
func extractInformerConfigFromMap(sourceMap map[string]interface{}) *InformerConfig {
	informer, ok := sourceMap["informer"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &InformerConfig{}
	if gvr, ok := informer["gvr"].(map[string]interface{}); ok {
		config.GVR = GVRConfig{
			Group:    getString(gvr, "group"),
			Version:  getString(gvr, "version"),
			Resource: getString(gvr, "resource"),
		}
	}
	config.Namespace = getString(informer, "namespace")
	config.LabelSelector = getString(informer, "labelSelector")
	config.ResyncPeriod = getString(informer, "resyncPeriod")
	return config
}

// extractLogsConfigFromMap extracts logs config from a map
func extractLogsConfigFromMap(sourceMap map[string]interface{}) *LogsConfig {
	logs, ok := sourceMap["logs"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &LogsConfig{
		PodSelector:  getString(logs, "podSelector"),
		Container:    getString(logs, "container"),
		PollInterval: getString(logs, "pollInterval"),
	}
	if config.PollInterval == "" {
		config.PollInterval = "1s"
	}
	if sinceSeconds, ok := logs["sinceSeconds"].(int); ok {
		config.SinceSeconds = sinceSeconds
	} else if sinceSeconds, ok := logs["sinceSeconds"].(float64); ok {
		config.SinceSeconds = int(sinceSeconds)
	} else {
		config.SinceSeconds = DefaultLogsSinceSeconds
	}
	if patterns, ok := logs["patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if patternMap, ok := p.(map[string]interface{}); ok {
				pattern := LogPattern{
					Regex: getString(patternMap, "regex"),
					Type:  getString(patternMap, "type"),
				}
				if priority, ok := patternMap["priority"].(float64); ok {
					pattern.Priority = priority
				}
				config.Patterns = append(config.Patterns, pattern)
			}
		}
	}
	return config
}

// extractWebhookConfigFromMap extracts webhook config from a map
func extractWebhookConfigFromMap(sourceMap map[string]interface{}) *WebhookConfig {
	webhook, ok := sourceMap["webhook"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &WebhookConfig{
		Path: getString(webhook, "path"),
	}
	// Extract port (default to 8080 if not specified)
	if port, ok := webhook["port"].(int); ok {
		config.Port = port
	} else if portStr, ok := webhook["port"].(string); ok {
		// Handle string port (YAML might parse as string)
		if parsed, err := strconv.Atoi(portStr); err == nil {
			config.Port = parsed
		} else {
			config.Port = 8080 // Default
		}
	} else {
		config.Port = 8080 // Default
	}
	// Extract buffer size
	if bufferSize, ok := webhook["bufferSize"].(int); ok {
		config.BufferSize = bufferSize
	}
	if auth, ok := webhook["auth"].(map[string]interface{}); ok {
		config.Auth = &AuthConfig{
			Type:      getString(auth, "type"),
			SecretRef: getString(auth, "secretRef"),
		}
	}
	if rateLimit, ok := webhook["rateLimit"].(map[string]interface{}); ok {
		if rpm, ok := rateLimit["requestsPerMinute"].(int); ok {
			config.RateLimit = &RateLimitConfig{
				RequestsPerMinute: rpm,
			}
		}
	}
	return config
}

// extractInformerConfig extracts informer config from spec
func extractInformerConfig(spec map[string]interface{}) *InformerConfig {
	informer, ok := spec["informer"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &InformerConfig{}
	if gvr, ok := informer["gvr"].(map[string]interface{}); ok {
		config.GVR = GVRConfig{
			Group:    getString(gvr, "group"),
			Version:  getString(gvr, "version"),
			Resource: getString(gvr, "resource"),
		}
	}
	config.Namespace = getString(informer, "namespace")
	config.LabelSelector = getString(informer, "labelSelector")
	config.ResyncPeriod = getString(informer, "resyncPeriod")
	return config
}

// extractWebhookConfig extracts webhook config from spec
func extractWebhookConfig(spec map[string]interface{}) *WebhookConfig {
	webhook, ok := spec["webhook"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &WebhookConfig{
		Path: getString(webhook, "path"),
	}
	// Extract port (default to 8080 if not specified)
	if port, ok := webhook["port"].(int); ok {
		config.Port = port
	} else if portStr, ok := webhook["port"].(string); ok {
		// Handle string port (YAML might parse as string)
		if parsed, err := strconv.Atoi(portStr); err == nil {
			config.Port = parsed
		} else {
			config.Port = 8080 // Default
		}
	} else {
		config.Port = 8080 // Default
	}
	// Extract buffer size
	if bufferSize, ok := webhook["bufferSize"].(int); ok {
		config.BufferSize = bufferSize
	}
	if auth, ok := webhook["auth"].(map[string]interface{}); ok {
		config.Auth = &AuthConfig{
			Type:      getString(auth, "type"),
			SecretRef: getString(auth, "secretRef"),
		}
	}
	if rateLimit, ok := webhook["rateLimit"].(map[string]interface{}); ok {
		if rpm, ok := rateLimit["requestsPerMinute"].(int); ok {
			config.RateLimit = &RateLimitConfig{
				RequestsPerMinute: rpm,
			}
		}
	}
	return config
}

// extractLogsConfig extracts logs config from spec
func extractLogsConfig(spec map[string]interface{}, logger *sdklog.Logger, source string) *LogsConfig {
	logs, logsOk := spec["logs"]
	if logsOk {
		logger.Info("Found logs section in spec",
			sdklog.Operation("ingester_convert"),
			sdklog.String("source", source),
			sdklog.String("logs_type", fmt.Sprintf("%T", logs)))
	}
	return extractLogsConfigFromMap(spec)
}
