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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/kube-zen/zen-watcher/pkg/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Server wraps the HTTP server and handlers
type Server struct {
	server           *http.Server
	ready            bool
	readyMu          sync.RWMutex
	falcoAlertsChan  chan map[string]interface{}
	auditEventsChan  chan map[string]interface{}
	webhookMetrics   *prometheus.CounterVec
	webhookDropped   *prometheus.CounterVec
	auth             *WebhookAuth
	rateLimiter      *RateLimiter
	haConfig         *config.HAConfig
	haStatus         *HAStatus
	haStatusMu       sync.RWMutex
	endpointRegistry *webhook.DynamicEndpointRegistry
	dynamicMux       *http.ServeMux // Mux for dynamic endpoints (same as main mux)
	dynamicMuxMu     sync.RWMutex
	registeredPaths  map[string]bool // Track which paths are registered in ServeMux
	ingesterStore    *config.IngesterStore // For accessing Ingester configs
	observationCreator interface { // For creating observations from webhook payloads
		CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error
	}
}

// HAStatus holds current HA status information
type HAStatus struct {
	Enabled      bool    `json:"enabled"`
	ReplicaID    string  `json:"replica_id"`
	CurrentLoad  float64 `json:"current_load"`
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  float64 `json:"memory_usage"`
	EventsPerSec float64 `json:"events_per_sec"`
	QueueDepth   int     `json:"queue_depth"`
	Healthy      bool    `json:"healthy"`
	LastUpdate   string  `json:"last_update"`
}

// NewServer creates a new HTTP server with handlers
func NewServer(falcoChan, auditChan chan map[string]interface{}, webhookMetrics, webhookDropped *prometheus.CounterVec) *Server {
	return NewServerWithIngester(falcoChan, auditChan, webhookMetrics, webhookDropped, nil, nil)
}

// NewServerWithIngester creates a new HTTP server with Ingester support
func NewServerWithIngester(
	falcoChan, auditChan chan map[string]interface{},
	webhookMetrics, webhookDropped *prometheus.CounterVec,
	ingesterStore *config.IngesterStore,
	observationCreator interface {
		CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error
	},
) *Server {
	port := os.Getenv("WATCHER_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	auth := NewWebhookAuth()

	// Rate limiter: 100 requests per minute per IP (configurable)
	maxRequests := 100
	if maxReqStr := os.Getenv("WEBHOOK_RATE_LIMIT"); maxReqStr != "" {
		if parsed, err := parseEnvInt(maxReqStr); err == nil && parsed > 0 {
			maxRequests = parsed
		}
	}
	rateLimiter := NewRateLimiter(maxRequests, 1*time.Minute)

	// Load HA configuration
	haConfig := config.LoadHAConfig()
	replicaID := os.Getenv("HOSTNAME")
	if replicaID == "" {
		replicaID = fmt.Sprintf("replica-%d", time.Now().UnixNano())
	}

	endpointRegistry := webhook.NewDynamicEndpointRegistry()

	s := &Server{
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		falcoAlertsChan:  falcoChan,
		auditEventsChan:  auditChan,
		webhookMetrics:   webhookMetrics,
		webhookDropped:   webhookDropped,
		auth:             auth,
		rateLimiter:      rateLimiter,
		haConfig:         haConfig,
		endpointRegistry:     endpointRegistry,
		dynamicMux:           mux, // Use main mux for dynamic endpoints
		registeredPaths:      make(map[string]bool), // Track registered paths
		ingesterStore:        ingesterStore,
		observationCreator:   observationCreator,
		haStatus: &HAStatus{
			Enabled:    haConfig.IsHAEnabled(),
			ReplicaID:  replicaID,
			Healthy:    true,
			LastUpdate: time.Now().Format(time.RFC3339),
		},
	}

	s.registerHandlers(mux)
	return s
}

// parseEnvInt parses an environment variable as an integer
func parseEnvInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// isPprofEnabled checks if pprof endpoints should be enabled
// Controlled by ENABLE_PPROF environment variable (default: false for security)
func (s *Server) isPprofEnabled() bool {
	enablePprof := os.Getenv("ENABLE_PPROF")
	return enablePprof == "true" || enablePprof == "1"
}

// registerHandlers registers all HTTP handlers
func (s *Server) registerHandlers(mux *http.ServeMux) {
	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "healthy")
	})

	// Readiness probe endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		s.readyMu.RLock()
		ready := s.ready
		s.readyMu.RUnlock()

		if ready {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "ready")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "not ready")
		}
	})

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// pprof endpoints (for performance profiling)
	// Security: Consider restricting /debug/pprof access via NetworkPolicy or authentication in production
	if s.isPprofEnabled() {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		// Additional pprof endpoints
		mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
		mux.Handle("/debug/pprof/block", pprof.Handler("block"))
		mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	}

	// HA-aware endpoints (only if HA is enabled)
	if s.haConfig != nil && s.haConfig.IsHAEnabled() {
		// HA health check endpoint
		mux.HandleFunc("/ha/health", s.handleHAHealth)

		// HA metrics endpoint
		mux.HandleFunc("/ha/metrics", s.handleHAMetrics)

		// HA status endpoint
		mux.HandleFunc("/ha/status", s.handleHAStatus)
	}

	// Falco webhook handler (with authentication and rate limiting)
	falcoHandler := s.auth.RequireAuth(s.handleFalcoWebhook)
	mux.HandleFunc("/falco/webhook", s.rateLimiter.RateLimitMiddleware(falcoHandler))

	// Audit webhook handler (with authentication and rate limiting)
	auditHandler := s.auth.RequireAuth(s.handleAuditWebhook)
	mux.HandleFunc("/audit/webhook", s.rateLimiter.RateLimitMiddleware(auditHandler))

	// Endpoint management API
	mux.HandleFunc("/endpoints", s.handleListEndpoints)
}

// createDynamicWebhookHandler creates a handler for a dynamic webhook endpoint
func (s *Server) createDynamicWebhookHandler(config *webhook.EndpointConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		correlationID := generateCorrelationID(r)
		ctx := logger.WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)

		logger.Info("Dynamic webhook request received",
			logger.Fields{
				Component: "server",
				Operation: "dynamic_webhook",
				Source:    config.Source,
				Additional: map[string]interface{}{
					"path":           config.Path,
					"method":         r.Method,
					"correlation_id": correlationID,
					"remote_addr":    r.RemoteAddr,
				},
			})

		// Validate request method
		if !contains(config.Methods, r.Method) {
			logger.Warn("Dynamic webhook rejected: invalid method",
				logger.Fields{
					Component: "server",
					Operation: "dynamic_webhook",
					Source:    config.Source,
					Reason:    "invalid_method",
					Additional: map[string]interface{}{
						"method":  r.Method,
						"allowed": config.Methods,
					},
				})
			s.webhookMetrics.WithLabelValues(config.Source, "405").Inc()
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error":          "method_not_allowed",
				"correlation_id": correlationID,
			})
			return
		}

		// Parse JSON payload
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			logger.Warn("Failed to parse dynamic webhook payload",
				logger.Fields{
					Component: "server",
					Operation: "dynamic_webhook",
					Source:    config.Source,
					Error:     err,
					Reason:    "parse_error",
				})
			s.webhookMetrics.WithLabelValues(config.Source, "400").Inc()
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":          "invalid_json",
				"correlation_id": correlationID,
			})
			return
		}

		// Apply validation if configured
		if config.Validation != nil && config.Validation.Enabled {
			if err := s.validatePayload(payload, config.Validation); err != nil {
				logger.Warn("Dynamic webhook validation failed",
					logger.Fields{
						Component: "server",
						Operation: "dynamic_webhook",
						Source:    config.Source,
						Error:     err,
						Reason:    "validation_failed",
					})
				s.webhookMetrics.WithLabelValues(config.Source, "400").Inc()
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":          "validation_failed",
					"details":        err.Error(),
					"correlation_id": correlationID,
				})
				return
			}
		}

		// Process webhook using Ingester configuration
		if s.ingesterStore != nil && s.observationCreator != nil {
			// Get Ingester config for this source
			ingesterConfig, exists := s.ingesterStore.GetBySource(config.Source)
			if !exists {
				logger.Warn("Ingester config not found for source",
					logger.Fields{
						Component:     "server",
						Operation:     "dynamic_webhook",
						Source:        config.Source,
						CorrelationID: correlationID,
						Reason:        "ingester_config_not_found",
					})
				s.webhookMetrics.WithLabelValues(config.Source, "404").Inc()
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{
					"error":          "ingester_config_not_found",
					"correlation_id": correlationID,
				})
				return
			}

			// Create Observation from webhook payload using Ingester destination mapping
			observation := s.createObservationFromWebhookPayload(payload, ingesterConfig, config.Source, correlationID)
			if observation == nil {
				logger.Error("Failed to create observation from webhook payload",
					logger.Fields{
						Component:     "server",
						Operation:     "dynamic_webhook",
						Source:        config.Source,
						CorrelationID: correlationID,
						Reason:        "observation_creation_failed",
					})
				s.webhookMetrics.WithLabelValues(config.Source, "500").Inc()
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error":          "observation_creation_failed",
					"correlation_id": correlationID,
				})
				return
			}

			// Create Observation CRD
			ctx := r.Context()
			if err := s.observationCreator.CreateObservation(ctx, observation); err != nil {
				logger.Error("Failed to create Observation CRD",
					logger.Fields{
						Component:     "server",
						Operation:     "dynamic_webhook",
						Source:        config.Source,
						CorrelationID: correlationID,
						Error:         err,
					})
				s.webhookMetrics.WithLabelValues(config.Source, "500").Inc()
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error":          "observation_crd_creation_failed",
					"correlation_id": correlationID,
				})
				return
			}

			logger.Info("Dynamic webhook processed and Observation created",
				logger.Fields{
					Component:     "server",
					Operation:     "dynamic_webhook",
					Source:        config.Source,
					CorrelationID: correlationID,
				})
			s.webhookMetrics.WithLabelValues(config.Source, "200").Inc()
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":         "ok",
				"correlation_id": correlationID,
			})
			return
		}

		// Fallback: Send to channel if Ingester processing not available
		select {
		case s.falcoAlertsChan <- payload:
			logger.Info("Dynamic webhook received and queued (fallback mode)",
				logger.Fields{
					Component:     "server",
					Operation:     "dynamic_webhook",
					Source:        config.Source,
					CorrelationID: correlationID,
				})
			s.webhookMetrics.WithLabelValues(config.Source, "200").Inc()
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":         "ok",
				"correlation_id": correlationID,
			})
		default:
			logger.Error("Dynamic webhook channel full, dropping",
				logger.Fields{
					Component:     "server",
					Operation:     "dynamic_webhook",
					Source:        config.Source,
					CorrelationID: correlationID,
					Reason:        "channel_full",
				})
			s.webhookMetrics.WithLabelValues(config.Source, "503").Inc()
			if s.webhookDropped != nil {
				s.webhookDropped.WithLabelValues(config.Source).Inc()
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error":          "service_unavailable",
				"correlation_id": correlationID,
			})
		}
	}
}

// createAuthMiddleware creates authentication middleware for a dynamic endpoint
func (s *Server) createAuthMiddleware(next http.HandlerFunc, authConfig *webhook.AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authConfig.Enabled || authConfig.Type == "none" {
			next(w, r)
			return
		}

		var token string
		if authConfig.HeaderName != "" {
			token = r.Header.Get(authConfig.HeaderName)
		}

		valid := false
		switch authConfig.Type {
		case "apiKey":
			valid = token == authConfig.APIKey
		case "bearer":
			// Bearer token validation would go here
			// For now, simple comparison
			valid = token != "" && token == authConfig.Secret
		default:
			valid = false
		}

		if !valid {
			logger.Warn("Dynamic webhook authentication failed",
				logger.Fields{
					Component: "server",
					Operation: "dynamic_webhook_auth",
					Reason:    "authentication_failed",
				})
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "unauthorized",
			})
			return
		}

		next(w, r)
	}
}

// createRateLimitMiddleware creates rate limiting middleware for a dynamic endpoint
func (s *Server) createRateLimitMiddleware(next http.HandlerFunc, rateLimitConfig *webhook.RateLimitConfig) http.HandlerFunc {
	if !rateLimitConfig.Enabled {
		return next
	}

	// Create a per-endpoint rate limiter
	limiter := NewRateLimiter(rateLimitConfig.RPM, 1*time.Minute)
	if rateLimitConfig.Burst > 0 {
		// Adjust burst if specified
		limiter = NewRateLimiter(rateLimitConfig.RPM, 1*time.Minute)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		if !limiter.Allow(clientIP) {
			logger.Warn("Dynamic webhook rate limit exceeded",
				logger.Fields{
					Component: "server",
					Operation: "dynamic_webhook_rate_limit",
					Reason:    "rate_limit_exceeded",
					Additional: map[string]interface{}{
						"remote_addr": r.RemoteAddr,
					},
				})
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "rate_limit_exceeded",
			})
			return
		}
		next(w, r)
	}
}


// validatePayload validates a webhook payload against validation config
func (s *Server) validatePayload(payload map[string]interface{}, validation *webhook.ValidationConfig) error {
	// Check required fields
	if len(validation.Required) > 0 {
		for _, field := range validation.Required {
			if _, exists := payload[field]; !exists {
				return fmt.Errorf("required field missing: %s", field)
			}
		}
	}

	// Apply custom validation rules
	for _, rule := range validation.CustomRules {
		value, exists := payload[rule.Field]
		if rule.Required && !exists {
			return fmt.Errorf("required field missing: %s", rule.Field)
		}
		if exists {
			// Type checking
			if rule.Type != "" {
				if !s.checkType(value, rule.Type) {
					return fmt.Errorf("field %s has wrong type, expected %s", rule.Field, rule.Type)
				}
			}
			// Pattern matching for strings
			if rule.Pattern != "" {
				if str, ok := value.(string); ok {
					matched, _ := regexp.MatchString(rule.Pattern, str)
					if !matched {
						return fmt.Errorf("field %s does not match pattern", rule.Field)
					}
				}
			}
		}
	}

	return nil
}

// checkType checks if a value matches the expected type
func (s *Server) checkType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok || value == nil
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// generateCorrelationID generates a correlation ID for a request
func generateCorrelationID(r *http.Request) string {
	// Try to get from header first
	if id := r.Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	// Generate new one
	return fmt.Sprintf("webhook-%d", time.Now().UnixNano())
}

// GetEndpointRegistry returns the endpoint registry (for use by controllers)
func (s *Server) GetEndpointRegistry() *webhook.DynamicEndpointRegistry {
	return s.endpointRegistry
}

// RegisterDynamicEndpoint registers a dynamic endpoint
// This function is idempotent - calling it multiple times with the same source
// will only register the endpoint once.
func (s *Server) RegisterDynamicEndpoint(config *webhook.EndpointConfig) error {
	if config == nil {
		return fmt.Errorf("endpoint config cannot be nil")
	}

	// Validate required fields
	if config.Source == "" {
		return fmt.Errorf("endpoint source cannot be empty")
	}
	if config.Path == "" {
		return fmt.Errorf("endpoint path cannot be empty")
	}

	// Check if endpoint is already registered (with lock to prevent race conditions)
	s.dynamicMuxMu.Lock()
	
	// Check if path is already registered in ServeMux
	// IMPORTANT: Check this FIRST before any other operations
	pathAlreadyRegistered := s.registeredPaths[config.Path]
	
	// Always log to verify the function is being called
	logger.Info("RegisterDynamicEndpoint: checking registration",
		logger.Fields{
			Component: "server",
			Operation: "dynamic_endpoint_register_check",
			Additional: map[string]interface{}{
				"source":             config.Source,
				"path":               config.Path,
				"path_registered":    pathAlreadyRegistered,
				"registered_paths_count": len(s.registeredPaths),
			},
		})
	
	if pathAlreadyRegistered {
		// Path already registered - check if it's the same source
		existingConfig, exists := s.endpointRegistry.GetEndpoint(config.Source)
		if exists && existingConfig.Path == config.Path {
			// Same source and path - already registered, skip
			s.dynamicMuxMu.Unlock()
			logger.Info("Endpoint already registered, skipping",
				logger.Fields{
					Component: "server",
					Operation: "dynamic_endpoint_register_skip",
					Additional: map[string]interface{}{
						"source": config.Source,
						"path":   config.Path,
					},
				})
			return nil
		}
		// Path registered but different source - this is an error
		s.dynamicMuxMu.Unlock()
		logger.Warn("Path already registered for different source",
			logger.Fields{
				Component: "server",
				Operation: "dynamic_endpoint_register_conflict",
				Additional: map[string]interface{}{
					"path":   config.Path,
					"source": config.Source,
				},
			})
		return fmt.Errorf("path %s already registered for a different source", config.Path)
	}
	
	// Check if source is already registered with a different path
	existingConfig, exists := s.endpointRegistry.GetEndpoint(config.Source)
	if exists {
		if existingConfig.Path != config.Path {
			// Different path for same source - this is an error
			s.dynamicMuxMu.Unlock()
			return fmt.Errorf("endpoint for source %s already registered with different path: %s vs %s", config.Source, existingConfig.Path, config.Path)
		}
		// Same source and path - should have been caught by path check above, but handle it anyway
		s.dynamicMuxMu.Unlock()
		return nil
	}

	// Register in registry first
	if err := s.endpointRegistry.RegisterEndpoint(config); err != nil {
		s.dynamicMuxMu.Unlock()
		return err
	}

	// Create handler for this endpoint
	handler := s.createDynamicWebhookHandler(config)

	// Apply middleware stack
	var wrappedHandler http.HandlerFunc = handler
	if config.Auth != nil && config.Auth.Enabled {
		wrappedHandler = s.createAuthMiddleware(wrappedHandler, config.Auth)
	}
	if config.RateLimit != nil && config.RateLimit.Enabled {
		wrappedHandler = s.createRateLimitMiddleware(wrappedHandler, config.RateLimit)
	}

	// Mark path as registered BEFORE calling HandleFunc to prevent race conditions
	// This ensures that if another goroutine tries to register the same path,
	// it will see it as already registered
	s.registeredPaths[config.Path] = true
	
	// Register handler in ServeMux (still holding lock)
	// Use recover to catch any panic from ServeMux and provide better error message
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic while registering endpoint in ServeMux",
				logger.Fields{
					Component: "server",
					Operation: "dynamic_endpoint_register_panic",
					Error:     fmt.Errorf("%v", r),
					Additional: map[string]interface{}{
						"source":             config.Source,
						"path":               config.Path,
						"path_registered":    s.registeredPaths[config.Path],
						"registered_paths_count": len(s.registeredPaths),
					},
				})
			panic(r) // Re-panic to maintain original behavior
		}
	}()
	
	s.dynamicMux.HandleFunc(config.Path, wrappedHandler)
	s.dynamicMuxMu.Unlock()

	logger.Info("Dynamic endpoint registered",
		logger.Fields{
			Component: "server",
			Operation: "dynamic_endpoint_registered",
			Additional: map[string]interface{}{
				"source": config.Source,
				"path":   config.Path,
			},
		})

	return nil
}

// UnregisterDynamicEndpoint removes a dynamic endpoint
func (s *Server) UnregisterDynamicEndpoint(source string) error {
	config, exists := s.endpointRegistry.GetEndpoint(source)
	if !exists {
		return fmt.Errorf("endpoint for source %s not found", source)
	}

	if err := s.endpointRegistry.UnregisterEndpoint(source); err != nil {
		return err
	}

	// Note: We can't easily remove handlers from http.ServeMux
	// The handler will remain but will return 404 since it's not in the registry
	// This is acceptable for now - endpoints are typically long-lived

	logger.Info("Dynamic endpoint unregistered",
		logger.Fields{
			Component: "server",
			Operation: "dynamic_endpoint_unregistered",
			Additional: map[string]interface{}{
				"source": source,
				"path":   config.Path,
			},
		})

	return nil
}

// handleListEndpoints lists all registered dynamic endpoints
func (s *Server) handleListEndpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	endpoints := s.endpointRegistry.GetAllEndpoints()
	response := make(map[string]interface{})
	for source, config := range endpoints {
		response[source] = map[string]interface{}{
			"path":    config.Path,
			"methods": config.Methods,
			"auth":    config.Auth != nil && config.Auth.Enabled,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleFalcoWebhook handles POST /falco/webhook
func (s *Server) handleFalcoWebhook(w http.ResponseWriter, r *http.Request) {
	// Log webhook request received (before any processing)
	logger.Info("Falco webhook request received",
		logger.Fields{
			Component: "server",
			Operation: "falco_webhook",
			Source:    "falco",
			EventType: "webhook_request",
			Additional: map[string]interface{}{
				"method":      r.Method,
				"remote_addr": r.RemoteAddr,
				"user_agent":  r.UserAgent(),
			},
		})

	if r.Method != http.MethodPost {
		logger.Warn("Falco webhook rejected: invalid method",
			logger.Fields{
				Component: "server",
				Operation: "falco_webhook",
				Source:    "falco",
				Reason:    "invalid_method",
				Additional: map[string]interface{}{
					"method": r.Method,
				},
			})
		s.webhookMetrics.WithLabelValues("falco", "405").Inc()
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var alert map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		logger.Warn("Failed to parse Falco alert",
			logger.Fields{
				Component: "server",
				Operation: "falco_webhook",
				Source:    "falco",
				Error:     err,
				Reason:    "parse_error",
			})
		s.webhookMetrics.WithLabelValues("falco", "400").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rule, _ := alert["rule"].(string)
	correlationID := logger.GetCorrelationID(r.Context())
	if correlationID == "" {
		correlationID = fmt.Sprintf("falco-%d", time.Now().UnixNano())
		ctx := logger.WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)
	}

	// Send to channel for processing (non-blocking)
	select {
	case s.falcoAlertsChan <- alert:
		logger.Info("Falco webhook received and queued for processing",
			logger.Fields{
				Component:     "server",
				Operation:     "falco_webhook",
				Source:        "falco",
				EventType:     "webhook_queued",
				CorrelationID: correlationID,
				Additional: map[string]interface{}{
					"rule":     rule,
					"priority": fmt.Sprintf("%v", alert["priority"]),
				},
			})
		s.webhookMetrics.WithLabelValues("falco", "200").Inc()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			logger.Warn("Failed to write response",
				logger.Fields{
					Component:     "server",
					Operation:     "falco_webhook",
					Source:        "falco",
					CorrelationID: correlationID,
					Error:         err,
				})
		}
	default:
		logger.Error("Falco alerts channel full, dropping alert",
			logger.Fields{
				Component:     "server",
				Operation:     "falco_webhook",
				Source:        "falco",
				EventType:     "channel_full",
				CorrelationID: correlationID,
				Reason:        "channel_buffer_full",
				Additional: map[string]interface{}{
					"rule":     rule,
					"priority": fmt.Sprintf("%v", alert["priority"]),
				},
			})
		s.webhookMetrics.WithLabelValues("falco", "503").Inc()
		if s.webhookDropped != nil {
			s.webhookDropped.WithLabelValues("falco").Inc()
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// handleAuditWebhook handles POST /audit/webhook
func (s *Server) handleAuditWebhook(w http.ResponseWriter, r *http.Request) {
	// Log webhook request received (before any processing)
	logger.Info("Audit webhook request received",
		logger.Fields{
			Component: "server",
			Operation: "audit_webhook",
			Source:    "audit",
			EventType: "webhook_request",
			Additional: map[string]interface{}{
				"method":      r.Method,
				"remote_addr": r.RemoteAddr,
				"user_agent":  r.UserAgent(),
			},
		})

	if r.Method != http.MethodPost {
		logger.Warn("Audit webhook rejected: invalid method",
			logger.Fields{
				Component: "server",
				Operation: "audit_webhook",
				Source:    "audit",
				Reason:    "invalid_method",
				Additional: map[string]interface{}{
					"method": r.Method,
				},
			})
		s.webhookMetrics.WithLabelValues("audit", "405").Inc()
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var auditEvent map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&auditEvent); err != nil {
		logger.Warn("Failed to parse audit event",
			logger.Fields{
				Component: "server",
				Operation: "audit_webhook",
				Source:    "audit",
				Error:     err,
				Reason:    "parse_error",
			})
		s.webhookMetrics.WithLabelValues("audit", "400").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	auditID := fmt.Sprintf("%v", auditEvent["auditID"])
	verb := fmt.Sprintf("%v", auditEvent["verb"])
	objectRef, _ := auditEvent["objectRef"].(map[string]interface{})
	resource := fmt.Sprintf("%v", objectRef["resource"])
	correlationID := logger.GetCorrelationID(r.Context())
	if correlationID == "" {
		correlationID = fmt.Sprintf("audit-%s", auditID)
		ctx := logger.WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)
	}

	// Send to channel for processing (non-blocking)
	select {
	case s.auditEventsChan <- auditEvent:
		logger.Info("Audit webhook received and queued for processing",
			logger.Fields{
				Component:     "server",
				Operation:     "audit_webhook",
				Source:        "audit",
				EventType:     "webhook_queued",
				CorrelationID: correlationID,
				Additional: map[string]interface{}{
					"audit_id": auditID,
					"verb":     verb,
					"resource": resource,
					"stage":    fmt.Sprintf("%v", auditEvent["stage"]),
				},
			})
		s.webhookMetrics.WithLabelValues("audit", "200").Inc()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			logger.Warn("Failed to write response",
				logger.Fields{
					Component:     "server",
					Operation:     "audit_webhook",
					Source:        "audit",
					CorrelationID: correlationID,
					Error:         err,
				})
		}
	default:
		logger.Error("Audit events channel full, dropping event",
			logger.Fields{
				Component:     "server",
				Operation:     "audit_webhook",
				Source:        "audit",
				EventType:     "channel_full",
				CorrelationID: correlationID,
				Reason:        "channel_buffer_full",
				Additional: map[string]interface{}{
					"audit_id": auditID,
					"verb":     verb,
					"resource": resource,
				},
			})
		s.webhookMetrics.WithLabelValues("audit", "503").Inc()
		if s.webhookDropped != nil {
			s.webhookDropped.WithLabelValues("audit").Inc()
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// Start starts the HTTP server in a goroutine
func (s *Server) Start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		endpoints := []string{
			"/health",
			"/ready",
			"/metrics",
			"/falco/webhook",
			"/audit/webhook",
		}
		if s.isPprofEnabled() {
			endpoints = append(endpoints,
				"/debug/pprof/",
				"/debug/pprof/profile",
				"/debug/pprof/heap",
				"/debug/pprof/allocs",
			)
		}

		logger.Info("HTTP server starting",
			logger.Fields{
				Component: "server",
				Operation: "http_start",
				Additional: map[string]interface{}{
					"address":   s.server.Addr,
					"endpoints": endpoints,
					"pprof":     s.isPprofEnabled(),
				},
			})

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error",
				logger.Fields{
					Component: "server",
					Operation: "http_serve",
					Error:     err,
				})
		}
	}()

	// Graceful shutdown handler
	go func() {
		<-ctx.Done()
		logger.Info("Shutting down HTTP server",
			logger.Fields{
				Component: "server",
				Operation: "http_shutdown",
			})

		// Get shutdown timeout from env var, default to 10 seconds
		shutdownTimeout := 10 * time.Second
		if timeoutStr := os.Getenv("HTTP_SHUTDOWN_TIMEOUT"); timeoutStr != "" {
			if parsed, err := time.ParseDuration(timeoutStr); err == nil {
				shutdownTimeout = parsed
			} else {
				logger.Warn("Invalid HTTP_SHUTDOWN_TIMEOUT, using default",
					logger.Fields{
						Component: "server",
						Operation: "http_shutdown",
						Additional: map[string]interface{}{
							"invalid_value": timeoutStr,
							"default":       "10s",
						},
					})
			}
		}

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown error",
				logger.Fields{
					Component: "server",
					Operation: "http_shutdown",
					Error:     err,
				})
		} else {
			logger.Info("HTTP server shut down gracefully",
				logger.Fields{
					Component: "server",
					Operation: "http_shutdown",
					Duration:  shutdownTimeout.String(),
				})
		}
	}()
}

// SetReady sets the readiness status
func (s *Server) SetReady(ready bool) {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	s.ready = ready
}

// handleHAHealth handles GET /ha/health - HA health check endpoint
func (s *Server) handleHAHealth(w http.ResponseWriter, r *http.Request) {
	s.haStatusMu.RLock()
	status := s.haStatus
	s.haStatusMu.RUnlock()

	if status == nil || !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "unhealthy",
			"replica_id": status.ReplicaID,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "healthy",
		"replica_id": status.ReplicaID,
		"enabled":    status.Enabled,
	})
}

// handleHAMetrics handles GET /ha/metrics - HA-specific metrics for auto-scaling
func (s *Server) handleHAMetrics(w http.ResponseWriter, r *http.Request) {
	s.haStatusMu.RLock()
	status := s.haStatus
	s.haStatusMu.RUnlock()

	if status == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"replica_id":     status.ReplicaID,
		"cpu_usage":      status.CPUUsage,
		"memory_usage":   status.MemoryUsage,
		"events_per_sec": status.EventsPerSec,
		"queue_depth":    status.QueueDepth,
		"current_load":   status.CurrentLoad,
		"timestamp":      time.Now().Format(time.RFC3339),
	})
}

// handleHAStatus handles GET /ha/status - Current HA status and load
func (s *Server) handleHAStatus(w http.ResponseWriter, r *http.Request) {
	s.haStatusMu.RLock()
	status := s.haStatus
	s.haStatusMu.RUnlock()

	if status == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// UpdateHAStatus updates the HA status with current metrics
func (s *Server) UpdateHAStatus(cpuUsage, memoryUsage, eventsPerSec, currentLoad float64, queueDepth int) {
	s.haStatusMu.Lock()
	defer s.haStatusMu.Unlock()

	if s.haStatus != nil {
		s.haStatus.CPUUsage = cpuUsage
		s.haStatus.MemoryUsage = memoryUsage
		s.haStatus.EventsPerSec = eventsPerSec
		s.haStatus.CurrentLoad = currentLoad
		s.haStatus.QueueDepth = queueDepth
		s.haStatus.LastUpdate = time.Now().Format(time.RFC3339)
		s.haStatus.Healthy = true
	}
}
