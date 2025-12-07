// Copyright 2024 The Zen Watcher Authors
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
	"time"

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
	adapterType, _ := spec["adapterType"].(string)

	config := &generic.SourceConfig{
		Source:      source,
		AdapterType: adapterType,
	}

	// Parse adapter-specific configs
	switch adapterType {
	case "informer":
		config.Informer = parseInformerConfig(spec)
	case "webhook":
		config.Webhook = parseWebhookConfig(spec)
	case "logs":
		config.Logs = parseLogsConfig(spec)
	case "configmap":
		config.ConfigMap = parseConfigMapConfig(spec)
	default:
		return nil, fmt.Errorf("unknown adapter type: %s", adapterType)
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
			Group:    getString(gvrSpec, "group"),
			Version:  getString(gvrSpec, "version"),
			Resource: getString(gvrSpec, "resource"),
		},
		Namespace:     getString(informerSpec, "namespace"),
		LabelSelector: getString(informerSpec, "labelSelector"),
		FieldSelector: getString(informerSpec, "fieldSelector"),
		ResyncPeriod:  getString(informerSpec, "resyncPeriod"),
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
		Path:       getString(webhookSpec, "path"),
		Port:       getInt(webhookSpec, "port"),
		BufferSize: getInt(webhookSpec, "bufferSize"),
	}

	if config.BufferSize == 0 {
		config.BufferSize = 100
	}

	authSpec, ok := webhookSpec["auth"].(map[string]interface{})
	if ok {
		config.Auth = &generic.AuthConfig{
			Type:       getString(authSpec, "type"),
			SecretName: getString(authSpec, "secretName"),
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
		PodSelector:  getString(logsSpec, "podSelector"),
		Container:    getString(logsSpec, "container"),
		SinceSeconds: getInt(logsSpec, "sinceSeconds"),
		PollInterval: getString(logsSpec, "pollInterval"),
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
				Regex:    getString(patternMap, "regex"),
				Type:     getString(patternMap, "type"),
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
		Namespace:     getString(cmSpec, "namespace"),
		LabelSelector: getString(cmSpec, "labelSelector"),
		PollInterval:  getString(cmSpec, "pollInterval"),
		JSONPath:      getString(cmSpec, "jsonPath"),
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
		Domain: getString(normSpec, "domain"),
		Type:   getString(normSpec, "type"),
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
				From: getString(fmMap, "from"),
				To:   getString(fmMap, "to"),
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
				Name:     getString(cMap, "name"),
				Field:    getString(cMap, "field"),
				Operator: getString(cMap, "operator"),
				Value:    cMap["value"],
				Message:  getString(cMap, "message"),
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
func getString(m map[string]interface{}, key string) string {
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

