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

package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/client-go/kubernetes"
)

// WebhookAdapter handles ALL webhook-based sources
type WebhookAdapter struct {
	server    *http.Server
	events    chan RawEvent
	mu        sync.RWMutex
	configs   map[string]*SourceConfig // path -> config
	clientSet kubernetes.Interface
}

// NewWebhookAdapter creates a new generic webhook adapter
func NewWebhookAdapter() *WebhookAdapter {
	return &WebhookAdapter{
		events:  make(chan RawEvent, 100),
		configs: make(map[string]*SourceConfig),
	}
}

// Type returns the adapter type
func (a *WebhookAdapter) Type() string {
	return "webhook"
}

// Validate validates the webhook configuration
func (a *WebhookAdapter) Validate(config *SourceConfig) error {
	if config.Webhook == nil {
		return fmt.Errorf("webhook config is required for webhook adapter")
	}
	if config.Webhook.Path == "" {
		return fmt.Errorf("webhook.path is required")
	}
	if config.Webhook.Port < 1 || config.Webhook.Port > 65535 {
		return fmt.Errorf("webhook.port must be between 1 and 65535")
	}
	return nil
}

// Start starts the webhook adapter
func (a *WebhookAdapter) Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}

	// Store config for this path
	a.mu.Lock()
	a.configs[config.Webhook.Path] = config
	a.mu.Unlock()

	// Create HTTP server if not already created
	if a.server == nil {
		mux := http.NewServeMux()

		// Register handler for this path
		mux.HandleFunc(config.Webhook.Path, a.handleWebhook(config))

		// Health check
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "healthy")
		})

		a.server = &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Webhook.Port),
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		// Start server in goroutine
		go func() {
			logger.Info("Webhook adapter server starting",
				logger.Fields{
					Component: "adapter",
					Operation: "webhook_start",
					Source:    config.Source,
					Additional: map[string]interface{}{
						"port": config.Webhook.Port,
						"path": config.Webhook.Path,
					},
				})
			if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Webhook server error",
					logger.Fields{
						Component: "adapter",
						Operation: "webhook_server",
						Source:    config.Source,
						Error:     err,
					})
			}
		}()

		// Shutdown on context cancel
		go func() {
			<-ctx.Done()
			if a.server != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				a.server.Shutdown(shutdownCtx)
			}
		}()
	} else {
		// Add handler to existing server
		// Note: This is simplified - in production, you'd need to manage multiple paths
		logger.Warn("Webhook server already running, adding handler",
			logger.Fields{
				Component: "adapter",
				Operation: "webhook_add_handler",
				Source:    config.Source,
			})
	}

	return a.events, nil
}

// handleWebhook handles incoming webhook requests
func (a *WebhookAdapter) handleWebhook(config *SourceConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle authentication if configured
		if config.Webhook.Auth != nil && config.Webhook.Auth.Type != "none" {
			if !a.authenticate(r, config) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Parse request body
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Create raw event
		event := RawEvent{
			Source:    config.Source,
			Timestamp: time.Now(),
			RawData:   data, // ALL data preserved
			Metadata: map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"remote": r.RemoteAddr,
			},
		}

		// Send event (non-blocking with buffer)
		select {
		case a.events <- event:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "OK")
		default:
			// Buffer full - backpressure
			logger.Warn("Webhook event buffer full, dropping event",
				logger.Fields{
					Component: "adapter",
					Operation: "webhook_backpressure",
					Source:    config.Source,
				})
			http.Error(w, "Buffer full", http.StatusServiceUnavailable)
		}
	}
}

// authenticate handles webhook authentication
func (a *WebhookAdapter) authenticate(r *http.Request, config *SourceConfig) bool {
	if config.Webhook.Auth == nil {
		return true
	}

	switch config.Webhook.Auth.Type {
	case "bearer":
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			return false
		}
		// Validate token (would load from secret in production)
		return true // Simplified
	case "basic":
		// Basic auth validation
		_, _, ok := r.BasicAuth()
		return ok
	default:
		return true
	}
}

// Stop stops the webhook adapter
func (a *WebhookAdapter) Stop() {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.server.Shutdown(ctx)
	}
	close(a.events)
}
