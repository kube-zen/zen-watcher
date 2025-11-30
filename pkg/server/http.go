package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	rateLimiter     *RateLimiter
}

// NewServer creates a new HTTP server with handlers
func NewServer(falcoChan, auditChan chan map[string]interface{}, webhookMetrics, webhookDropped *prometheus.CounterVec) *Server {
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
	}

	s.registerHandlers(mux)
	return s
}

// parseEnvInt parses an environment variable as an integer
func parseEnvInt(s string) (int, error) {
	return strconv.Atoi(s)
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

	// Falco webhook handler (with authentication and rate limiting)
	falcoHandler := s.auth.RequireAuth(s.handleFalcoWebhook)
	mux.HandleFunc("/falco/webhook", s.rateLimiter.RateLimitMiddleware(falcoHandler))

	// Audit webhook handler (with authentication and rate limiting)
	auditHandler := s.auth.RequireAuth(s.handleAuditWebhook)
	mux.HandleFunc("/audit/webhook", s.rateLimiter.RateLimitMiddleware(auditHandler))
}

// handleFalcoWebhook handles POST /falco/webhook
func (s *Server) handleFalcoWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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
		logger.Info("Falco webhook received and queued",
			logger.Fields{
				Component:     "server",
				Operation:     "falco_webhook",
				Source:        "falco",
				EventType:     "webhook_received",
				CorrelationID: correlationID,
				Additional: map[string]interface{}{
					"rule": rule,
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
		logger.Warn("Falco alerts channel full, dropping alert",
			logger.Fields{
				Component:     "server",
				Operation:     "falco_webhook",
				Source:        "falco",
				EventType:     "channel_full",
				CorrelationID: correlationID,
				Reason:        "channel_buffer_full",
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
	if r.Method != http.MethodPost {
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
			})
		s.webhookMetrics.WithLabelValues("audit", "400").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	auditID := fmt.Sprintf("%v", auditEvent["auditID"])
	correlationID := logger.GetCorrelationID(r.Context())
	if correlationID == "" {
		correlationID = fmt.Sprintf("audit-%s", auditID)
		ctx := logger.WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)
	}

	// Send to channel for processing (non-blocking)
	select {
	case s.auditEventsChan <- auditEvent:
		logger.Info("Audit webhook received and queued",
			logger.Fields{
				Component:     "server",
				Operation:     "audit_webhook",
				Source:        "audit",
				EventType:     "webhook_received",
				CorrelationID: correlationID,
				Additional: map[string]interface{}{
					"audit_id": auditID,
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
		logger.Warn("Audit events channel full, dropping event",
			logger.Fields{
				Component:     "server",
				Operation:     "audit_webhook",
				Source:        "audit",
				EventType:     "channel_full",
				CorrelationID: correlationID,
				Reason:        "channel_buffer_full",
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
		logger.Info("HTTP server starting",
			logger.Fields{
				Component: "server",
				Operation: "http_start",
				Additional: map[string]interface{}{
					"address": s.server.Addr,
					"endpoints": []string{
						"/health",
						"/ready",
						"/metrics",
						"/falco/webhook",
						"/audit/webhook",
					},
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
