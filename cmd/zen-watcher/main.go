package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/kube-zen/zen-watcher/internal/kubernetes"
	"github.com/kube-zen/zen-watcher/internal/lifecycle"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/gc"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/server"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
)

func main() {
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🚀 zen-watcher v1.0.22 (Go 1.24, Apache 2.0)")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Setup signal handling and context
	ctx, stopCh := lifecycle.SetupSignalHandler()

	// Initialize metrics
	m := metrics.NewMetrics()

	// Initialize Kubernetes clients
	clients, err := kubernetes.NewClients()
	if err != nil {
		log.Fatalf("❌ Failed to initialize Kubernetes clients: %v", err)
	}

	// Get GVRs
	gvrs := kubernetes.NewGVRs()

	// Create informer factory
	informerFactory := kubernetes.NewInformerFactory(clients.Dynamic)

	// Load filter configuration from ConfigMap (initial load)
	filterConfig, err := filter.LoadFilterConfig(clients.Standard)
	if err != nil {
		log.Printf("⚠️  Failed to load filter config: %v (continuing without filter)", err)
		filterConfig = &filter.FilterConfig{Sources: make(map[string]filter.SourceFilter)}
	}
	filterInstance := filter.NewFilter(filterConfig)

	// Create ConfigMap loader for dynamic reloading
	configMapLoader := config.NewConfigMapLoader(clients.Standard, filterInstance)

	// Create processors with centralized observation creator and filter
	// Flow: filter() → normalize() → dedup() → create Observation CRD + update metrics + log
	eventProcessor, webhookProcessor, observationCreator := watcher.NewProcessors(
		clients.Dynamic,
		gvrs.Observations,
		m.EventsTotal,
		m.ObservationsCreated,
		m.ObservationsFiltered,
		m.ObservationsDeduped,
		m.ObservationsCreateErrors,
		m.EventProcessingDuration,
		filterInstance,
	)

	// Setup informers
	if err := kubernetes.SetupInformers(ctx, informerFactory, gvrs, eventProcessor, stopCh); err != nil {
		log.Fatalf("❌ Failed to setup informers: %v", err)
	}

	// Update informer cache sync metrics
	m.InformerCacheSync.WithLabelValues("policyreports").Set(1)
	m.InformerCacheSync.WithLabelValues("vulnerabilityreports").Set(1)

	// Create webhook channels
	falcoAlertsChan := make(chan map[string]interface{}, 100)
	auditEventsChan := make(chan map[string]interface{}, 200)

	// Create HTTP server
	httpServer := server.NewServer(falcoAlertsChan, auditEventsChan, m.WebhookRequests, m.WebhookDropped)

	// Create ConfigMap poller with centralized observation creator
	configMapPoller := watcher.NewConfigMapPoller(
		clients.Standard,
		clients.Dynamic,
		gvrs.Observations,
		eventProcessor,
		webhookProcessor,
		m.EventsTotal,
		observationCreator,
	)

	// WaitGroup for goroutines
	var wg sync.WaitGroup

	// Start HTTP server
	httpServer.Start(ctx, &wg)

	// Process webhook channels in background
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				// Drain channel non-blockingly
				for {
					select {
					case <-falcoAlertsChan:
					default:
						return
					}
				}
			case alert := <-falcoAlertsChan:
				log.Printf("  🔄 [FALCO] Processing alert from channel: %v", alert["rule"])
				if err := webhookProcessor.ProcessFalcoAlert(ctx, alert); err != nil {
					log.Printf("⚠️  Failed to process Falco alert: %v", err)
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				// Drain channel non-blockingly
				for {
					select {
					case <-auditEventsChan:
					default:
						return
					}
				}
			case auditEvent := <-auditEventsChan:
				auditID := fmt.Sprintf("%v", auditEvent["auditID"])
				log.Printf("  🔄 [AUDIT] Processing event from channel: auditID=%s", auditID)
				if err := webhookProcessor.ProcessAuditEvent(ctx, auditEvent); err != nil {
					log.Printf("⚠️  Failed to process audit event: %v", err)
				}
			}
		}
	}()

	// Mark server as ready
	httpServer.SetReady(true)

	// Start ConfigMap poller
	go configMapPoller.Start(ctx)

	// Start ConfigMap loader for dynamic filter config reloading
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := configMapLoader.Start(ctx); err != nil {
			log.Printf("⚠️  ConfigMap loader stopped: %v", err)
		}
	}()

	// Create and start garbage collector
	gcCollector := gc.NewCollector(
		clients.Dynamic,
		gvrs.Observations,
		m.ObservationsDeleted,
		m.GCRunsTotal,
		m.GCDuration,
		m.GCErrors,
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		gcCollector.Start(ctx)
	}()

	// Log configuration
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("✅ zen-watcher READY")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	autoDetect := os.Getenv("AUTO_DETECT_ENABLED")
	if autoDetect == "" {
		autoDetect = "true"
	}
	log.Printf("🔍 Auto-detect: %s", autoDetect)

	// Wait for shutdown
	lifecycle.WaitForShutdown(ctx, &wg, func() {
		totalCount := eventProcessor.GetTotalCount() + webhookProcessor.GetTotalCount()
		log.Printf("✅ zen-watcher stopped (created %d events)", totalCount)
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	})
}
