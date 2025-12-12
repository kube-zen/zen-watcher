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

package webhook

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// EndpointRegistrar is an interface for registering/unregistering endpoints
// This allows the controller to work with the HTTP server's endpoint registry
type EndpointRegistrar interface {
	RegisterDynamicEndpoint(config *EndpointConfig) error
	UnregisterDynamicEndpoint(source string) error
}

// IngesterWebhookController watches Ingester CRDs and registers webhook endpoints
type IngesterWebhookController struct {
	registrar     EndpointRegistrar
	ingesterStore *config.IngesterStore
	dynClient     dynamic.Interface
	factory       dynamicinformer.DynamicSharedInformerFactory
	informer      cache.SharedInformer
	mu            sync.RWMutex
}

// NewIngesterWebhookController creates a new controller for webhook Ingester CRDs
func NewIngesterWebhookController(
	registrar EndpointRegistrar,
	ingesterStore *config.IngesterStore,
	dynClient dynamic.Interface,
) *IngesterWebhookController {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 10*time.Minute)

	return &IngesterWebhookController{
		registrar:     registrar,
		ingesterStore: ingesterStore,
		dynClient:     dynClient,
		factory:       factory,
	}
}

// Start starts watching Ingester CRDs for webhook configurations
func (c *IngesterWebhookController) Start(ctx context.Context) error {
	logger.Info("Starting Ingester webhook controller",
		logger.Fields{
			Component: "webhook",
			Operation: "ingester_webhook_controller_start",
		})

	// Get informer for Ingester CRDs
	c.informer = c.factory.ForResource(config.IngesterGVR).Informer()

	// Set up event handlers
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAdd,
		UpdateFunc: c.onUpdate,
		DeleteFunc: c.onDelete,
	}

	c.informer.AddEventHandler(handlers)

	// Start the informer factory
	c.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("failed to sync Ingester informer cache")
	}

	logger.Info("Ingester webhook controller started and synced",
		logger.Fields{
			Component: "webhook",
			Operation: "ingester_webhook_controller_synced",
		})

	// Register existing webhook Ingesters that were present before the controller started
	// Add a small delay to ensure event handlers have finished registering endpoints
	// from the initial informer sync. This prevents race conditions where registerExistingWebhooks
	// tries to register an endpoint that was just registered by an event handler.
	// The delay is minimal (2 seconds) and only affects startup, not runtime performance.
	time.Sleep(2 * time.Second)

	// This is safe because RegisterDynamicEndpoint checks for duplicates, but the delay
	// ensures the check can properly see already-registered paths
	c.registerExistingWebhooks()

	return nil
}

// Stop stops the controller
func (c *IngesterWebhookController) Stop() {
	// Informer will stop when context is cancelled
}

// onAdd handles Ingester CRD add events
func (c *IngesterWebhookController) onAdd(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}

	c.handleIngesterChange(u)
}

// onUpdate handles Ingester CRD update events
func (c *IngesterWebhookController) onUpdate(oldObj, newObj interface{}) {
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		return
	}

	c.handleIngesterChange(u)
}

// onDelete handles Ingester CRD delete events
func (c *IngesterWebhookController) onDelete(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Handle deletedFinalStateUnknown
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			u, ok = tombstone.Obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
		} else {
			return
		}
	}

	c.handleIngesterDelete(u)
}

// handleIngesterChange processes an Ingester CRD change
func (c *IngesterWebhookController) handleIngesterChange(u *unstructured.Unstructured) {
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		return
	}

	// Check if this is a webhook ingester
	ingesterType, ok := spec["ingester"].(string)
	if !ok || ingesterType != "webhook" {
		return
	}

	// Get source name (required)
	source, ok := spec["source"].(string)
	if !ok || source == "" {
		logger.Warn("Ingester CRD missing source field",
			logger.Fields{
				Component: "webhook",
				Operation: "ingester_webhook_register",
				Namespace: u.GetNamespace(),
				Additional: map[string]interface{}{
					"name": u.GetName(),
				},
			})
		return
	}

	// Get webhook configuration
	webhookSpec, ok := spec["webhook"].(map[string]interface{})
	if !ok {
		logger.Warn("Ingester CRD missing webhook configuration",
			logger.Fields{
				Component: "webhook",
				Operation: "ingester_webhook_register",
				Namespace: u.GetNamespace(),
				Source:    source,
				Additional: map[string]interface{}{
					"name": u.GetName(),
				},
			})
		return
	}

	// Build endpoint config from Ingester CRD
	endpointConfig := c.buildEndpointConfig(u, source, webhookSpec, spec)

	// Register endpoint via registrar
	if err := c.registrar.RegisterDynamicEndpoint(endpointConfig); err != nil {
		logger.Error("Failed to register webhook endpoint",
			logger.Fields{
				Component: "webhook",
				Operation: "ingester_webhook_register",
				Namespace: u.GetNamespace(),
				Source:    source,
				Error:     err,
				Additional: map[string]interface{}{
					"name": u.GetName(),
					"path": endpointConfig.Path,
				},
			})
		return
	}

	logger.Info("Registered webhook endpoint from Ingester CRD",
		logger.Fields{
			Component: "webhook",
			Operation: "ingester_webhook_registered",
			Namespace: u.GetNamespace(),
			Source:    source,
			Additional: map[string]interface{}{
				"name":     u.GetName(),
				"path":     endpointConfig.Path,
				"methods":  endpointConfig.Methods,
				"authType": getString(webhookSpec, "auth", "type"),
			},
		})
}

// handleIngesterDelete processes an Ingester CRD deletion
func (c *IngesterWebhookController) handleIngesterDelete(u *unstructured.Unstructured) {
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		return
	}

	// Check if this is a webhook ingester
	ingesterType, ok := spec["ingester"].(string)
	if !ok || ingesterType != "webhook" {
		return
	}

	// Get source name
	source, ok := spec["source"].(string)
	if !ok || source == "" {
		return
	}

	// Unregister endpoint via registrar
	if err := c.registrar.UnregisterDynamicEndpoint(source); err != nil {
		logger.Warn("Failed to unregister webhook endpoint",
			logger.Fields{
				Component: "webhook",
				Operation: "ingester_webhook_unregister",
				Namespace: u.GetNamespace(),
				Source:    source,
				Error:     err,
				Additional: map[string]interface{}{
					"name": u.GetName(),
				},
			})
		return
	}

	logger.Info("Unregistered webhook endpoint from Ingester CRD",
		logger.Fields{
			Component: "webhook",
			Operation: "ingester_webhook_unregistered",
			Namespace: u.GetNamespace(),
			Source:    source,
			Additional: map[string]interface{}{
				"name": u.GetName(),
			},
		})
}

// buildEndpointConfig builds an EndpointConfig from Ingester CRD
func (c *IngesterWebhookController) buildEndpointConfig(
	u *unstructured.Unstructured,
	source string,
	webhookSpec map[string]interface{},
	spec map[string]interface{},
) *EndpointConfig {
	// Get path - use spec.webhook.path or default to /ingest/{source}
	path := getString(webhookSpec, "path")
	if path == "" {
		path = fmt.Sprintf("/ingest/%s", source)
	}

	// Get methods - default to POST
	methods := []string{"POST"}
	if methodsVal, ok := webhookSpec["methods"].([]interface{}); ok {
		methods = make([]string, 0, len(methodsVal))
		for _, m := range methodsVal {
			if mStr, ok := m.(string); ok {
				methods = append(methods, mStr)
			}
		}
	}

	// Build auth config
	var authConfig *AuthConfig
	if auth, ok := webhookSpec["auth"].(map[string]interface{}); ok {
		authType := getString(auth, "type")
		if authType == "" {
			authType = "none"
		}
		secretName := getString(auth, "secretName")
		if secretName == "" {
			secretName = getString(auth, "secretRef") // Support both field names
		}

		authConfig = &AuthConfig{
			Enabled:    authType != "none",
			Type:       authType,
			SecretName: secretName,
		}
	}

	// Build rate limit config
	var rateLimitConfig *RateLimitConfig
	if rateLimit, ok := webhookSpec["rateLimit"].(map[string]interface{}); ok {
		enabled := true
		if enabledVal, ok := rateLimit["enabled"].(bool); ok {
			enabled = enabledVal
		}

		// Support both requestsPerSecond and requestsPerMinute
		rps := 0.0
		if rpsVal, ok := rateLimit["requestsPerSecond"].(float64); ok {
			rps = rpsVal
		} else if rpmVal, ok := rateLimit["requestsPerMinute"].(float64); ok {
			rps = rpmVal / 60.0 // Convert to requests per second
		}

		burst := 0
		if burstVal, ok := rateLimit["burst"].(float64); ok {
			burst = int(burstVal)
		}

		if enabled && (rps > 0 || burst > 0) {
			// Convert RPS to RPM for our RateLimitConfig
			rpm := int(rps * 60)
			if rpm == 0 && rps > 0 {
				rpm = 1 // Minimum 1 RPM
			}
			if burst == 0 {
				burst = rpm // Default burst to RPM
			}

			rateLimitConfig = &RateLimitConfig{
				Enabled: true,
				RPM:     rpm,
				Burst:   burst,
			}
		}
	}

	// Build processing config from Ingester destinations and processing
	var processingConfig *ProcessingConfig
	if destinations, ok := spec["destinations"].([]interface{}); ok && len(destinations) > 0 {
		// Extract mapping from first destination (typically CRD destination)
		if dest, ok := destinations[0].(map[string]interface{}); ok {
			if mapping, ok := dest["mapping"].(map[string]interface{}); ok {
				processingConfig = &ProcessingConfig{
					Outputs: []OutputConfig{
						{
							Type:    getString(dest, "type"),
							Value:   getString(dest, "value"),
							Mapping: mapping,
						},
					},
				}
			}
		}
	}

	// Add filter and dedup from processing config
	if processing, ok := spec["processing"].(map[string]interface{}); ok {
		if processingConfig == nil {
			processingConfig = &ProcessingConfig{}
		}

		// Extract filter config
		if filter, ok := processing["filter"].(map[string]interface{}); ok {
			if enabled, ok := filter["enabled"].(bool); ok && enabled {
				processingConfig.Filters = []FilterRule{
					{
						Field:    "priority",
						Operator: "gte",
						Value:    fmt.Sprintf("%v", filter["minPriority"]),
					},
				}
			}
		}

		// Extract dedup config
		if dedup, ok := processing["dedup"].(map[string]interface{}); ok {
			if enabled, ok := dedup["enabled"].(bool); ok && enabled {
				processingConfig.Dedup = &DedupConfig{
					Enabled:  true,
					Window:   getString(dedup, "window"),
					Strategy: getString(dedup, "strategy"),
				}
			}
		}
	}

	return &EndpointConfig{
		Source:     source,
		Path:       path,
		Methods:    methods,
		Auth:       authConfig,
		RateLimit:  rateLimitConfig,
		Processing: processingConfig,
	}
}

// registerExistingWebhooks registers all existing webhook Ingesters
// This is called after the informer syncs to ensure all existing Ingesters are registered.
// It's safe to call even if event handlers have already registered some endpoints,
// because RegisterDynamicEndpoint checks for duplicates before registering.
func (c *IngesterWebhookController) registerExistingWebhooks() {
	if c.ingesterStore == nil {
		return
	}

	// Get all Ingesters from store
	allConfigs := c.ingesterStore.ListAll()
	for _, ingesterConfig := range allConfigs {
		if ingesterConfig.Ingester == "webhook" && ingesterConfig.Webhook != nil {
			// Rebuild endpoint config from IngesterConfig
			path := ingesterConfig.Webhook.Path
			if path == "" {
				path = fmt.Sprintf("/ingest/%s", ingesterConfig.Source)
			}

			endpointConfig := &EndpointConfig{
				Source:  ingesterConfig.Source,
				Path:    path,
				Methods: []string{"POST"}, // Default
			}

			if ingesterConfig.Webhook.Auth != nil {
				endpointConfig.Auth = &AuthConfig{
					Type:       ingesterConfig.Webhook.Auth.Type,
					SecretName: ingesterConfig.Webhook.Auth.SecretRef,
					Enabled:    ingesterConfig.Webhook.Auth.Type != "none",
				}
			}

			if ingesterConfig.Webhook.RateLimit != nil {
				endpointConfig.RateLimit = &RateLimitConfig{
					Enabled: true,
					RPM:     ingesterConfig.Webhook.RateLimit.RequestsPerMinute,
					Burst:   ingesterConfig.Webhook.RateLimit.RequestsPerMinute,
				}
			}

			// Register endpoint - RegisterDynamicEndpoint will check for duplicates
			if err := c.registrar.RegisterDynamicEndpoint(endpointConfig); err != nil {
				logger.Warn("Failed to register existing webhook endpoint",
					logger.Fields{
						Component: "webhook",
						Operation: "ingester_webhook_register_existing",
						Source:    ingesterConfig.Source,
						Error:     err,
					})
			}
		}
	}
}

// Helper functions

func getString(m map[string]interface{}, keys ...string) string {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - return the value
			if val, ok := current[key].(string); ok {
				return val
			}
			return ""
		}
		// Navigate nested maps
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}
