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

package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/internal/informers"
	"github.com/kube-zen/zen-watcher/internal/kubernetes"
	"github.com/kube-zen/zen-watcher/internal/lifecycle"
	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/balancer"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/dispatcher"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/gc"
	"github.com/kube-zen/zen-watcher/pkg/leader"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/optimization"
	"github.com/kube-zen/zen-watcher/pkg/orchestrator"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	"github.com/kube-zen/zen-watcher/pkg/scaling"
	"github.com/kube-zen/zen-watcher/pkg/server"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Version, Commit, and BuildDate are set via ldflags during build
var (
	Version   = "1.0.0-alpha"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Global system metrics tracker for HA coordination
var systemMetrics *metrics.SystemMetrics

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

	// Initialize system metrics early for HA coordination
	systemMetrics = metrics.NewSystemMetrics()
	defer systemMetrics.Close()

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

	// Register optimization decision metrics
	// This registers Prometheus metrics for optimization decisions
	optimization.RegisterDecisionMetrics()

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
	filterInstance := filter.NewFilterWithMetrics(filterConfig, m)

	// Create ConfigMap loader for dynamic reloading
	configMapLoader := config.NewConfigMapLoader(clients.Standard, filterInstance)

	// Initialize ConfigManager for feature configuration
	configNamespace := os.Getenv("CONFIG_NAMESPACE")
	if configNamespace == "" {
		configNamespace = "zen-system"
	}
	configManager := config.NewConfigManagerWithMetrics(clients.Standard, configNamespace, m)

	// Ingester CRD is now the single source of configuration
	// Filter and dedup configuration is part of Ingester CRD spec.processing

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

	// Processing order is configured via Ingester CRD spec.processing.order

	// Set system metrics tracker for HA coordination
	if systemMetrics != nil {
		observationCreator.SetSystemMetrics(systemMetrics)
	}

	// Set destination metrics for tracking delivery
	observationCreator.SetDestinationMetrics(m)

	// Create processor for GenericOrchestrator (uses same filter, deduper, and observationCreator)
	deduper := observationCreator.GetDeduper()
	proc := processor.NewProcessorWithMetrics(filterInstance, deduper, observationCreator, m)

	// Initialize leader checker (for zen-lead annotation-based leader election)
	var leaderChecker *leader.Checker
	enableLeaderElection := os.Getenv("ENABLE_LEADER_ELECTION") == "true"
	if enableLeaderElection {
		var err error
		leaderChecker, err = leader.NewChecker(clients.Standard)
		if err != nil {
			log.Warn("Failed to initialize leader checker, continuing without leader election",
				logger.Fields{
					Component: "main",
					Operation: "leader_init",
					Error:     err,
				})
		} else {
			log.Info("Leader election enabled via zen-lead",
				logger.Fields{
					Component: "main",
					Operation: "leader_init",
					Additional: map[string]interface{}{
						"leader_election": "zen-lead",
					},
				})
		}
	}

	// Create informer manager for generic adapters
	informerManager := informers.NewManager(informers.Config{
		DynamicClient: clients.Dynamic,
		DefaultResync: 0, // Watch-only, no periodic resync
	})

	// Create generic adapter factory
	genericAdapterFactory := generic.NewFactory(informerManager, clients.Standard)

	// Create GenericOrchestrator with metrics for dynamic Ingester CRD-based adapter management
	genericOrchestrator := orchestrator.NewGenericOrchestratorWithMetrics(
		genericAdapterFactory,
		clients.Dynamic,
		proc,
		m,
	)

	// Create webhook channels for Falco and Audit webhooks
	// Note: Other webhook sources can be configured via Ingester CRDs
	falcoAlertsChan := make(chan map[string]interface{}, 100)
	auditEventsChan := make(chan map[string]interface{}, 200)

	// Create Ingester store and informer for Ingester CRD support
	ingesterStore := config.NewIngesterStore()
	ingesterInformer := config.NewIngesterInformer(ingesterStore, clients.Dynamic)

	// Set up GVR resolver for dynamic destination GVR support
	// The resolver looks up the destination GVR from the ingester config store
	observationCreator.SetGVRResolver(func(source string) schema.GroupVersionResource {
		// Get ingester config by source
		ingesterConfig, exists := ingesterStore.GetBySource(source)
		if exists && ingesterConfig != nil && len(ingesterConfig.Destinations) > 0 {
			// Use first CRD destination's GVR
			for _, dest := range ingesterConfig.Destinations {
				if dest.Type == "crd" {
					return dest.GVR
				}
			}
		}
		// Fallback to default observations GVR
		return gvrs.Observations
	})

	// Create HTTP server (handles Falco and Audit webhooks)
	httpServer := server.NewServerWithIngester(
		falcoAlertsChan,
		auditEventsChan,
		m.WebhookRequests,
		m.WebhookDropped,
		ingesterStore,
		observationCreator, // Parameters kept for API compatibility
	)

	// Create adapter factory - only creates K8sEventsAdapter (exception)
	// All other sources are configured via Ingester CRDs
	adapterFactory := watcher.NewAdapterFactory(clients.Standard)

	// Create all adapters
	adapters := adapterFactory.CreateAdapters()

	// Create adapter launcher to run all adapters and process events
	adapterLauncher := watcher.NewAdapterLauncher(adapters, observationCreator)

	// WaitGroup for goroutines
	var wg sync.WaitGroup

	// Create worker pool (will be configured from ConfigMap)
	var workerPool *dispatcher.WorkerPool
	workerPool = dispatcher.NewWorkerPool(5, 10) // Defaults, will be updated from config

	// Register config change handler
	configManager.OnConfigChange(func(newConfig map[string]interface{}) {
		handleConfigChange(newConfig, workerPool, adapterLauncher, filterInstance, *log)
	})

	// Start ConfigManager
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := configManager.Start(ctx); err != nil {
			log.Error("ConfigManager stopped",
				logger.Fields{
					Component: "main",
					Operation: "config_manager",
					Error:     err,
				})
		}
	}()

	// Apply initial configuration
	initialConfig := configManager.GetConfigWithDefaults()
	handleConfigChange(initialConfig, workerPool, adapterLauncher, filterInstance, *log)

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

	// Filter and dedup configuration is now part of Ingester CRD spec.processing
	// The IngesterInformer (started below) handles all configuration updates

	// Start Ingester informer to populate the store
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ingesterInformer.Start(ctx); err != nil {
			log.Error("Ingester informer stopped",
				logger.Fields{
					Component: "main",
					Operation: "ingester_informer",
					Error:     err,
				})
		}
	}()

	// Start GenericOrchestrator for dynamic Ingester CRD-based adapter management
	// Only start if leader election is disabled OR if we are the leader
	wg.Add(1)
	go func() {
		defer wg.Done()
		if enableLeaderElection && leaderChecker != nil {
			// Check if we are the leader before starting informer-based components
			isLeader, err := leaderChecker.IsLeader(ctx)
			if err != nil {
				log.Error("Failed to check leader status for GenericOrchestrator",
					logger.Fields{
						Component: "main",
						Operation: "leader_check",
						Error:     err,
					})
				return
			}
			if !isLeader {
				log.Info("Not the leader, skipping GenericOrchestrator (informer-based sources)",
					logger.Fields{
						Component: "main",
						Operation: "generic_orchestrator",
						Additional: map[string]interface{}{
							"reason": "not_leader",
						},
					})
				// Watch for leader changes and start when we become leader
				go leaderChecker.WatchLeader(ctx, func(isLeader bool) {
					if isLeader {
						log.Info("Became leader, starting GenericOrchestrator",
							logger.Fields{
								Component: "main",
								Operation: "generic_orchestrator",
							})
						if err := genericOrchestrator.Start(ctx); err != nil {
							log.Error("GenericOrchestrator stopped",
								logger.Fields{
									Component: "main",
									Operation: "generic_orchestrator",
									Error:     err,
								})
						}
					}
				})
				return
			}
		}
		// Start GenericOrchestrator (either no leader election or we are the leader)
		if err := genericOrchestrator.Start(ctx); err != nil {
			log.Error("GenericOrchestrator stopped",
				logger.Fields{
					Component: "main",
					Operation: "generic_orchestrator",
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
		// Ingester CRD configuration is accessed via ingesterStore
		optimizer = optimization.NewOptimizerWithProcessor(obsSmartProc, nil)
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
		// Ingester CRD configuration is accessed via ingesterStore
		optimizer = optimization.NewOptimizer(nil)
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

	// Initialize optimization advisor (if Prometheus client is available)
	// Note: Prometheus client would be needed for full metrics analysis
	// For now, we'll create a simplified version that works with the metrics we have
	// optimizationLogger := logging.NewOptimizationLogger(15 * time.Minute)

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
	// Only start if leader election is disabled OR if we are the leader
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
		if enableLeaderElection && leaderChecker != nil {
			// Check if we are the leader before starting GC
			isLeader, err := leaderChecker.IsLeader(ctx)
			if err != nil {
				log.Error("Failed to check leader status for GC",
					logger.Fields{
						Component: "main",
						Operation: "leader_check",
						Error:     err,
					})
				return
			}
			if !isLeader {
				log.Info("Not the leader, skipping garbage collector",
					logger.Fields{
						Component: "main",
						Operation: "gc",
						Additional: map[string]interface{}{
							"reason": "not_leader",
						},
					})
				// Watch for leader changes and start when we become leader
				go leaderChecker.WatchLeader(ctx, func(isLeader bool) {
					if isLeader {
						log.Info("Became leader, starting garbage collector",
							logger.Fields{
								Component: "main",
								Operation: "gc",
							})
						gcCollector.Start(ctx)
					}
				})
				return
			}
		}
		// Start GC (either no leader election or we are the leader)
		gcCollector.Start(ctx)
	}()

	// Initialize HA optimization components (if HA is enabled)
	haConfig := config.LoadHAConfig()
	var haDedupOptimizer *optimization.HADedupOptimizer
	var haScalingCoordinator *scaling.HPACoordinator
	var haLoadBalancer *balancer.LoadBalancer
	var haMetrics *metrics.HAMetrics

	if haConfig.IsHAEnabled() {
		log.Info("HA optimization enabled, initializing HA components",
			logger.Fields{
				Component: "main",
				Operation: "ha_init",
			})

		// Initialize HA metrics
		haMetrics = metrics.NewHAMetrics()

		// Get replica ID from environment
		replicaID := os.Getenv("HOSTNAME")
		if replicaID == "" {
			replicaID = fmt.Sprintf("replica-%d", time.Now().UnixNano())
		}

		// Initialize HA Dedup Optimizer
		if haConfig.DedupOptimization.Enabled {
			eventCounter := m.EventsTotal.WithLabelValues("", "", "", "", "", "")
			haDedupOptimizer = optimization.NewHADedupOptimizer(&haConfig.DedupOptimization, eventCounter)
			if haDedupOptimizer != nil {
				haDedupOptimizer.Start(30 * time.Second) // Update every 30 seconds
				log.Info("HA dedup optimizer started",
					logger.Fields{
						Component: "main",
						Operation: "ha_dedup_init",
					})
			}
		}

		// Initialize HA Scaling Coordinator
		if haConfig.AutoScaling.Enabled {
			haScalingCoordinator = scaling.NewHPACoordinator(&haConfig.AutoScaling, haMetrics, replicaID)
			if haScalingCoordinator != nil {
				haScalingCoordinator.Start(ctx, 1*time.Minute) // Evaluate every minute
				log.Info("HA scaling coordinator started",
					logger.Fields{
						Component: "main",
						Operation: "ha_scaling_init",
					})
			}
		}

		// Initialize HA Load Balancer
		if haConfig.LoadBalancing.Strategy != "" {
			haLoadBalancer = balancer.NewLoadBalancer(&haConfig.LoadBalancing)
			log.Info("HA load balancer initialized",
				logger.Fields{
					Component: "main",
					Operation: "ha_balancer_init",
					Additional: map[string]interface{}{
						"strategy": haConfig.LoadBalancing.Strategy,
					},
				})
		}

		// Cache optimization is handled by the deduper itself
		// No separate adaptive cache manager needed

		// Start metrics collection loop for HA components
		if haScalingCoordinator != nil || haLoadBalancer != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						// Collect real system metrics
						cpuUsage := systemMetrics.GetCPUUsagePercent()
						memoryUsage := float64(systemMetrics.GetMemoryUsage())
						eventsPerSec := systemMetrics.GetEventsPerSecond()
						queueDepth := systemMetrics.GetQueueDepth()
						responseTime := systemMetrics.GetResponseTime()

						// Log real metrics for debugging (if debug mode enabled)
						if os.Getenv("DEBUG_METRICS") == "true" {
							log.Debug("HA Metrics collected",
								logger.Fields{
									Component: "ha",
									Operation: "metrics_collection",
									Additional: map[string]interface{}{
										"cpu_usage":      cpuUsage,
										"memory_usage":   memoryUsage,
										"events_per_sec": eventsPerSec,
										"queue_depth":    queueDepth,
										"response_time":  responseTime,
									},
								})
						}

						// Update scaling coordinator
						if haScalingCoordinator != nil {
							haScalingCoordinator.UpdateMetrics(cpuUsage, memoryUsage, eventsPerSec, queueDepth, responseTime)
						}

						// Update load balancer
						if haLoadBalancer != nil {
							load := 0.0 // TODO: Calculate load factor
							haLoadBalancer.UpdateReplica(replicaID, load, cpuUsage, memoryUsage, eventsPerSec, true)
						}

						// Update HTTP server HA status
						httpServer.UpdateHAStatus(cpuUsage, memoryUsage, eventsPerSec, 0.0, queueDepth)

						// Cache metrics are handled by the deduper itself

						// Update HA dedup optimizer with events
						if haDedupOptimizer != nil {
							// Events are recorded automatically via the event counter
							// Get optimal window and update deduper
							optimalWindow := haDedupOptimizer.GetOptimalWindow()
							deduper := observationCreator.GetDeduper()
							if deduper != nil {
								deduper.SetDefaultWindow(int(optimalWindow.Seconds()))
							}
						}

						// Update queue depth from actual channels
						if systemMetrics != nil {
							queueDepth := len(falcoAlertsChan) + len(auditEventsChan)
							systemMetrics.SetQueueDepth(queueDepth)
						}

						// Cache size is managed by the deduper itself

					case <-ctx.Done():
						return
					}
				}
			}()
		}
	}

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
		if workerPool != nil {
			workerPool.Stop()
		}
		log.Info("zen-watcher stopped",
			logger.Fields{
				Component: "main",
				Operation: "shutdown",
			})
	})
}

// handleConfigChange handles configuration changes from ConfigMap
func handleConfigChange(
	newConfig map[string]interface{},
	workerPool *dispatcher.WorkerPool,
	adapterLauncher *watcher.AdapterLauncher,
	filterInstance *filter.Filter,
	log logger.Logger,
) {
	// Update worker pool configuration
	if workerPoolConfig, ok := newConfig["worker_pool"].(map[string]interface{}); ok {
		newWorkerConfig := dispatcher.WorkerPoolConfig{
			Enabled:   getBool(workerPoolConfig, "enabled", false),
			Size:      getInt(workerPoolConfig, "size", 5),
			QueueSize: getInt(workerPoolConfig, "queue_size", 1000),
		}

		if newWorkerConfig.Enabled {
			workerPool.UpdateConfig(newWorkerConfig)
			if !workerPool.IsRunning() {
				workerPool.Start()
			}
			adapterLauncher.SetWorkerPool(workerPool)
			log.Info("Worker pool configuration updated",
				logger.Fields{
					Component: "main",
					Operation: "config_update",
					Additional: map[string]interface{}{
						"enabled":    newWorkerConfig.Enabled,
						"size":       newWorkerConfig.Size,
						"queue_size": newWorkerConfig.QueueSize,
					},
				})
		} else {
			workerPool.Stop()
			adapterLauncher.SetWorkerPool(nil)
			log.Info("Worker pool disabled via configuration",
				logger.Fields{
					Component: "main",
					Operation: "config_update",
				})
		}
	}

	// Update namespace filtering
	if nsFilterConfigMap, ok := newConfig["namespace_filtering"].(map[string]interface{}); ok {
		enabled := getBool(nsFilterConfigMap, "enabled", true)
		if enabled {
			// Get current filter config
			currentFilterConfig := filterInstance.GetConfig()
			if currentFilterConfig == nil {
				currentFilterConfig = &filter.FilterConfig{
					Sources: make(map[string]filter.SourceFilter),
				}
			}

			// Create a new config with updated global namespace filter
			// We need to preserve existing source filters and expression
			newFilterConfig := &filter.FilterConfig{
				Expression: currentFilterConfig.Expression,
				Sources:    currentFilterConfig.Sources,
			}

			// Create or update global namespace filter
			globalFilter := &filter.GlobalNamespaceFilter{
				Enabled: true,
			}

			// Extract included namespaces
			if included, ok := nsFilterConfigMap["included_namespaces"].([]interface{}); ok {
				globalFilter.IncludedNamespaces = make([]string, 0, len(included))
				for _, ns := range included {
					if nsStr, ok := ns.(string); ok {
						globalFilter.IncludedNamespaces = append(globalFilter.IncludedNamespaces, nsStr)
					}
				}
			}

			// Extract excluded namespaces
			if excluded, ok := nsFilterConfigMap["excluded_namespaces"].([]interface{}); ok {
				globalFilter.ExcludedNamespaces = make([]string, 0, len(excluded))
				for _, ns := range excluded {
					if nsStr, ok := ns.(string); ok {
						globalFilter.ExcludedNamespaces = append(globalFilter.ExcludedNamespaces, nsStr)
					}
				}
			}

			// Update filter config with global namespace filter
			newFilterConfig.GlobalNamespaceFilter = globalFilter
			filterInstance.UpdateConfig(newFilterConfig)

			log.Info("Namespace filtering configuration updated",
				logger.Fields{
					Component: "main",
					Operation: "config_update",
					Additional: map[string]interface{}{
						"enabled":             enabled,
						"included_namespaces": globalFilter.IncludedNamespaces,
						"excluded_namespaces": globalFilter.ExcludedNamespaces,
					},
				})
		} else {
			// Disable global namespace filtering
			currentFilterConfig := filterInstance.GetConfig()
			if currentFilterConfig != nil {
				// Create a copy to avoid modifying the original
				newConfig := &filter.FilterConfig{
					Expression: currentFilterConfig.Expression,
					Sources:    currentFilterConfig.Sources,
				}
				if currentFilterConfig.GlobalNamespaceFilter != nil {
					newConfig.GlobalNamespaceFilter = &filter.GlobalNamespaceFilter{
						Enabled:            false,
						IncludedNamespaces: currentFilterConfig.GlobalNamespaceFilter.IncludedNamespaces,
						ExcludedNamespaces: currentFilterConfig.GlobalNamespaceFilter.ExcludedNamespaces,
					}
				}
				filterInstance.UpdateConfig(newConfig)
			}
			log.Info("Namespace filtering disabled via configuration",
				logger.Fields{
					Component: "main",
					Operation: "config_update",
				})
		}
	}
}

// Helper functions for type conversion
func getBool(config map[string]interface{}, key string, defaultValue bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return defaultValue
}

func getInt(config map[string]interface{}, key string, defaultValue int) int {
	if value, ok := config[key].(int); ok {
		return value
	}
	if floatValue, ok := config[key].(float64); ok {
		return int(floatValue)
	}
	return defaultValue
}

func getDuration(config map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if strValue, ok := config[key].(string); ok {
		if duration, err := time.ParseDuration(strValue); err == nil {
			return duration
		}
	}
	return defaultValue
}
