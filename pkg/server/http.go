package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

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
}

// NewServer creates a new HTTP server with handlers
func NewServer(falcoChan, auditChan chan map[string]interface{}, webhookMetrics, webhookDropped *prometheus.CounterVec) *Server {
	port := os.Getenv("WATCHER_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
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
	}

	s.registerHandlers(mux)
	return s
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

	// Falco webhook handler
	mux.HandleFunc("/falco/webhook", s.handleFalcoWebhook)

	// Audit webhook handler
	mux.HandleFunc("/audit/webhook", s.handleAuditWebhook)
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
		log.Printf("⚠️  Failed to parse Falco alert: %v", err)
		s.webhookMetrics.WithLabelValues("falco", "400").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Send to channel for processing (non-blocking)
	select {
	case s.falcoAlertsChan <- alert:
		log.Printf("  ✅ [FALCO] Webhook received and queued for processing: %v", alert["rule"])
		s.webhookMetrics.WithLabelValues("falco", "200").Inc()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			log.Printf("⚠️  Failed to write response: %v", err)
		}
	default:
		log.Println("⚠️  Falco alerts channel full, dropping alert")
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
		log.Printf("⚠️  Failed to parse audit event: %v", err)
		s.webhookMetrics.WithLabelValues("audit", "400").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Send to channel for processing (non-blocking)
	select {
	case s.auditEventsChan <- auditEvent:
		auditID := fmt.Sprintf("%v", auditEvent["auditID"])
		log.Printf("  ✅ [AUDIT] Webhook received and queued for processing: auditID=%s", auditID)
		s.webhookMetrics.WithLabelValues("audit", "200").Inc()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			log.Printf("⚠️  Failed to write response: %v", err)
		}
	default:
		log.Println("⚠️  Audit events channel full, dropping event")
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
		log.Println("🌐 HTTP server starting on " + s.server.Addr)
		log.Println("   Endpoints:")
		log.Println("     - /health - Health check")
		log.Println("     - /ready - Readiness probe")
		log.Println("     - /metrics - Prometheus metrics")
		log.Println("     - /falco/webhook - Falco alerts")
		log.Println("     - /audit/webhook - Audit events")

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown handler
	go func() {
		<-ctx.Done()
		log.Println("🛑 Shutting down HTTP server...")

		// Get shutdown timeout from env var, default to 10 seconds
		shutdownTimeout := 10 * time.Second
		if timeoutStr := os.Getenv("HTTP_SHUTDOWN_TIMEOUT"); timeoutStr != "" {
			if parsed, err := time.ParseDuration(timeoutStr); err == nil {
				shutdownTimeout = parsed
			} else {
				log.Printf("⚠️  Invalid HTTP_SHUTDOWN_TIMEOUT value '%s', using default 10s", timeoutStr)
			}
		}

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("⚠️  HTTP server shutdown error: %v", err)
		} else {
			log.Println("✅ HTTP server shut down gracefully")
		}
	}()
}

// SetReady sets the readiness status
func (s *Server) SetReady(ready bool) {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	s.ready = ready
}
