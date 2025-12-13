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
	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
)

// ConvertIngesterConfigToGeneric converts an IngesterConfig to generic.SourceConfig
func ConvertIngesterConfigToGeneric(ingesterConfig *IngesterConfig) *generic.SourceConfig {
	if ingesterConfig == nil {
		return nil
	}

	config := &generic.SourceConfig{
		Source:   ingesterConfig.Source,
		Ingester: ingesterConfig.Ingester,
	}

	// Convert informer config
	if ingesterConfig.Informer != nil {
		config.Informer = &generic.InformerConfig{
			GVR: generic.GVRConfig{
				Group:    ingesterConfig.Informer.GVR.Group,
				Version:  ingesterConfig.Informer.GVR.Version,
				Resource: ingesterConfig.Informer.GVR.Resource,
			},
			Namespace:     ingesterConfig.Informer.Namespace,
			LabelSelector: ingesterConfig.Informer.LabelSelector,
			ResyncPeriod:  ingesterConfig.Informer.ResyncPeriod,
		}
	}

	// Convert webhook config
	if ingesterConfig.Webhook != nil {
		config.Webhook = &generic.WebhookConfig{
			Path: ingesterConfig.Webhook.Path,
		}
		if ingesterConfig.Webhook.Auth != nil {
			config.Webhook.Auth = &generic.AuthConfig{
				Type:       ingesterConfig.Webhook.Auth.Type,
				SecretName: ingesterConfig.Webhook.Auth.SecretRef,
			}
		}
	}

	// Convert normalization config
	if ingesterConfig.Normalization != nil {
		config.Normalization = &generic.NormalizationConfig{
			Domain:   ingesterConfig.Normalization.Domain,
			Type:     ingesterConfig.Normalization.Type,
			Priority: ingesterConfig.Normalization.Priority,
		}
		// Convert field mappings
		for _, fm := range ingesterConfig.Normalization.FieldMapping {
			config.Normalization.FieldMapping = append(config.Normalization.FieldMapping, generic.FieldMapping{
				From: fm.From,
				To:   fm.To,
			})
		}
	}

	// Convert dedup config (W33 - v1.1)
	if ingesterConfig.Dedup != nil {
		config.Dedup = &generic.DedupConfig{
			Enabled:            ingesterConfig.Dedup.Enabled,
			Window:             ingesterConfig.Dedup.Window,
			Strategy:           ingesterConfig.Dedup.Strategy,
			Fields:             ingesterConfig.Dedup.Fields,
			MaxEventsPerWindow: ingesterConfig.Dedup.MaxEventsPerWindow,
		}
		// Default strategy if not set
		if config.Dedup.Strategy == "" {
			config.Dedup.Strategy = "fingerprint"
		}
	}

	// Convert processing config (W33 - v1.1)
	if ingesterConfig.Processing != nil {
		config.Processing = &generic.ProcessingConfig{
			Order:               ingesterConfig.Processing.Order,
			AutoOptimize:        ingesterConfig.Processing.AutoOptimize,
			AnalysisInterval:    ingesterConfig.Processing.AnalysisInterval,
			ConfidenceThreshold: ingesterConfig.Processing.ConfidenceThreshold,
		}
	}

	return config
}
