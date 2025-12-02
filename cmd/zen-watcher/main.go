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
	"sync"

	"github.com/kube-zen/zen-watcher/internal/kubernetes"
	"github.com/kube-zen/zen-watcher/internal/lifecycle"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/gc"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/server"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
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
				"version":    "1.0.22",
				"go_version": "1.24",
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

	// Create centralized observation creator with filter
	// This is used by AdapterLauncher to process all events
	observationCreator := watcher.NewObservationCreator(
		clients.Dynamic,
		gvrs.Observations,
		m.EventsTotal,
		m.ObservationsCreated,
		m.ObservationsFiltered,
		m.ObservationsDeduped,
		m.ObservationsCreateErrors,
		filterInstance,
	)

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
