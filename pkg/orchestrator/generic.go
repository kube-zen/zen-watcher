// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may Obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orchestrator

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/config"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

// GenericOrchestrator manages generic adapters based on Ingester CRDs
type GenericOrchestrator struct {
	adapterFactory     *generic.Factory
	dynClient          dynamic.Interface
	processor          *processor.Processor
	batchProcessor     *processor.BatchProcessor          // Optional: for batch processing
	activeAdapters     map[string]generic.GenericAdapter  // source -> adapter
	activeConfigs      map[string]*generic.SourceConfig   // source -> config
	activeEventStreams map[string]<-chan generic.RawEvent // source -> event stream
	mu                 sync.RWMutex
	ctx                context.Context
	cancel             context.CancelFunc
	enableBatching     bool             // Whether to use batch processing
	metrics            *metrics.Metrics // Optional metrics
	// Track last event timestamp per source
	lastEventTimestamp map[string]time.Time
	lastEventMu        sync.RWMutex
	// Status updater for tracking source status
	statusUpdater      *IngesterStatusUpdater
}

// NewGenericOrchestrator creates a new generic adapter orchestrator
func NewGenericOrchestrator(
	adapterFactory *generic.Factory,
	dynClient dynamic.Interface,
	proc *processor.Processor,
) *GenericOrchestrator {
	return NewGenericOrchestratorWithMetrics(adapterFactory, dynClient, proc, nil)
}

// NewGenericOrchestratorWithMetrics creates a new generic adapter orchestrator with metrics
func NewGenericOrchestratorWithMetrics(
	adapterFactory *generic.Factory,
	dynClient dynamic.Interface,
	proc *processor.Processor,
	m *metrics.Metrics,
) *GenericOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	// Enable batch processing by default for performance optimization
	// Batch size: 15 events, max age: 100ms (balances throughput and latency)
	enableBatching := true
	maxBatchSize := 15
	maxBatchAge := 100 * time.Millisecond

	var batchProcessor *processor.BatchProcessor
	if enableBatching {
		batchProcessor = processor.NewBatchProcessor(proc, maxBatchSize, maxBatchAge)
	}

	return &GenericOrchestrator{
		adapterFactory:     adapterFactory,
		dynClient:          dynClient,
		processor:          proc,
		batchProcessor:     batchProcessor,
		activeAdapters:     make(map[string]generic.GenericAdapter),
		activeConfigs:      make(map[string]*generic.SourceConfig),
		activeEventStreams: make(map[string]<-chan generic.RawEvent),
		ctx:                ctx,
		cancel:             cancel,
		enableBatching:     enableBatching,
		metrics:            m,
		lastEventTimestamp: make(map[string]time.Time),
		statusUpdater:      NewIngesterStatusUpdater(dynClient),
	}
}

// Start starts the orchestrator and watches for Ingester CRD changes
func (o *GenericOrchestrator) Start(ctx context.Context) error {
	logger := sdklog.NewLogger("zen-watcher-orchestrator")
	logger.Info("Starting generic adapter orchestrator",
		sdklog.Operation("start"))

	// Load initial configs
	o.reloadAdapters()

	// Watch for changes
	go o.watchConfigChanges(ctx)

	// Start status update loop
	go o.statusUpdateLoop(ctx)

	return nil
}

// watchConfigChanges watches for Ingester CRD changes
func (o *GenericOrchestrator) watchConfigChanges(ctx context.Context) {
	// Poll periodically for Ingester CRD changes
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.reloadAdapters()
		}
	}
}

// reloadAdapters reloads all adapters based on current Ingester CRDs
func (o *GenericOrchestrator) reloadAdapters() {
	// Load all Ingester CRDs
	configs, err := o.dynClient.Resource(config.IngesterGVR).List(o.ctx, metav1.ListOptions{})
	if err != nil {
		logger := sdklog.NewLogger("zen-watcher-orchestrator")
		logger.Error(err, "Failed to list Ingester CRDs",
			sdklog.Operation("reload_adapters"))
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// Track which sources are still active
	activeSources := make(map[string]bool)

	// Start/update adapters for each config
	for _, item := range configs.Items {
		// Convert Ingester CRD to IngesterConfig(s) - supports multi-source
		// Create IngesterInformer instance to access conversion method
		store := config.NewIngesterStore()
		ii := config.NewIngesterInformer(store, o.dynClient)
		ingesterConfigs := ii.ConvertToIngesterConfigs(&item)
		if len(ingesterConfigs) == 0 {
			logger := sdklog.NewLogger("zen-watcher-orchestrator")
			logger.Warn("Failed to convert Ingester CRD",
				sdklog.Operation("convert_ingester"),
				sdklog.String("name", item.GetName()),
				sdklog.String("namespace", item.GetNamespace()))
			continue
		}

		// Process each config (one per source in multi-source mode)
		for _, ingesterConfig := range ingesterConfigs {
			o.processIngesterConfigItem(ingesterConfig, item, activeSources)
		}
	}

	// Stop adapters for sources that no longer exist
	for source, adapter := range o.activeAdapters {
		if !activeSources[source] {
			// Get config before deleting
			config := o.activeConfigs[source]
			// Extract namespace/name from source (format: namespace/name/sourceName)
			namespace, name, sourceName := o.parseSourceIdentifier(source)
			if namespace != "" && name != "" {
				// Update status: stopped
				tracker := o.statusUpdater.GetOrCreateTracker(namespace, name)
				if config != nil {
					tracker.UpdateSourceState(sourceName, config.Ingester, SourceStateStopped, nil)
				}
			}
			adapter.Stop()
			delete(o.activeAdapters, source)
			delete(o.activeConfigs, source)
			delete(o.activeEventStreams, source)
			if o.metrics != nil {
				// Get ingester type from config before deletion
				if config != nil {
					o.metrics.IngestersActive.WithLabelValues(source, config.Ingester, namespace).Set(0)
				}
				o.metrics.IngestersStatus.WithLabelValues(source).Set(0) // 0 = inactive
			}
			logger := sdklog.NewLogger("zen-watcher-orchestrator")
			logger.Info("Generic adapter stopped",
				sdklog.Operation("adapter_stopped"),
				sdklog.String("source", source))
		}
	}
}

// processEvents processes RawEvents from an adapter
func (o *GenericOrchestrator) processEvents(source string, config *generic.SourceConfig, events <-chan generic.RawEvent) {
	eventCount := 0
	lastRateUpdate := time.Now()

	if o.enableBatching && o.batchProcessor != nil {
		o.processBatchEvents(source, config, events, &eventCount, &lastRateUpdate)
	} else {
		o.processSingleEvents(source, config, events, &eventCount, &lastRateUpdate)
	}
}

// processBatchEvents processes events using batch processor
func (o *GenericOrchestrator) processBatchEvents(source string, config *generic.SourceConfig, events <-chan generic.RawEvent, eventCount *int, lastRateUpdate *time.Time) {
	for rawEvent := range events {
		processStart := time.Now()
		if err := o.batchProcessor.AddEvent(o.ctx, &rawEvent, config); err != nil {
			if o.metrics != nil {
				o.metrics.IngesterErrorsTotal.WithLabelValues(source, "batch_add_failed", "batch").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-orchestrator")
			logger.Error(err, "Failed to add event to batch",
				sdklog.Operation("batch_add_event"),
				sdklog.String("source", source))
		} else {
			o.updateEventMetrics(source, config.Ingester, processStart, eventCount, lastRateUpdate, "batch")
		}
	}
}

// processSingleEvents processes events one-by-one
func (o *GenericOrchestrator) processSingleEvents(source string, config *generic.SourceConfig, events <-chan generic.RawEvent, eventCount *int, lastRateUpdate *time.Time) {
	for rawEvent := range events {
		processStart := time.Now()
		if err := o.processor.ProcessEvent(o.ctx, &rawEvent, config); err != nil {
			if o.metrics != nil {
				o.metrics.IngesterErrorsTotal.WithLabelValues(source, "process_failed", "processor").Inc()
			}
			logger := sdklog.NewLogger("zen-watcher-orchestrator")
			logger.Error(err, "Failed to process event",
				sdklog.Operation("process_event"),
				sdklog.String("source", source))
		} else {
			o.updateEventMetrics(source, config.Ingester, processStart, eventCount, lastRateUpdate, "processor")
			// Update source lastSeen in status
			namespace, name, sourceName := o.parseSourceIdentifier(source)
			if namespace != "" && name != "" {
				tracker := o.statusUpdater.GetOrCreateTracker(namespace, name)
				tracker.UpdateSourceLastSeen(sourceName)
			}
		}
	}
}

// updateEventMetrics updates event processing metrics
func (o *GenericOrchestrator) updateEventMetrics(source, ingester string, processStart time.Time, eventCount *int, lastRateUpdate *time.Time, processorType string) {
	*eventCount++
	if o.metrics != nil {
		o.metrics.IngesterEventsProcessed.WithLabelValues(source, ingester).Inc()
		o.metrics.IngesterProcessingLatency.WithLabelValues(source, processorType).Observe(time.Since(processStart).Seconds())

		// Update last event timestamp
		now := time.Now()
		o.lastEventMu.Lock()
		o.lastEventTimestamp[source] = now
		o.lastEventMu.Unlock()
		o.metrics.IngestersLastEventTimestamp.WithLabelValues(source).Set(float64(now.Unix()))

		// Update rate every second
		if time.Since(*lastRateUpdate) >= time.Second {
			rate := float64(*eventCount) / time.Since(*lastRateUpdate).Seconds()
			o.metrics.IngesterEventsProcessedRate.WithLabelValues(source).Set(rate)
			*eventCount = 0
			*lastRateUpdate = time.Now()
		}
	}
}

// processIngesterConfigItem processes a single ingester config item
func (o *GenericOrchestrator) processIngesterConfigItem(ingesterConfig *config.IngesterConfig, item *unstructured.Unstructured, activeSources map[string]bool) {
	// Convert IngesterConfig to generic.SourceConfig
	genericConfig := config.ConvertIngesterConfigToGeneric(ingesterConfig)
	if genericConfig == nil {
		if o.metrics != nil {
			o.metrics.IngestersConfigErrors.WithLabelValues(ingesterConfig.Source, "convert_to_generic_failed").Inc()
		}
		logger := sdklog.NewLogger("zen-watcher-orchestrator")
		logger.Warn("Failed to convert IngesterConfig to generic.SourceConfig",
			sdklog.Operation("convert_to_generic"),
			sdklog.String("source", ingesterConfig.Source))
		return
	}

	source := genericConfig.Source
	activeSources[source] = true

	// Update ingester active status
	if o.metrics != nil {
		o.metrics.IngestersActive.WithLabelValues(source, genericConfig.Ingester, item.GetNamespace()).Set(1)
		o.metrics.IngestersStatus.WithLabelValues(source).Set(1) // 1 = active
	}

	// Check if adapter already exists
	if existingAdapter, exists := o.activeAdapters[source]; exists {
		if o.configChanged(source, genericConfig) {
			existingAdapter.Stop()
			delete(o.activeAdapters, source)
		} else {
			return // Config unchanged, skip
		}
	}

	// Extract source name and create/start adapter
	sourceName := o.extractSourceName(source, item.GetNamespace(), item.GetName())
	if o.createAndStartAdapterForSource(source, sourceName, genericConfig, item) {
		logger := sdklog.NewLogger("zen-watcher-orchestrator")
		logger.Info("Generic adapter started",
			sdklog.Operation("adapter_started"),
			sdklog.String("source", source),
			sdklog.String("ingester", genericConfig.Ingester),
			sdklog.String("namespace", item.GetNamespace()),
			sdklog.String("name", item.GetName()))
	}
}

// createAndStartAdapterForSource creates and starts an adapter for a source
func (o *GenericOrchestrator) createAndStartAdapterForSource(source, sourceName string, genericConfig *generic.SourceConfig, item *unstructured.Unstructured) bool {
	startupStartTime := time.Now()
	adapter, err := o.adapterFactory.NewAdapter(genericConfig.Ingester)
	if err != nil {
		o.handleAdapterError(source, sourceName, genericConfig, item, err, "create_adapter_failed")
		return false
	}

	if err := adapter.Validate(genericConfig); err != nil {
		o.handleAdapterError(source, sourceName, genericConfig, item, err, "validation_failed")
		return false
	}

	events, err := adapter.Start(o.ctx, genericConfig)
	if err != nil {
		o.handleAdapterError(source, sourceName, genericConfig, item, err, "start_failed")
		return false
	}

	// Update status: running
	tracker := o.statusUpdater.GetOrCreateTracker(item.GetNamespace(), item.GetName())
	tracker.UpdateSourceState(sourceName, genericConfig.Ingester, SourceStateRunning, nil)

	// Record startup duration
	if o.metrics != nil {
		o.metrics.IngestersStartupDuration.WithLabelValues(source).Observe(time.Since(startupStartTime).Seconds())
	}

	// Store adapter, config, and event stream
	o.activeAdapters[source] = adapter
	o.activeConfigs[source] = genericConfig
	o.activeEventStreams[source] = events

	// Process events from this adapter
	go o.processEvents(source, genericConfig, events)

	return true
}

// handleAdapterError handles adapter creation/validation/start errors
func (o *GenericOrchestrator) handleAdapterError(source, sourceName string, genericConfig *generic.SourceConfig, item *unstructured.Unstructured, err error, errorType string) {
	tracker := o.statusUpdater.GetOrCreateTracker(item.GetNamespace(), item.GetName())
	tracker.UpdateSourceState(sourceName, genericConfig.Ingester, SourceStateError, err)
	if o.metrics != nil {
		o.metrics.IngestersConfigErrors.WithLabelValues(source, errorType).Inc()
		o.metrics.IngestersStatus.WithLabelValues(source).Set(-1) // -1 = error
	}
	logger := sdklog.NewLogger("zen-watcher-orchestrator")
	logger.Error(err, "Adapter operation failed",
		sdklog.Operation(errorType),
		sdklog.String("source", source))
}

// configChanged checks if config has changed
func (o *GenericOrchestrator) configChanged(source string, newConfig *generic.SourceConfig) bool {
	oldConfig, exists := o.activeConfigs[source]
	if !exists {
		return true // New config
	}
	// Simplified comparison - in production, would do deep comparison
	return oldConfig.Ingester != newConfig.Ingester
}

// Stop stops all adapters
func (o *GenericOrchestrator) Stop() {
	o.cancel()
	if o.batchProcessor != nil {
		o.batchProcessor.Stop()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for source, adapter := range o.activeAdapters {
		adapter.Stop()
		delete(o.activeAdapters, source)
		delete(o.activeConfigs, source)
		delete(o.activeEventStreams, source)
	}
}

// extractSourceName extracts the source name from a source identifier
// Source format: namespace/name/sourceName (multi-source) or legacy source string
func (o *GenericOrchestrator) extractSourceName(source, namespace, name string) string {
	// Check if source is in multi-source format: namespace/name/sourceName
	expectedPrefix := namespace + "/" + name + "/"
	if len(source) > len(expectedPrefix) && source[:len(expectedPrefix)] == expectedPrefix {
		return source[len(expectedPrefix):]
	}
	// Legacy mode: use source as-is
	return source
}

// parseSourceIdentifier parses a source identifier into namespace, name, and sourceName
// Returns empty strings if format is not recognized
func (o *GenericOrchestrator) parseSourceIdentifier(source string) (namespace, name, sourceName string) {
	// Format: namespace/name/sourceName (multi-source)
	parts := strings.Split(source, "/")
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	// Legacy format: just return empty strings (can't extract)
	return "", "", source
}

// statusUpdateLoop periodically updates Ingester status
func (o *GenericOrchestrator) statusUpdateLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.updateAllIngesterStatus(ctx)
		}
	}
}

// updateAllIngesterStatus updates status for all tracked Ingesters
func (o *GenericOrchestrator) updateAllIngesterStatus(ctx context.Context) {
	o.mu.RLock()
	ingesterMap := make(map[string]bool) // namespace/name -> true
	for source := range o.activeAdapters {
		namespace, name, _ := o.parseSourceIdentifier(source)
		if namespace != "" && name != "" {
			ingesterMap[namespace+"/"+name] = true
		}
	}
	o.mu.RUnlock()

	for key := range ingesterMap {
		parts := strings.Split(key, "/")
		if len(parts) == 2 {
			if err := o.statusUpdater.UpdateStatus(ctx, parts[0], parts[1]); err != nil {
				logger := sdklog.NewLogger("zen-watcher-orchestrator")
				logger.Warn("Failed to update Ingester status",
					sdklog.Operation("update_status"),
					sdklog.String("namespace", parts[0]),
					sdklog.String("name", parts[1]),
					sdklog.Error(err))
			}
		}
	}
}
