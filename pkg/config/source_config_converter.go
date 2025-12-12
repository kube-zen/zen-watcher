// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may Obtain a copy of the License at
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

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ConvertToGenericSourceConfig converts an ObservationSourceConfig CRD to generic.SourceConfig
func ConvertToGenericSourceConfig(u *unstructured.Unstructured) (*generic.SourceConfig, error) {
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("spec not found or invalid")
	}

	source, _ := spec["source"].(string)
	ingester, _ := spec["ingester"].(string)

	config := &generic.SourceConfig{
		Source:   source,
		Ingester: ingester,
	}

	// Parse adapter-specific configs
	switch ingester {
	case "informer":
		config.Informer = parseInformerConfig(spec)
	case "webhook":
		config.Webhook = parseWebhookConfig(spec)
	case "logs":
		config.Logs = parseLogsConfig(spec)
	case "cm", "configmap": // ConfigMap adapter is not supported, use Informer adapter instead
		// ConfigMap adapter is not supported - return error
		return nil, fmt.Errorf("ConfigMap adapter is not supported. Use Informer adapter with GVR { group: \"\", version: \"v1\", resource: \"configmaps\" } instead")
	case "k8s-events":
		// k8s-events uses the native Kubernetes Events API, no additional config needed
		// This is handled by the K8sEventsAdapter which is created separately
		// We just need to mark it as k8s-events ingester type
	default:
		return nil, fmt.Errorf("unknown ingester type: %s", ingester)
	}

	// Parse normalization
	config.Normalization = parseNormalizationConfig(spec)

	// Parse thresholds
	config.Thresholds = parseThresholdsConfig(spec)

	// Parse rate limit
	config.RateLimit = parseRateLimitConfig(spec)

	return config, nil
}

func parseInformerConfig(spec map[string]interface{}) *generic.InformerConfig {
	informerSpec, ok := spec["informer"].(map[string]interface{})
	if !ok {
		return nil
	}

	gvrSpec, _ := informerSpec["gvr"].(map[string]interface{})
	config := &generic.InformerConfig{
		GVR: generic.GVRConfig{
			Group:    getStringValue(gvrSpec, "group"),
			Version:  getStringValue(gvrSpec, "version"),
			Resource: getStringValue(gvrSpec, "resource"),
		},
		Namespace:     getStringValue(informerSpec, "namespace"),
		LabelSelector: getStringValue(informerSpec, "labelSelector"),
		FieldSelector: getStringValue(informerSpec, "fieldSelector"),
		ResyncPeriod:  getStringValue(informerSpec, "resyncPeriod"),
	}

	if config.ResyncPeriod == "" {
		config.ResyncPeriod = "0"
	}

	return config
}

func parseWebhookConfig(spec map[string]interface{}) *generic.WebhookConfig {
	webhookSpec, ok := spec["webhook"].(map[string]interface{})
	if !ok {
		return nil
	}

	config := &generic.WebhookConfig{
		Path:       getStringValue(webhookSpec, "path"),
		Port:       getInt(webhookSpec, "port"),
		BufferSize: getInt(webhookSpec, "bufferSize"),
	}

	if config.BufferSize == 0 {
		config.BufferSize = 100
	}

	authSpec, ok := webhookSpec["auth"].(map[string]interface{})
	if ok {
		config.Auth = &generic.AuthConfig{
			Type:       getStringValue(authSpec, "type"),
			SecretName: getStringValue(authSpec, "secretName"),
		}
		if config.Auth.Type == "" {
			config.Auth.Type = "none"
		}
	}

	return config
}

func parseLogsConfig(spec map[string]interface{}) *generic.LogsConfig {
	logsSpec, ok := spec["logs"].(map[string]interface{})
	if !ok {
		return nil
	}

	config := &generic.LogsConfig{
		PodSelector:  getStringValue(logsSpec, "podSelector"),
		Container:    getStringValue(logsSpec, "container"),
		SinceSeconds: getInt(logsSpec, "sinceSeconds"),
		PollInterval: getStringValue(logsSpec, "pollInterval"),
	}

	if config.SinceSeconds == 0 {
		config.SinceSeconds = 300
	}
	if config.PollInterval == "" {
		config.PollInterval = "1s"
	}

	patterns, ok := logsSpec["patterns"].([]interface{})
	if ok {
		config.Patterns = make([]generic.LogPattern, 0, len(patterns))
		for _, p := range patterns {
			patternMap, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			pattern := generic.LogPattern{
				Regex:    getStringValue(patternMap, "regex"),
				Type:     getStringValue(patternMap, "type"),
				Priority: getFloat(patternMap, "priority"),
			}
			config.Patterns = append(config.Patterns, pattern)
		}
	}

	return config
}

func parseConfigMapConfig(spec map[string]interface{}) *generic.ConfigMapConfig {
	cmSpec, ok := spec["configmap"].(map[string]interface{})
	if !ok {
		return nil
	}

	config := &generic.ConfigMapConfig{
		Namespace:     getStringValue(cmSpec, "namespace"),
		LabelSelector: getStringValue(cmSpec, "labelSelector"),
		PollInterval:  getStringValue(cmSpec, "pollInterval"),
		JSONPath:      getStringValue(cmSpec, "jsonPath"),
	}

	if config.PollInterval == "" {
		config.PollInterval = "5m"
	}

	return config
}

func parseNormalizationConfig(spec map[string]interface{}) *generic.NormalizationConfig {
	normSpec, ok := spec["normalization"].(map[string]interface{})
	if !ok {
		return nil
	}

	config := &generic.NormalizationConfig{
		Domain: getStringValue(normSpec, "domain"),
		Type:   getStringValue(normSpec, "type"),
	}

	priorityMap, ok := normSpec["priority"].(map[string]interface{})
	if ok {
		config.Priority = make(map[string]float64)
		for k, v := range priorityMap {
			if f, ok := v.(float64); ok {
				config.Priority[k] = f
			}
		}
	}

	fieldMappings, ok := normSpec["fieldMapping"].([]interface{})
	if ok {
		config.FieldMapping = make([]generic.FieldMapping, 0, len(fieldMappings))
		for _, fm := range fieldMappings {
			fmMap, ok := fm.(map[string]interface{})
			if !ok {
				continue
			}
			config.FieldMapping = append(config.FieldMapping, generic.FieldMapping{
				From: getStringValue(fmMap, "from"),
				To:   getStringValue(fmMap, "to"),
			})
		}
	}

	return config
}

func parseThresholdsConfig(spec map[string]interface{}) *generic.ThresholdsConfig {
	thresholdsSpec, ok := spec["thresholds"].(map[string]interface{})
	if !ok {
		return nil
	}

	config := &generic.ThresholdsConfig{}

	obsPerMin, ok := thresholdsSpec["observationsPerMinute"].(map[string]interface{})
	if ok {
		config.ObservationsPerMinute = &generic.ThresholdValues{
			Warning:  getInt(obsPerMin, "warning"),
			Critical: getInt(obsPerMin, "critical"),
		}
	}

	custom, ok := thresholdsSpec["custom"].([]interface{})
	if ok {
		config.Custom = make([]generic.CustomThreshold, 0, len(custom))
		for _, c := range custom {
			cMap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			config.Custom = append(config.Custom, generic.CustomThreshold{
				Name:     getStringValue(cMap, "name"),
				Field:    getStringValue(cMap, "field"),
				Operator: getStringValue(cMap, "operator"),
				Value:    cMap["value"],
				Message:  getStringValue(cMap, "message"),
			})
		}
	}

	return config
}

func parseRateLimitConfig(spec map[string]interface{}) *generic.RateLimitConfig {
	rlSpec, ok := spec["rateLimit"].(map[string]interface{})
	if !ok {
		return nil
	}

	config := &generic.RateLimitConfig{
		ObservationsPerMinute: getInt(rlSpec, "observationsPerMinute"),
		Burst:                 getInt(rlSpec, "burst"),
	}

	if config.Burst == 0 {
		config.Burst = 2
	}

	return config
}

// Helper functions
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return 0
}

func getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return 0.0
}
