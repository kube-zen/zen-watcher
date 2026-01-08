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
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	sdkconfig "github.com/kube-zen/zen-sdk/pkg/config"
	"github.com/kube-zen/zen-sdk/pkg/leader"
	sdklifecycle "github.com/kube-zen/zen-sdk/pkg/lifecycle"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-sdk/pkg/zenlead"
	"github.com/kube-zen/zen-watcher/internal/informers"
	"github.com/kube-zen/zen-watcher/internal/kubernetes"
	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/balancer"
	watcherconfig "github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/dispatcher"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/gc"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/orchestrator"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	"github.com/kube-zen/zen-watcher/pkg/scaling"
	"github.com/kube-zen/zen-watcher/pkg/server"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// Version, Commit, and BuildDate are set via ldflags during build
// Version should match the VERSION file in the repository root
var (
	Version   = "1.2.1"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Global system metrics tracker for HA coordination
var systemMetrics *metrics.SystemMetrics

// Scheme for controller-runtime (used only for leader election)
var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

var (
	leaderElectionMode = flag.String("leader-election-mode", "builtin", "Leader election mode: builtin (default) or disabled")
	leaderElectionID   = flag.String("leader-election-id", "", "The ID for leader election (default: zen-watcher-leader-election). Required for builtin mode.")
)

func main() {
	flag.Parse()

	// Initialize logger and system metrics
	log := sdklog.NewLogger("zen-watcher")
	setupLog := log.WithComponent("setup")
	systemMetrics = metrics.NewSystemMetrics()
	defer systemMetrics.Close()

	setupLog.Info("zen-watcher starting",
		sdklog.String("version", Version),
		sdklog.String("commit", Commit),
		sdklog.String("buildDate", BuildDate),
		sdklog.String("license", "Apache 2.0"))

	// Setup signal handling and context
	ctx, cancel := sdklifecycle.ShutdownContext(context.Background(), "zen-watcher")
	defer cancel()

	// Initialize core components
	m := metrics.NewMetrics()

	// Initialize Kubernetes clients
	clients, err := kubernetes.NewClients()
	if err != nil {
		setupLog.Error(err, "Failed to initialize Kubernetes clients", sdklog.ErrorCode("CLIENT_ERROR"), sdklog.Operation("kubernetes_init"))
		os.Exit(1)
	}

	// Initialize filter and config
	filterInstance, configMapLoader, configManager := initializeFilterAndConfig(clients, m, setupLog)

	// Initialize observation creator and processor
	gvrs := kubernetes.NewGVRs()
	observationCreator, proc := initializeObservationCreator(clients, gvrs, filterInstance, m, setupLog)

	// Setup leader election and manager
	namespace, err := leader.RequirePodNamespace()
	if err != nil {
		setupLog.Error(err, "Failed to determine pod namespace", sdklog.ErrorCode("NAMESPACE_ERROR"), sdklog.Operation("namespace_init"))
		os.Exit(1)
	}

	zenlead.ControllerRuntimeDefaults(clients.Config)
	leaderManager, leaderElectedCh := setupLeaderElection(clients, namespace, setupLog)

	// Initialize adapters and orchestrator
	genericOrchestrator, _, ingesterInformer, httpServer := initializeAdapters(clients, proc, m, gvrs, observationCreator, setupLog)

	// Start all services
	var wg sync.WaitGroup
	startAllServices(ctx, &wg, configManager, configMapLoader, ingesterInformer, genericOrchestrator, httpServer,
		observationCreator, clients, gvrs, m, leaderElectedCh, filterInstance, log, setupLog, leaderManager)

	// Wait for shutdown
	<-ctx.Done()
	setupLog.Info("Shutting down zen-watcher...", sdklog.Operation("shutdown"))
	wg.Wait()
	setupLog.Info("zen-watcher stopped", sdklog.Operation("shutdown_complete"))
}

// initializeFilterAndConfig initializes filter and config components
func initializeFilterAndConfig(clients *kubernetes.Clients, m *metrics.Metrics, setupLog *sdklog.Logger) (*filter.Filter, *watcherconfig.ConfigMapLoader, *watcherconfig.ConfigManager) {
	// Load filter configuration from ConfigMap (initial load)
	filterConfig, err := filter.LoadFilterConfig(clients.Standard)
	if err != nil {
		setupLog.Warn("Failed to load filter config, continuing without filter", sdklog.Operation("filter_load"), sdklog.String("error", err.Error()))
		filterConfig = &filter.FilterConfig{Sources: make(map[string]filter.SourceFilter)}
	}
	filterInstance := filter.NewFilterWithMetrics(filterConfig, m)

	// Create ConfigMap loader for dynamic reloading
	configMapLoader := watcherconfig.NewConfigMapLoader(clients.Standard, filterInstance)

	// Initialize ConfigManager for feature configuration
	configNamespace := sdkconfig.RequireEnvWithDefault("CONFIG_NAMESPACE", "zen-system")
	configManager := watcherconfig.NewConfigManagerWithMetrics(clients.Standard, configNamespace, m)

	return filterInstance, configMapLoader, configManager
}

// initializeObservationCreator initializes observation creator and processor
func initializeObservationCreator(clients *kubernetes.Clients, gvrs *kubernetes.GVRs, filterInstance *filter.Filter, m *metrics.Metrics, setupLog *sdklog.Logger) (*watcher.ObservationCreator, *processor.Processor) {
	// Create centralized observation creator
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

	// Set system metrics tracker for HA coordination
	if systemMetrics != nil {
		observationCreator.SetSystemMetrics(systemMetrics)
	}

	// Set destination metrics for tracking delivery
	observationCreator.SetDestinationMetrics(m)

	// Create processor for GenericOrchestrator
	deduper := observationCreator.GetDeduper()
	proc := processor.NewProcessorWithMetrics(filterInstance, deduper, observationCreator, m)

	return observationCreator, proc
}

// setupLeaderElection sets up leader election and returns manager and elected channel
func setupLeaderElection(clients *kubernetes.Clients, namespace string, setupLog *sdklog.Logger) (ctrl.Manager, <-chan struct{}) {
	// Setup controller-runtime manager
	baseOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable metrics server (we have our own)
		},
		HealthProbeBindAddress: ":8081", // Health probes on separate port
	}

	// Configure leader election
	leConfig := configureLeaderElection(namespace, setupLog)

	// Prepare manager options with leader election
	mgrOpts, err := zenlead.PrepareManagerOptions(&baseOpts, &leConfig)
	if err != nil {
		setupLog.Error(err, "Failed to prepare manager options", sdklog.ErrorCode("MANAGER_OPTIONS_ERROR"), sdklog.Operation("leader_init"))
		os.Exit(1)
	}

	// Get replica count from environment
	replicaCount := sdkconfig.RequireEnvIntWithDefault("REPLICA_COUNT", 1)

	// Enforce safe HA configuration
	if err := zenlead.EnforceSafeHA(replicaCount, mgrOpts.LeaderElection); err != nil {
		setupLog.Error(err, "Unsafe HA configuration", sdklog.ErrorCode("UNSAFE_HA_CONFIG"), sdklog.Operation("leader_init"))
		os.Exit(1)
	}

	// Always create manager (leader election is configured via options)
	leaderManager, err := ctrl.NewManager(clients.Config, mgrOpts)
	if err != nil {
		setupLog.Error(err, "Failed to create leader election manager", sdklog.ErrorCode("MANAGER_CREATE_ERROR"), sdklog.Operation("leader_manager_init"))
		os.Exit(1)
	}

	// Get elected channel
	var leaderElectedCh <-chan struct{}
	if mgrOpts.LeaderElection {
		leaderElectedCh = leaderManager.Elected()
	} else {
		// No leader election - create a channel that's immediately ready
		ch := make(chan struct{})
		close(ch)
		leaderElectedCh = ch
	}

	return leaderManager, leaderElectedCh
}

// configureLeaderElection configures leader election based on mode
func configureLeaderElection(namespace string, setupLog *sdklog.Logger) zenlead.LeaderElectionConfig {
	var leConfig zenlead.LeaderElectionConfig

	// Determine election ID (default if not provided)
	electionID := *leaderElectionID
	if electionID == "" {
		electionID = "zen-watcher-leader-election"
	}

	// Configure based on mode
	switch *leaderElectionMode {
	case "builtin":
		leConfig = zenlead.LeaderElectionConfig{
			Mode:       zenlead.BuiltIn,
			ElectionID: electionID,
			Namespace:  namespace,
		}
		setupLog.Info("Leader election mode: builtin", sdklog.Operation("leader_init"))
	case "disabled":
		leConfig = zenlead.LeaderElectionConfig{
			Mode: zenlead.Disabled,
		}
		setupLog.Info("Leader election disabled - single replica only (unsafe if replicas > 1)", sdklog.Operation("leader_init"))
	default:
		setupLog.Error(fmt.Errorf("invalid --leader-election-mode: %q (must be builtin or disabled)", *leaderElectionMode), "invalid configuration", sdklog.ErrorCode("INVALID_CONFIG"), sdklog.Operation("leader_init"), sdklog.String("mode", *leaderElectionMode))
		os.Exit(1)
	}

	return leConfig
}

// initializeAdapters initializes adapters and orchestrator
func initializeAdapters(clients *kubernetes.Clients, proc *processor.Processor, m *metrics.Metrics, gvrs *kubernetes.GVRs, observationCreator *watcher.ObservationCreator, setupLog *sdklog.Logger) (*orchestrator.GenericOrchestrator, *watcherconfig.IngesterStore, *watcherconfig.IngesterInformer, *server.Server) {
	// Create informer manager for generic adapters
	informerManager := informers.NewManager(informers.Config{
		DynamicClient: clients.Dynamic,
		DefaultResync: 0, // Watch-only, no periodic resync
	})

	// Create generic adapter factory with metrics support for webhook adapters
	genericAdapterFactory := generic.NewFactoryWithMetrics(
		informerManager,
		clients.Standard,
		m.WebhookRequests,
		m.WebhookDropped,
	)

	// Create HTTP server (webhook routes will be registered dynamically)
	httpServer := server.NewServer(m.WebhookRequests, m.WebhookDropped)

	// Set route registrar on factory so webhook adapters register routes on main server
	genericAdapterFactory.SetRouteRegistrar(httpServer.RegisterWebhookHandler)

	// Create GenericOrchestrator with metrics
	genericOrchestrator := orchestrator.NewGenericOrchestratorWithMetrics(
		genericAdapterFactory,
		clients.Dynamic,
		proc,
		m,
	)

	// Create Ingester store and informer
	ingesterStore := watcherconfig.NewIngesterStore()
	ingesterInformer := watcherconfig.NewIngesterInformer(ingesterStore, clients.Dynamic)

	// Set up GVR resolver for dynamic destination GVR support
	observationCreator.SetGVRResolver(func(source string) schema.GroupVersionResource {
		ingesterConfig, exists := ingesterStore.GetBySource(source)
		if exists && ingesterConfig != nil && len(ingesterConfig.Destinations) > 0 {
			for _, dest := range ingesterConfig.Destinations {
				if dest.Type == "crd" {
					return dest.GVR
				}
			}
		}
		return gvrs.Observations
	})

	return genericOrchestrator, ingesterStore, ingesterInformer, httpServer
}

// startAllServices starts all services
func startAllServices(ctx context.Context, wg *sync.WaitGroup, configManager *watcherconfig.ConfigManager, configMapLoader *watcherconfig.ConfigMapLoader, ingesterInformer *watcherconfig.IngesterInformer, genericOrchestrator *orchestrator.GenericOrchestrator, httpServer *server.Server, observationCreator *watcher.ObservationCreator, clients *kubernetes.Clients, gvrs *kubernetes.GVRs, m *metrics.Metrics, leaderElectedCh <-chan struct{}, filterInstance *filter.Filter, log *sdklog.Logger, setupLog *sdklog.Logger, leaderManager ctrl.Manager) {
	// Start leader election manager (required for leader election to work)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := leaderManager.Start(ctx); err != nil {
			setupLog.Error(err, "Leader election manager stopped", sdklog.Operation("leader_manager"))
		}
	}()
	// Create adapter factory and launcher
	adapterFactory := watcher.NewAdapterFactory(clients.Standard)
	adapters := adapterFactory.CreateAdapters()
	adapterLauncher := watcher.NewAdapterLauncher(adapters, observationCreator)

	// Create worker pool
	workerPool := dispatcher.NewWorkerPool(5, 10)

	// Register config change handler
	configManager.OnConfigChange(func(newConfig map[string]interface{}) {
		handleConfigChange(newConfig, workerPool, adapterLauncher, filterInstance, log)
	})

	// Start ConfigManager
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := configManager.Start(ctx); err != nil {
			setupLog.Error(err, "ConfigManager stopped", sdklog.Operation("config_manager"))
		}
	}()

	// Apply initial configuration
	initialConfig := configManager.GetConfigWithDefaults()
	handleConfigChange(initialConfig, workerPool, adapterLauncher, filterInstance, log)

	// Start HTTP server
	httpServer.Start(ctx, wg)
	httpServer.SetReady(true)

	// Start adapter launcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := adapterLauncher.Start(ctx); err != nil {
			setupLog.Error(err, "Adapter launcher stopped", sdklog.Operation("adapter_launcher"))
		}
	}()

	// Start ConfigMap loader
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := configMapLoader.Start(ctx); err != nil {
			setupLog.Error(err, "ConfigMap loader stopped", sdklog.Operation("configmap_loader"))
		}
	}()

	// Start Ingester informer
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ingesterInformer.Start(ctx); err != nil {
			setupLog.Error(err, "Ingester informer stopped", sdklog.Operation("ingester_informer"))
		}
	}()

	// Start GenericOrchestrator
	startGenericOrchestrator(ctx, wg, genericOrchestrator, leaderElectedCh, setupLog)

	// Start garbage collector
	startGarbageCollector(ctx, wg, clients, gvrs, m, leaderElectedCh, setupLog)

	// Start HA components
	startHAComponents(ctx, wg, observationCreator, m, httpServer, setupLog)

	// Log configuration
	setupLog.Info("zen-watcher ready", sdklog.Operation("startup_complete"))
	autoDetect := sdkconfig.RequireEnvWithDefault("AUTO_DETECT_ENABLED", "true")
	setupLog.Info("Configuration loaded",
		sdklog.Operation("config_load"),
		sdklog.String("auto_detect_enabled", autoDetect))
}

// startGenericOrchestrator starts the GenericOrchestrator
func startGenericOrchestrator(ctx context.Context, wg *sync.WaitGroup, genericOrchestrator *orchestrator.GenericOrchestrator, leaderElectedCh <-chan struct{}, setupLog *sdklog.Logger) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		setupLog.Info("Waiting for leader election before starting GenericOrchestrator", sdklog.Operation("generic_orchestrator"))
		select {
		case <-leaderElectedCh:
			setupLog.Info("Elected as leader, starting GenericOrchestrator", sdklog.Operation("generic_orchestrator"))
		case <-ctx.Done():
			return
		}
		if err := genericOrchestrator.Start(ctx); err != nil {
			setupLog.Error(err, "GenericOrchestrator stopped", sdklog.Operation("generic_orchestrator"))
		}
	}()
}

// startGarbageCollector starts the garbage collector
func startGarbageCollector(ctx context.Context, wg *sync.WaitGroup, clients *kubernetes.Clients, gvrs *kubernetes.GVRs, m *metrics.Metrics, leaderElectedCh <-chan struct{}, setupLog *sdklog.Logger) {
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
		setupLog.Info("Waiting for leader election before starting garbage collector", sdklog.Operation("gc"))
		select {
		case <-leaderElectedCh:
			setupLog.Info("Elected as leader, starting garbage collector", sdklog.Operation("gc"))
		case <-ctx.Done():
			return
		}
		gcCollector.Start(ctx)
	}()
}

// startHAComponents starts HA optimization components
func startHAComponents(ctx context.Context, wg *sync.WaitGroup, observationCreator *watcher.ObservationCreator, m *metrics.Metrics, httpServer *server.Server, setupLog *sdklog.Logger) {
	haConfig := watcherconfig.LoadHAConfig()
	if !haConfig.IsHAEnabled() {
		return
	}

	setupLog.Info("HA enabled, initializing HA components", sdklog.Operation("ha_init"))

	haMetrics := metrics.NewHAMetrics()
	replicaID := sdkconfig.RequireEnvWithDefault("HOSTNAME", fmt.Sprintf("replica-%d", time.Now().UnixNano()))

	var haScalingCoordinator *scaling.HPACoordinator
	var haLoadBalancer *balancer.LoadBalancer

	if haConfig.AutoScaling.Enabled {
		haScalingCoordinator = scaling.NewHPACoordinator(&haConfig.AutoScaling, haMetrics, replicaID)
		if haScalingCoordinator != nil {
			haScalingCoordinator.Start(ctx, 1*time.Minute)
			setupLog.Info("HA scaling coordinator started", sdklog.Operation("ha_scaling_init"))
		}
	}

	if haConfig.LoadBalancing.Strategy != "" {
		haLoadBalancer = balancer.NewLoadBalancer(&haConfig.LoadBalancing)
		setupLog.Info("HA load balancer initialized",
			sdklog.Operation("ha_balancer_init"),
			sdklog.String("strategy", haConfig.LoadBalancing.Strategy))
	}

	if haScalingCoordinator != nil || haLoadBalancer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startHAMetricsLoop(ctx, haScalingCoordinator, haLoadBalancer, observationCreator, httpServer, setupLog)
		}()
	}
}

// startHAMetricsLoop starts the HA metrics collection loop
func startHAMetricsLoop(ctx context.Context, haScalingCoordinator *scaling.HPACoordinator, haLoadBalancer *balancer.LoadBalancer, observationCreator *watcher.ObservationCreator, httpServer *server.Server, setupLog *sdklog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updateHAMetrics(haScalingCoordinator, haLoadBalancer, observationCreator, httpServer, setupLog)
		case <-ctx.Done():
			return
		}
	}
}

// updateHAMetrics updates HA metrics
func updateHAMetrics(haScalingCoordinator *scaling.HPACoordinator, haLoadBalancer *balancer.LoadBalancer, observationCreator *watcher.ObservationCreator, httpServer *server.Server, setupLog *sdklog.Logger) {
	// Collect real system metrics
	cpuUsage := systemMetrics.GetCPUUsagePercent()
	memoryUsage := float64(systemMetrics.GetMemoryUsage())
	eventsPerSec := systemMetrics.GetEventsPerSecond()
	queueDepth := systemMetrics.GetQueueDepth()
	responseTime := systemMetrics.GetResponseTime()

	// Log real metrics for debugging (if debug mode enabled)
	if sdkconfig.RequireEnvWithDefault("DEBUG_METRICS", "false") == "true" {
		setupLog.Debug("HA Metrics collected",
			sdklog.Operation("metrics_collection"),
			sdklog.Float64("cpu_usage", cpuUsage),
			sdklog.Float64("memory_usage", memoryUsage),
			sdklog.Float64("events_per_sec", eventsPerSec),
			sdklog.Float64("queue_depth", float64(queueDepth)),
			sdklog.Float64("response_time", responseTime))
	}

	// Update scaling coordinator
	if haScalingCoordinator != nil {
		haScalingCoordinator.UpdateMetrics(cpuUsage, memoryUsage, eventsPerSec, queueDepth, responseTime)
	}

	// Update load balancer
	if haLoadBalancer != nil {
		load := 0.0 // TODO: Calculate load factor
		replicaID := sdkconfig.RequireEnvWithDefault("HOSTNAME", fmt.Sprintf("replica-%d", time.Now().UnixNano()))
		haLoadBalancer.UpdateReplica(replicaID, load, cpuUsage, memoryUsage, eventsPerSec, true)
	}

	// Update HTTP server HA status
	httpServer.UpdateHAStatus(cpuUsage, memoryUsage, eventsPerSec, 0.0, queueDepth)

	// Update queue depth: webhook events are now handled generically via adapters
	// Queue depth is already calculated from systemMetrics above, so no additional update needed
}

// handleConfigChange handles configuration changes from ConfigMap
func handleConfigChange(
	newConfig map[string]interface{},
	workerPool *dispatcher.WorkerPool,
	adapterLauncher *watcher.AdapterLauncher,
	filterInstance *filter.Filter,
	log *sdklog.Logger,
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
				sdklog.Operation("config_update"),
				sdklog.Bool("enabled", newWorkerConfig.Enabled),
				sdklog.Int("size", newWorkerConfig.Size),
				sdklog.Int("queue_size", newWorkerConfig.QueueSize))
		} else {
			workerPool.Stop()
			adapterLauncher.SetWorkerPool(nil)
			log.Info("Worker pool disabled via configuration", sdklog.Operation("config_update"))
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
				sdklog.Operation("config_update"),
				sdklog.Bool("enabled", enabled),
				sdklog.Strings("included_namespaces", globalFilter.IncludedNamespaces),
				sdklog.Strings("excluded_namespaces", globalFilter.ExcludedNamespaces))
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
			log.Info("Namespace filtering disabled via configuration", sdklog.Operation("config_update"))
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

// getDuration parses a duration from config map
// nolint:unused // Kept for future use
func getDuration(config map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if strValue, ok := config[key].(string); ok {
		if duration, err := time.ParseDuration(strValue); err == nil {
			return duration
		}
	}
	return defaultValue
}
