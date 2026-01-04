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

package generic

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WebhookAdapter handles ALL webhook-based sources
type WebhookAdapter struct {
	server         *http.Server
	events         chan RawEvent
	mu             sync.RWMutex
	configs        map[string]*SourceConfig // path -> config
	clientSet      kubernetes.Interface
	secretCache    map[string]*secretCacheEntry // namespace/secretName -> cached secret
	secretMu       sync.RWMutex
	webhookMetrics *prometheus.CounterVec // Metrics for webhook requests (optional)
	webhookDropped *prometheus.CounterVec // Metrics for webhook events dropped (optional)
}

type secretCacheEntry struct {
	secret    *corev1.Secret
	timestamp time.Time
}

// NewWebhookAdapter creates a new generic webhook adapter
func NewWebhookAdapter(clientSet kubernetes.Interface) *WebhookAdapter {
	return NewWebhookAdapterWithMetrics(clientSet, nil, nil)
}

// NewWebhookAdapterWithMetrics creates a new generic webhook adapter with metrics support
func NewWebhookAdapterWithMetrics(clientSet kubernetes.Interface, webhookMetrics *prometheus.CounterVec, webhookDropped *prometheus.CounterVec) *WebhookAdapter {
	return &WebhookAdapter{
		events:         make(chan RawEvent, 100),
		configs:        make(map[string]*SourceConfig),
		clientSet:      clientSet,
		secretCache:    make(map[string]*secretCacheEntry),
		webhookMetrics: webhookMetrics,
		webhookDropped: webhookDropped,
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
	// Require SecretName when auth is enabled and not "none"
	if config.Webhook.Auth != nil && config.Webhook.Auth.Type != "none" {
		if config.Webhook.Auth.SecretName == "" {
			return fmt.Errorf("webhook.auth.secretName is required when auth.type is %s", config.Webhook.Auth.Type)
		}
		if config.Namespace == "" {
			return fmt.Errorf("namespace is required when webhook auth is enabled")
		}
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
			_, _ = fmt.Fprintf(w, "healthy")
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
			logger := sdklog.NewLogger("zen-watcher-adapter")
			logger.Info("Webhook adapter server starting",
				sdklog.Operation("webhook_start"),
				sdklog.String("source", config.Source),
				sdklog.Int("port", config.Webhook.Port),
				sdklog.String("path", config.Webhook.Path))
			if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger := sdklog.NewLogger("zen-watcher-adapter")
				logger.Error(err, "Webhook server error",
					sdklog.Operation("webhook_server"),
					sdklog.String("source", config.Source))
			}
		}()

		// Shutdown on context cancel
		go func() {
			<-ctx.Done()
			if a.server != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := a.server.Shutdown(shutdownCtx); err != nil {
					logger := sdklog.NewLogger("zen-watcher-adapter")
					logger.Warn("Webhook server shutdown error",
						sdklog.Operation("webhook_shutdown"),
						sdklog.String("source", config.Source),
						sdklog.Error(err))
				}
			}
		}()
	} else {
		// Add handler to existing server
		// Note: This is simplified - in production, you'd need to manage multiple paths
		logger := sdklog.NewLogger("zen-watcher-adapter")
		logger.Warn("Webhook server already running, adding handler",
			sdklog.Operation("webhook_add_handler"),
			sdklog.String("source", config.Source))
	}

	return a.events, nil
}

// handleWebhook handles incoming webhook requests
func (a *WebhookAdapter) handleWebhook(config *SourceConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Use source name as endpoint label for metrics
		endpoint := config.Source

		// Handle authentication if configured
		if config.Webhook.Auth != nil && config.Webhook.Auth.Type != "none" {
			if !a.authenticate(r, config) {
				// Track authentication failure in metrics
				if a.webhookMetrics != nil {
					a.webhookMetrics.WithLabelValues(endpoint, "401").Inc()
				}
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Limit request body size to prevent DoS attacks (default: 1MiB)
		maxRequestBytes := int64(1048576) // 1MiB default
		if maxBytesStr := os.Getenv("SERVER_MAX_REQUEST_BYTES"); maxBytesStr != "" {
			if parsed, err := strconv.ParseInt(maxBytesStr, 10, 64); err == nil && parsed > 0 {
				maxRequestBytes = parsed
			}
		}
		limitedBody := http.MaxBytesReader(w, r.Body, maxRequestBytes)
		defer func() { _ = limitedBody.Close() }()

		// Parse request body
		var data map[string]interface{}
		if err := json.NewDecoder(limitedBody).Decode(&data); err != nil {
			if err == io.EOF {
				// Track request body too large in metrics
				if a.webhookMetrics != nil {
					a.webhookMetrics.WithLabelValues(endpoint, "413").Inc()
				}
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			// Track parse error in metrics
			if a.webhookMetrics != nil {
				a.webhookMetrics.WithLabelValues(endpoint, "400").Inc()
			}
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
			// Track successful request in metrics
			if a.webhookMetrics != nil {
				a.webhookMetrics.WithLabelValues(endpoint, "200").Inc()
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "OK")
		default:
			// Buffer full - backpressure
			logger := sdklog.NewLogger("zen-watcher-adapter")
			logger.Warn("Webhook event buffer full, dropping event",
				sdklog.Operation("webhook_backpressure"),
				sdklog.String("source", config.Source))
			// Track service unavailable in metrics
			if a.webhookMetrics != nil {
				a.webhookMetrics.WithLabelValues(endpoint, "503").Inc()
			}
			if a.webhookDropped != nil {
				a.webhookDropped.WithLabelValues(endpoint).Inc()
			}
			http.Error(w, "Buffer full", http.StatusServiceUnavailable)
		}
	}
}

// loadSecret loads a secret from Kubernetes, with caching (5 minute TTL)
func (a *WebhookAdapter) loadSecret(ctx context.Context, namespace, secretName string) (*corev1.Secret, error) {
	cacheKey := fmt.Sprintf("%s/%s", namespace, secretName)

	// Check cache first
	a.secretMu.RLock()
	if entry, found := a.secretCache[cacheKey]; found {
		// Cache is valid for 5 minutes
		if time.Since(entry.timestamp) < 5*time.Minute {
			a.secretMu.RUnlock()
			return entry.secret, nil
		}
	}
	a.secretMu.RUnlock()

	// Load from Kubernetes
	secret, err := a.clientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to load secret %s/%s: %w", namespace, secretName, err)
	}

	// Update cache
	a.secretMu.Lock()
	a.secretCache[cacheKey] = &secretCacheEntry{
		secret:    secret,
		timestamp: time.Now(),
	}
	a.secretMu.Unlock()

	return secret, nil
}

// authenticate handles webhook authentication
// Security: If Auth is configured (non-nil) and Type is not "none", authentication is REQUIRED.
// Returns false (rejects request) if:
// - Auth is configured but SecretName is missing
// - Secret cannot be loaded
// - Authentication fails
// Returns true only if:
// - Auth is nil (no auth required) OR
// - Auth.Type == "none" (explicitly no auth) OR
// - Auth is properly configured AND validation succeeds
func (a *WebhookAdapter) authenticate(r *http.Request, config *SourceConfig) bool {
	if config.Webhook.Auth == nil {
		return true // No auth required
	}

	// If auth type is explicitly "none", allow
	if config.Webhook.Auth.Type == "none" {
		return true
	}

	// Auth is required - ensure SecretName is configured
	if config.Webhook.Auth.SecretName == "" {
		logger := sdklog.NewLogger("zen-watcher-adapter")
		logger.Warn("Webhook auth enabled but secretName not configured - rejecting request",
			sdklog.Operation("auth_validate"),
			sdklog.String("source", config.Source),
			sdklog.String("path", config.Webhook.Path),
			sdklog.String("auth_type", config.Webhook.Auth.Type))
		return false // Hard-fail: auth required but not properly configured
	}

	secret, err := a.loadSecret(r.Context(), config.Namespace, config.Webhook.Auth.SecretName)
	if err != nil {
		logger := sdklog.NewLogger("zen-watcher-adapter")
		if errors.IsNotFound(err) {
			logger.Warn("Secret not found for webhook auth",
				sdklog.Operation("auth_secret_load"),
				sdklog.String("source", config.Source),
				sdklog.String("namespace", config.Namespace),
				sdklog.String("secret", config.Webhook.Auth.SecretName))
		} else {
			logger.Warn("Failed to load secret for webhook auth",
				sdklog.Operation("auth_secret_load"),
				sdklog.String("source", config.Source),
				sdklog.String("namespace", config.Namespace),
				sdklog.String("secret", config.Webhook.Auth.SecretName),
				sdklog.Error(err))
		}
		return false
	}

	switch config.Webhook.Auth.Type {
	case "bearer":
		return a.authenticateBearer(r, secret)
	case "basic":
		return a.authenticateBasic(r, secret)
	default:
		logger := sdklog.NewLogger("zen-watcher-adapter")
		logger.Warn("Unsupported auth type",
			sdklog.Operation("auth_validate"),
			sdklog.String("source", config.Source),
			sdklog.String("type", config.Webhook.Auth.Type))
		return false
	}
}

// authenticateBearer validates bearer token authentication
func (a *WebhookAdapter) authenticateBearer(r *http.Request, secret *corev1.Secret) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// Extract token (support "Bearer <token>" or just "<token>")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}

	// Get expected token from secret
	// Secret format: key "token" contains the expected bearer token
	expectedTokenBytes, found := secret.Data["token"]
	if !found {
		logger := sdklog.NewLogger("zen-watcher-adapter")
		logger.Warn("Secret missing 'token' key for bearer auth",
			sdklog.Operation("auth_bearer_validate"),
			sdklog.String("namespace", secret.Namespace),
			sdklog.String("secret", secret.Name))
		return false
	}

	expectedToken := string(expectedTokenBytes)

	// Constant-time comparison
	return subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1
}

// authenticateBasic validates basic authentication
func (a *WebhookAdapter) authenticateBasic(r *http.Request, secret *corev1.Secret) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	// Get expected credentials from secret
	// Secret format:
	// - key "username" contains the expected username
	// - key "password" contains the bcrypt-hashed password (or plain text for v0)
	expectedUsernameBytes, usernameFound := secret.Data["username"]
	expectedPasswordBytes, passwordFound := secret.Data["password"]

	if !usernameFound || !passwordFound {
		logger := sdklog.NewLogger("zen-watcher-adapter")
		logger.Warn("Secret missing 'username' or 'password' key for basic auth",
			sdklog.Operation("auth_basic_validate"),
			sdklog.String("namespace", secret.Namespace),
			sdklog.String("secret", secret.Name))
		return false
	}

	expectedUsername := string(expectedUsernameBytes)
	expectedPasswordHash := string(expectedPasswordBytes)

	// Constant-time username comparison
	if subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) != 1 {
		return false
	}

	// Check if password is bcrypt-hashed (starts with $2a$, $2b$, or $2y$)
	if strings.HasPrefix(expectedPasswordHash, "$2a$") ||
		strings.HasPrefix(expectedPasswordHash, "$2b$") ||
		strings.HasPrefix(expectedPasswordHash, "$2y$") {
		// Use bcrypt comparison
		err := bcrypt.CompareHashAndPassword([]byte(expectedPasswordHash), []byte(password))
		return err == nil
	}

	// Plain text password (for v0 compatibility) - constant-time comparison
	return subtle.ConstantTimeCompare([]byte(password), []byte(expectedPasswordHash)) == 1
}

// Stop stops the webhook adapter
func (a *WebhookAdapter) Stop() {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			logger := sdklog.NewLogger("zen-watcher-adapter")
			logger.Warn("Webhook server shutdown error",
				sdklog.Operation("webhook_stop"),
				sdklog.Error(err))
		}
	}
	close(a.events)
}
