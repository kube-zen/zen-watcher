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
	"strconv"
	"sync"
	"time"

	sdklifecycle "github.com/kube-zen/zen-sdk/pkg/lifecycle"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	sdkmetrics "github.com/kube-zen/zen-sdk/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Package-level logger to avoid repeated allocations
var serverLogger = sdklog.NewLogger("zen-watcher-server")

// Server wraps the HTTP server and handlers
type Server struct {
	server          *http.Server
	pprofServer     *http.Server // Separate server for pprof (localhost only)
	mux             *http.ServeMux
	muxMu           sync.RWMutex // Protects mux for dynamic route registration
	ready           bool
	readyMu         sync.RWMutex
	webhookMetrics  *prometheus.CounterVec
	webhookDropped  *prometheus.CounterVec
	auth            *WebhookAuth
	rateLimiter     *PerKeyRateLimiter
	haConfig        *config.HAConfig
	haStatus        *HAStatus
	haStatusMu      sync.RWMutex
	maxRequestBytes int64 // Maximum request body size (default: 1MiB)
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

// NewServer creates a new HTTP server
func NewServer(webhookMetrics, webhookDropped *prometheus.CounterVec) *Server {
	port := os.Getenv("WATCHER_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	auth := NewWebhookAuthWithMetrics(webhookMetrics)

	// Initialize HTTP metrics using zen-sdk/pkg/metrics
	httpMetrics, err := sdkmetrics.NewHTTPMetrics(sdkmetrics.HTTPMetricsConfig{
		Component: "zen-watcher",
		Prefix:    "http",
		Registry:  prometheus.DefaultRegisterer,
	})
	if err != nil {
		// Log warning but continue without HTTP metrics
		fmt.Fprintf(os.Stderr, "Failed to initialize HTTP metrics: %v\n", err)
		httpMetrics = nil
	}

	// Rate limiter: 100 requests per minute per IP (configurable)
	maxRequests := 100
	if maxReqStr := os.Getenv("WEBHOOK_RATE_LIMIT"); maxReqStr != "" {
		if parsed, err := parseEnvInt(maxReqStr); err == nil && parsed > 0 {
			maxRequests = parsed
		}
	}
	// Rate limit metrics (optional - can be nil if not provided)
	// The rate limiter will still track rejections via webhookMetrics (status="429")
	// Dedicated rate limit metrics can be added later if needed for more detailed observability
	var rateLimitMetrics *prometheus.CounterVec
	rateLimiter := NewPerKeyRateLimiterWithMetrics(maxRequests, 1*time.Minute, auth.GetTrustedProxyCIDRs(), webhookMetrics, rateLimitMetrics)

	// Load HA configuration
	haConfig := config.LoadHAConfig()
	replicaID := os.Getenv("HOSTNAME")
	if replicaID == "" {
		replicaID = fmt.Sprintf("replica-%d", time.Now().UnixNano())
	}

	// Load max request body size (default: 1MiB = 1048576 bytes)
	maxRequestBytes := int64(1048576) // 1MiB default
	if maxBytesStr := os.Getenv("SERVER_MAX_REQUEST_BYTES"); maxBytesStr != "" {
		if parsed, err := strconv.ParseInt(maxBytesStr, 10, 64); err == nil && parsed > 0 {
			maxRequestBytes = parsed
		} else {
			serverLogger.Warn("Invalid SERVER_MAX_REQUEST_BYTES, using default",
				sdklog.Operation("server_init"),
				sdklog.ErrorCode("CONFIG_ERROR"),
				sdklog.String("invalid_value", maxBytesStr),
				sdklog.Int64("default", maxRequestBytes))
		}
	}

	s := &Server{
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      mux, // Will be wrapped with metrics middleware after handlers are registered
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		mux:            mux,
		webhookMetrics: webhookMetrics,
		webhookDropped: webhookDropped,
		auth:           auth,
		rateLimiter:    rateLimiter,
		haConfig:       haConfig,
		haStatus: &HAStatus{
			Enabled:    haConfig.IsHAEnabled(),
			ReplicaID:  replicaID,
			Healthy:    true,
			LastUpdate: time.Now().Format(time.RFC3339),
		},
		maxRequestBytes: maxRequestBytes,
	}

	// Register core handlers (health, metrics, HA endpoints)
	s.registerHandlers(mux)

	// Wrap mux with HTTP metrics middleware if available (after handlers are registered)
	if httpMetrics != nil {
		s.server.Handler = httpMetrics.Middleware("zen-watcher")(mux)
	}

	// Initialize pprof server if enabled (localhost only for security)
	if s.isPprofEnabled() {
		s.initPprofServer()
	}

	return s
}

// NewServerWithIngester creates a new HTTP server (kept for backward compatibility)
func NewServerWithIngester(
	falcoChan, auditChan chan map[string]interface{},
	webhookMetrics, webhookDropped *prometheus.CounterVec,
	ingesterStore *config.IngesterStore,
	observationCreator interface {
		CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error
	},
) *Server {
	// Note: Legacy parameters kept for backward compatibility, but ignored
	// All webhook handling is now done via generic webhook adapter
	return NewServer(webhookMetrics, webhookDropped)
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

// initPprofServer initializes a separate HTTP server for pprof endpoints bound to localhost only
func (s *Server) initPprofServer() {
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6060" // Default pprof port
	}

	pprofMux := http.NewServeMux()
	// Register all pprof endpoints
	pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
	pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	// Additional pprof endpoints
	pprofMux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	pprofMux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	pprofMux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	pprofMux.Handle("/debug/pprof/block", pprof.Handler("block"))
	pprofMux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))

	s.pprofServer = &http.Server{
		Addr:         "127.0.0.1:" + pprofPort, // Bind to localhost only for security
		Handler:      pprofMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second, // Longer timeout for CPU profiling
		IdleTimeout:  60 * time.Second,
	}
}

// registerHandlers registers all HTTP handlers
func (s *Server) registerHandlers(mux *http.ServeMux) {
	// Health check endpoint (Kubernetes standard: /healthz)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "alive",
			"service":   "zen-watcher",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			serverLogger.Warn("Failed to encode healthz response",
				sdklog.Operation("healthz"),
				sdklog.ErrorCode("ENCODE_ERROR"),
				sdklog.Error(err))
		}
	})

	// Legacy /health endpoint (kept for backward compatibility)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "healthy")
	})

	// Readiness probe endpoint (Kubernetes standard: /readyz)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		s.readyMu.RLock()
		ready := s.ready
		s.readyMu.RUnlock()

		if ready {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ready",
			}); err != nil {
				serverLogger.Warn("Failed to encode readyz response",
					sdklog.Operation("readyz"),
					sdklog.ErrorCode("ENCODE_ERROR"),
					sdklog.Error(err))
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "not_ready",
			}); err != nil {
				serverLogger.Warn("Failed to encode readyz response",
					sdklog.Operation("readyz"),
					sdklog.ErrorCode("ENCODE_ERROR"),
					sdklog.Error(err))
			}
		}
	})

	// Legacy /ready endpoint (kept for backward compatibility)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		s.readyMu.RLock()
		ready := s.ready
		s.readyMu.RUnlock()

		if ready {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "ready")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "not ready")
		}
	})

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// pprof endpoints are registered on a separate localhost-only server (see startPprofServer)

	// HA-aware endpoints (only if HA is enabled)
	if s.haConfig != nil && s.haConfig.IsHAEnabled() {
		// HA health check endpoint
		mux.HandleFunc("/ha/health", s.handleHAHealth)

		// HA metrics endpoint
		mux.HandleFunc("/ha/metrics", s.handleHAMetrics)

		// HA status endpoint
		mux.HandleFunc("/ha/status", s.handleHAStatus)
	}

	// Webhook endpoints are now registered dynamically via RegisterWebhookHandler()
	// No hardcoded Falco/Audit handlers - all webhooks are handled generically via Ingester CRDs
}

// RegisterWebhookHandler registers a webhook handler on the main server mux dynamically
// This allows generic webhook adapters to register routes based on Ingester CRDs
func (s *Server) RegisterWebhookHandler(path string, handler http.HandlerFunc) error {
	if path == "" {
		return fmt.Errorf("webhook path cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("webhook handler cannot be nil")
	}

	s.muxMu.Lock()
	defer s.muxMu.Unlock()

	if s.mux == nil {
		return fmt.Errorf("server mux not initialized")
	}

	// Apply authentication and rate limiting middleware
	wrappedHandler := s.auth.RequireAuth(handler)
	wrappedHandler = s.rateLimiter.RateLimitMiddleware(wrappedHandler)

	s.mux.HandleFunc(path, wrappedHandler)

	serverLogger.Info("Webhook handler registered dynamically",
		sdklog.Operation("register_webhook"),
		sdklog.String("path", path))

	return nil
}

// UnregisterWebhookHandler removes a webhook handler from the main server mux
func (s *Server) UnregisterWebhookHandler(path string) error {
	if path == "" {
		return fmt.Errorf("webhook path cannot be empty")
	}

	s.muxMu.Lock()
	defer s.muxMu.Unlock()

	if s.mux == nil {
		return fmt.Errorf("server mux not initialized")
	}

	// Note: http.ServeMux doesn't have an Unregister method
	// We'll need to create a wrapper mux that supports dynamic registration/unregistration
	// For now, this is a placeholder - handlers are only added, not removed
	// In practice, restarting the pod with updated Ingester CRDs handles this

	serverLogger.Info("Webhook handler unregistered",
		sdklog.Operation("unregister_webhook"),
		sdklog.String("path", path))

	return nil
}

// GetMux returns the server's mux for direct handler registration (advanced use)
func (s *Server) GetMux() *http.ServeMux {
	s.muxMu.RLock()
	defer s.muxMu.RUnlock()
	return s.mux
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
			// Webhook endpoints are registered dynamically via RegisterWebhookHandler()
		}

		serverLogger.Info("HTTP server starting",
			sdklog.Operation("http_start"),
			sdklog.String("address", s.server.Addr),
			sdklog.Strings("endpoints", endpoints),
			sdklog.Bool("pprof", s.isPprofEnabled()))

		if s.isPprofEnabled() && s.pprofServer != nil {
			serverLogger.Info("pprof server starting (localhost only)",
				sdklog.Operation("pprof_start"),
				sdklog.String("address", s.pprofServer.Addr))
		}

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverLogger.Error(err, "HTTP server error",
				sdklog.Operation("http_serve"),
				sdklog.ErrorCode("HTTP_SERVER_ERROR"))
		}
	}()

	// Start pprof server if enabled (separate goroutine, localhost only)
	if s.isPprofEnabled() && s.pprofServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverLogger.Error(err, "pprof server error",
					sdklog.Operation("pprof_serve"),
					sdklog.ErrorCode("PPROF_SERVER_ERROR"))
			}
		}()
	}

	// Graceful shutdown handler
	go func() {
		<-ctx.Done()
		// Get shutdown timeout from env var, default to 10 seconds
		shutdownTimeout := 10 * time.Second
		if timeoutStr := os.Getenv("HTTP_SHUTDOWN_TIMEOUT"); timeoutStr != "" {
			if parsed, err := time.ParseDuration(timeoutStr); err == nil {
				shutdownTimeout = parsed
			} else {
				serverLogger.Warn("Invalid HTTP_SHUTDOWN_TIMEOUT, using default",
					sdklog.Operation("http_shutdown"),
					sdklog.ErrorCode("CONFIG_ERROR"),
					sdklog.String("invalid_value", timeoutStr),
					sdklog.String("default", "10s"))
			}
		}

		// Use zen-sdk lifecycle for graceful shutdown
		if err := sdklifecycle.ShutdownHTTPServer(ctx, s.server, "zen-watcher-server", shutdownTimeout); err != nil {
			serverLogger.Error(err, "HTTP server shutdown error",
				sdklog.Operation("http_shutdown"),
				sdklog.ErrorCode("HTTP_SHUTDOWN_ERROR"))
		}

		// Shutdown pprof server if enabled
		if s.pprofServer != nil {
			if err := sdklifecycle.ShutdownHTTPServer(ctx, s.pprofServer, "pprof-server", shutdownTimeout); err != nil {
				serverLogger.Error(err, "pprof server shutdown error",
					sdklog.Operation("pprof_shutdown"),
					sdklog.ErrorCode("PPROF_SHUTDOWN_ERROR"))
			}
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
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "unhealthy",
			"replica_id": status.ReplicaID,
		}); err != nil {
			serverLogger.Warn("Failed to encode HA health response",
				sdklog.Operation("ha_health"),
				sdklog.ErrorCode("ENCODE_ERROR"),
				sdklog.Error(err))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "healthy",
		"replica_id": status.ReplicaID,
		"enabled":    status.Enabled,
	}); err != nil {
		serverLogger.Warn("Failed to encode HA health response",
			sdklog.Operation("ha_health"),
			sdklog.ErrorCode("ENCODE_ERROR"),
			sdklog.Error(err))
	}
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
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"replica_id":     status.ReplicaID,
		"cpu_usage":      status.CPUUsage,
		"memory_usage":   status.MemoryUsage,
		"events_per_sec": status.EventsPerSec,
		"queue_depth":    status.QueueDepth,
		"current_load":   status.CurrentLoad,
		"timestamp":      time.Now().Format(time.RFC3339),
	}); err != nil {
		serverLogger.Warn("Failed to encode HA metrics response",
			sdklog.Operation("ha_metrics"),
			sdklog.ErrorCode("ENCODE_ERROR"),
			sdklog.Error(err))
	}
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
	if err := json.NewEncoder(w).Encode(status); err != nil {
		serverLogger.Warn("Failed to encode HA status response",
			sdklog.Operation("ha_status"),
			sdklog.ErrorCode("ENCODE_ERROR"),
			sdklog.Error(err))
	}
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
