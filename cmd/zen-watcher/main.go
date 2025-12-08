// Copyright 2024 The Zen Watcher Authors
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

package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/internal/kubernetes"
	"github.com/kube-zen/zen-watcher/internal/lifecycle"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/gc"
	"github.com/kube-zen/zen-watcher/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/optimization"
	"github.com/kube-zen/zen-watcher/pkg/server"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
)

// Version, Commit, and BuildDate are set via ldflags during build
var (
	Version   = "1.0.0-alpha"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	// Initialize structured logger
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}
	development := os.Getenv("LOG_DEVELOPMENT") == "true"
	if err := logger.Init(logLevel, development); err != nil {
		// Fallback to standard log if zap init fails
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	log := logger.GetLogger()
	log.Info("zen-watcher starting",
		logger.Fields{
			Component: "main",
			Additional: map[string]interface{}{
				"version":    Version,
				"commit":     Commit,
				"build_date": BuildDate,
				"license":    "Apache 2.0",
			},
		})

	// Setup signal handling and context
	ctx, _ := lifecycle.SetupSignalHandler()

	// Initialize metrics
	m := metrics.NewMetrics()

	// Initialize Kubernetes clients
	clients, err := kubernetes.NewClients()
	if err != nil {
		log.Fatal("Failed to initialize Kubernetes clients",
			logger.Fields{
				Component: "main",
				Operation: "kubernetes_init",
				Error:     err,
			})
	}

	// Get GVRs
	gvrs := kubernetes.NewGVRs()

	// Create informer factory
	informerFactory := kubernetes.NewInformerFactory(clients.Dynamic)

	// Load filter configuration from ConfigMap (initial load)
	filterConfig, err := filter.LoadFilterConfig(clients.Standard)
	if err != nil {
		log.Warn("Failed to load filter config, continuing without filter",
			logger.Fields{
				Component: "main",
				Operation: "filter_load",
				Error:     err,
			})
		filterConfig = &filter.FilterConfig{Sources: make(map[string]filter.SourceFilter)}
	}
	filterInstance := filter.NewFilter(filterConfig)

	// Create ConfigMap loader for dynamic reloading
	configMapLoader := config.NewConfigMapLoader(clients.Standard, filterInstance)

	// Create SourceConfig and TypeConfig loaders for new universal event watcher architecture
	sourceConfigLoader := config.NewSourceConfigLoader(clients.Dynamic)
	typeConfigLoader := config.NewTypeConfigLoader(clients.Dynamic)

	// Create optimization metrics wrapper
	optimizationMetrics := watcher.NewOptimizationMetrics(
		m.FilterPassRate,
		m.DedupEffectiveness,
		m.LowSeverityPercent,
		m.ObservationsPerMinute,
		m.ObservationsPerHour,
		m.SeverityDistribution,
	)

	// Create centralized observation creator with filter and optimization metrics
	// This is used by AdapterLauncher to process all events
	observationCreator := watcher.NewObservationCreatorWithOptimization(
		clients.Dynamic,
		gvrs.Observations,
		m.EventsTotal,
		m.ObservationsCreated,
		m.ObservationsFiltered,
		m.ObservationsDeduped,
		m.ObservationsCreateErrors,
		filterInstance,
		optimizationMetrics,
	)

	// Set source config loader for dynamic processing order
	observationCreator.SetSourceConfigLoader(sourceConfigLoader)

	// Create webhook channels for Falco and Audit adapters
	falcoAlertsChan := make(chan map[string]interface{}, 100)
	auditEventsChan := make(chan map[string]interface{}, 200)

	// Create HTTP server (adapters will read from channels)
	httpServer := server.NewServer(falcoAlertsChan, auditEventsChan, m.WebhookRequests, m.WebhookDropped)

	// Create adapter factory to build all source adapters
	adapterFactory := watcher.NewAdapterFactory(
		informerFactory,
		gvrs.PolicyReport,
		gvrs.TrivyReport,
		clients.Standard,
		falcoAlertsChan,
		auditEventsChan,
	)

	// Create all adapters
	adapters := adapterFactory.CreateAdapters()

	// Create adapter launcher to run all adapters and process events
	adapterLauncher := watcher.NewAdapterLauncher(adapters, observationCreator)

	// WaitGroup for goroutines
	var wg sync.WaitGroup

	// Start HTTP server
	httpServer.Start(ctx, &wg)

	// Mark server as ready
	httpServer.SetReady(true)

	// Start adapter launcher (runs all adapters and processes events)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := adapterLauncher.Start(ctx); err != nil {
			log.Error("Adapter launcher stopped",
				logger.Fields{
					Component: "main",
					Operation: "adapter_launcher",
					Error:     err,
				})
		}
	}()

	// Start ConfigMap loader for dynamic filter config reloading
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := configMapLoader.Start(ctx); err != nil {
			log.Error("ConfigMap loader stopped",
				logger.Fields{
					Component: "main",
					Operation: "configmap_loader",
					Error:     err,
				})
		}
	}()

	// Create ObservationFilter loader for CRD-based filter configuration
	// This merges with ConfigMap config automatically
	observationFilterLoader := config.NewObservationFilterLoader(clients.Dynamic, filterInstance, configMapLoader)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := observationFilterLoader.Start(ctx); err != nil {
			log.Error("ObservationFilter loader stopped",
				logger.Fields{
					Component: "main",
					Operation: "observationfilter_loader",
					Error:     err,
				})
		}
	}()

	// Get default dedup window from env for ObservationDedupConfig loader
	defaultDedupWindow := 60
	if windowStr := os.Getenv("DEDUP_WINDOW_SECONDS"); windowStr != "" {
		if w, err := strconv.Atoi(windowStr); err == nil && w > 0 {
			defaultDedupWindow = w
		}
	}

	// Create ObservationDedupConfig loader for CRD-based dedup configuration
	// This allows per-source deduplication windows to be configured via CRD
	observationDedupConfigLoader := config.NewObservationDedupConfigLoader(
		clients.Dynamic,
		observationCreator.GetDeduper(),
		defaultDedupWindow,
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := observationDedupConfigLoader.Start(ctx); err != nil {
			log.Error("ObservationDedupConfig loader stopped",
				logger.Fields{
					Component: "main",
					Operation: "observationdedupconfig_loader",
					Error:     err,
				})
		}
	}()

	// Start SourceConfig loader watcher (for new universal event watcher architecture)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sourceConfigLoader.Start(ctx); err != nil {
			log.Error("SourceConfig loader stopped",
				logger.Fields{
					Component: "main",
					Operation: "sourceconfig_loader",
					Error:     err,
				})
		}
	}()

	// Start TypeConfig loader watcher (for new universal event watcher architecture)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := typeConfigLoader.Start(ctx); err != nil {
			log.Error("TypeConfig loader stopped",
				logger.Fields{
					Component: "main",
					Operation: "typeconfig_loader",
					Error:     err,
				})
		}
	}()

	// Create and start optimization engine (for per-source auto-optimization)
	// Share the SmartProcessor instance from ObservationCreator so metrics are unified
	obsSmartProc := observationCreator.GetSmartProcessor()
	var optimizer *optimization.Optimizer
	
	if obsSmartProc != nil {
		// Create optimizer with shared SmartProcessor
		optimizer = optimization.NewOptimizerWithProcessor(obsSmartProc, sourceConfigLoader)
		log.Info("Optimization engine initialized with shared SmartProcessor",
			logger.Fields{
				Component: "main",
				Operation: "optimizer_integration",
				Additional: map[string]interface{}{
					"optimization_enabled": true,
				},
			})
	} else {
		// Fallback: create optimizer with its own SmartProcessor
		optimizer = optimization.NewOptimizer(sourceConfigLoader)
		log.Info("Optimization engine initialized with independent SmartProcessor",
			logger.Fields{
				Component: "main",
				Operation: "optimizer_integration",
			})
	}
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := optimizer.Start(ctx); err != nil {
			log.Error("Optimizer stopped",
				logger.Fields{
					Component: "main",
					Operation: "optimizer",
					Error:     err,
				})
		}
	}()

	// Initialize optimization advisor and logger
	// Note: Prometheus client would be needed for full metrics analysis
	// For now, we'll create a simplified version that works with the metrics we have
	optimizationLogger := logging.NewOptimizationLogger(15 * time.Minute)

	// Start optimization advisor (if Prometheus client is available)
	// This is optional and can be enabled when Prometheus integration is ready
	// advisor := advisor.NewAdvisor(metricsAnalyzer, suggestionEngine, impactTracker)
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := advisor.Start(ctx); err != nil {
	// 		log.Error("Optimization advisor stopped", ...)
	// 	}
	// }()

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
	log.Info("zen-watcher ready",
		logger.Fields{
			Component: "main",
			Operation: "startup_complete",
		})
	autoDetect := os.Getenv("AUTO_DETECT_ENABLED")
	if autoDetect == "" {
		autoDetect = "true"
	}
	log.Info("Configuration loaded",
		logger.Fields{
			Component: "main",
			Operation: "config_load",
			Additional: map[string]interface{}{
				"auto_detect_enabled": autoDetect,
			},
		})

	// Wait for shutdown
	lifecycle.WaitForShutdown(ctx, &wg, func() {
		// Stop adapter launcher gracefully
		adapterLauncher.Stop()
		log.Info("zen-watcher stopped",
			logger.Fields{
				Component: "main",
				Operation: "shutdown",
			})
	})
}
