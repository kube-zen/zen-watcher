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

	"github.com/kube-zen/zen-watcher/pkg/config"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Server wraps the HTTP server and handlers
type Server struct {
	server          *http.Server
	ready           bool
	readyMu         sync.RWMutex
	falcoAlertsChan chan map[string]interface{}
	auditEventsChan chan map[string]interface{}
	webhookMetrics  *prometheus.CounterVec
	webhookDropped  *prometheus.CounterVec
	auth            *WebhookAuth
	rateLimiter     *PerKeyRateLimiter
	haConfig        *config.HAConfig
	haStatus        *HAStatus
	haStatusMu      sync.RWMutex
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

// NewServerWithIngester creates a new HTTP server (kept for backward compatibility)
func NewServerWithIngester(
	falcoChan, auditChan chan map[string]interface{},
	webhookMetrics, webhookDropped *prometheus.CounterVec,
	ingesterStore *config.IngesterStore,
	observationCreator interface {
		CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error
	},
) *Server {
	// Note: ingesterStore and observationCreator parameters are kept for API compatibility
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
	rateLimiter := NewPerKeyRateLimiter(maxRequests, 1*time.Minute)

	// Load HA configuration
	haConfig := config.LoadHAConfig()
	replicaID := os.Getenv("HOSTNAME")
	if replicaID == "" {
		replicaID = fmt.Sprintf("replica-%d", time.Now().UnixNano())
	}

	s := &Server{
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		falcoAlertsChan: falcoChan,
		auditEventsChan: auditChan,
		webhookMetrics:  webhookMetrics,
		webhookDropped:  webhookDropped,
		auth:            auth,
		rateLimiter:     rateLimiter,
		haConfig:        haConfig,
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
}

// handleFalcoWebhook handles POST /falco/webhook
func (s *Server) handleFalcoWebhook(w http.ResponseWriter, r *http.Request) {
	// Log webhook request received (before any processing)
	logger := sdklog.NewLogger("zen-watcher-server")
	logger.InfoC(r.Context(), "Falco webhook request received",
		sdklog.Operation("falco_webhook"),
		sdklog.String("source", "falco"),
		sdklog.String("method", r.Method),
		sdklog.String("remote_addr", r.RemoteAddr),
		sdklog.String("user_agent", r.UserAgent()))

	if r.Method != http.MethodPost {
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.WarnC(r.Context(), "Falco webhook rejected: invalid method",
			sdklog.Operation("falco_webhook"),
			sdklog.String("source", "falco"),
			sdklog.String("reason", "invalid_method"),
			sdklog.String("method", r.Method))
		s.webhookMetrics.WithLabelValues("falco", "405").Inc()
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var alert map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.WarnC(r.Context(), "Failed to parse Falco alert",
			sdklog.Operation("falco_webhook"),
			sdklog.String("source", "falco"),
			sdklog.String("reason", "parse_error"),
			sdklog.Error(err))
		s.webhookMetrics.WithLabelValues("falco", "400").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rule, _ := alert["rule"].(string)

	// Send to channel for processing (non-blocking)
	select {
	case s.falcoAlertsChan <- alert:
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.InfoC(r.Context(), "Falco webhook received and queued for processing",
			sdklog.Operation("falco_webhook"),
			sdklog.String("source", "falco"),
			sdklog.String("rule", rule),
			sdklog.String("priority", fmt.Sprintf("%v", alert["priority"])))
		s.webhookMetrics.WithLabelValues("falco", "200").Inc()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.WarnC(r.Context(), "Failed to write response",
				sdklog.Operation("falco_webhook"),
				sdklog.String("source", "falco"),
				sdklog.Error(err))
		}
	default:
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.ErrorC(r.Context(), fmt.Errorf("channel buffer full"), "Falco alerts channel full, dropping alert",
			sdklog.Operation("falco_webhook"),
			sdklog.String("source", "falco"),
			sdklog.String("reason", "channel_buffer_full"),
			sdklog.String("rule", rule),
			sdklog.String("priority", fmt.Sprintf("%v", alert["priority"])))
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
	logger := sdklog.NewLogger("zen-watcher-server")
	logger.InfoC(r.Context(), "Audit webhook request received",
		sdklog.Operation("audit_webhook"),
		sdklog.String("source", "audit"),
		sdklog.String("method", r.Method),
		sdklog.String("remote_addr", r.RemoteAddr),
		sdklog.String("user_agent", r.UserAgent()))

	if r.Method != http.MethodPost {
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.WarnC(r.Context(), "Audit webhook rejected: invalid method",
			sdklog.Operation("audit_webhook"),
			sdklog.String("source", "audit"),
			sdklog.String("reason", "invalid_method"),
			sdklog.String("method", r.Method))
		s.webhookMetrics.WithLabelValues("audit", "405").Inc()
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var auditEvent map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&auditEvent); err != nil {
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.WarnC(r.Context(), "Failed to parse audit event",
			sdklog.Operation("audit_webhook"),
			sdklog.String("source", "audit"),
			sdklog.String("reason", "parse_error"),
			sdklog.Error(err))
		s.webhookMetrics.WithLabelValues("audit", "400").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	auditID := fmt.Sprintf("%v", auditEvent["auditID"])
	verb := fmt.Sprintf("%v", auditEvent["verb"])
	objectRef, _ := auditEvent["objectRef"].(map[string]interface{})
	resource := fmt.Sprintf("%v", objectRef["resource"])
	correlationID := sdklog.GetRequestID(r.Context())
	if correlationID == "" {
		correlationID = fmt.Sprintf("audit-%s", auditID)
		ctx := sdklog.WithRequestID(r.Context(), correlationID)
		r = r.WithContext(ctx)
	}

	// Send to channel for processing (non-blocking)
	select {
	case s.auditEventsChan <- auditEvent:
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.InfoC(r.Context(), "Audit webhook received and queued for processing",
			sdklog.Operation("audit_webhook"),
			sdklog.String("source", "audit"),
			sdklog.String("audit_id", auditID),
			sdklog.String("verb", verb),
			sdklog.String("resource", resource),
			sdklog.String("stage", fmt.Sprintf("%v", auditEvent["stage"])))
		s.webhookMetrics.WithLabelValues("audit", "200").Inc()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.WarnC(r.Context(), "Failed to write response",
				sdklog.Operation("audit_webhook"),
				sdklog.String("source", "audit"),
				sdklog.Error(err))
		}
	default:
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.ErrorC(r.Context(), fmt.Errorf("channel buffer full"), "Audit events channel full, dropping event",
			sdklog.Operation("audit_webhook"),
			sdklog.String("source", "audit"),
			sdklog.String("reason", "channel_buffer_full"),
			sdklog.String("audit_id", auditID),
			sdklog.String("verb", verb),
			sdklog.String("resource", resource))
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

		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Info("HTTP server starting",
			sdklog.Operation("http_start"),
			sdklog.String("address", s.server.Addr),
			sdklog.Strings("endpoints", endpoints),
			sdklog.Bool("pprof", s.isPprofEnabled()))

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Error(err, "HTTP server error",
				sdklog.Operation("http_serve"))
		}
	}()

	// Graceful shutdown handler
	go func() {
		<-ctx.Done()
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Info("Shutting down HTTP server",
			sdklog.Operation("http_shutdown"))

		// Get shutdown timeout from env var, default to 10 seconds
		shutdownTimeout := 10 * time.Second
		if timeoutStr := os.Getenv("HTTP_SHUTDOWN_TIMEOUT"); timeoutStr != "" {
			if parsed, err := time.ParseDuration(timeoutStr); err == nil {
				shutdownTimeout = parsed
			} else {
				logger := sdklog.NewLogger("zen-watcher-server")
				logger.Warn("Invalid HTTP_SHUTDOWN_TIMEOUT, using default",
					sdklog.Operation("http_shutdown"),
					sdklog.String("invalid_value", timeoutStr),
					sdklog.String("default", "10s"))
			}
		}

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Error(err, "HTTP server shutdown error",
				sdklog.Operation("http_shutdown"))
		} else {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Info("HTTP server shut down gracefully",
				sdklog.Operation("http_shutdown"),
				sdklog.Duration("duration", shutdownTimeout))
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
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Warn("Failed to encode HA health response",
				sdklog.Operation("ha_health"),
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
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Warn("Failed to encode HA health response",
			sdklog.Operation("ha_health"),
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
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Warn("Failed to encode HA metrics response",
			sdklog.Operation("ha_metrics"),
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
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Warn("Failed to encode HA status response",
			sdklog.Operation("ha_status"),
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
