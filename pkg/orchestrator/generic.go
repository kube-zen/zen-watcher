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
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
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
	}
}

// Start starts the orchestrator and watches for Ingester CRD changes
func (o *GenericOrchestrator) Start(ctx context.Context) error {
	logger.Info("Starting generic adapter orchestrator",
		logger.Fields{
			Component: "orchestrator",
			Operation: "start",
		})

	// Load initial configs
	o.reloadAdapters()

	// Watch for changes
	go o.watchConfigChanges(ctx)

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
		logger.Error("Failed to list Ingester CRDs",
			logger.Fields{
				Component: "orchestrator",
				Operation: "reload_adapters",
				Error:     err,
			})
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// Track which sources are still active
	activeSources := make(map[string]bool)

	// Start/update adapters for each config
	for _, item := range configs.Items {
		// Convert Ingester CRD to IngesterConfig
		// Create IngesterInformer instance to access conversion method
		store := config.NewIngesterStore()
		ii := config.NewIngesterInformer(store, o.dynClient)
		ingesterConfig := ii.ConvertToIngesterConfig(&item)
		if ingesterConfig == nil {
			logger.Warn("Failed to convert Ingester CRD",
				logger.Fields{
					Component: "orchestrator",
					Operation: "convert_ingester",
					Additional: map[string]interface{}{
						"name":      item.GetName(),
						"namespace": item.GetNamespace(),
					},
				})
			continue
		}

		// Convert IngesterConfig to generic.SourceConfig
		genericConfig := config.ConvertIngesterConfigToGeneric(ingesterConfig)
		if genericConfig == nil {
			if o.metrics != nil {
				o.metrics.IngestersConfigErrors.WithLabelValues(ingesterConfig.Source, "convert_to_generic_failed").Inc()
			}
			logger.Warn("Failed to convert IngesterConfig to generic.SourceConfig",
				logger.Fields{
					Component: "orchestrator",
					Operation: "convert_to_generic",
					Source:    ingesterConfig.Source,
				})
			continue
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
			// Check if config changed
			if o.configChanged(source, genericConfig) {
				// Stop old adapter
				existingAdapter.Stop()
				delete(o.activeAdapters, source)
			} else {
				// Config unchanged, skip
				continue
			}
		}

		// Create new adapter
		startupStartTime := time.Now()
		adapter, err := o.adapterFactory.NewAdapter(genericConfig.Ingester)
		if err != nil {
			if o.metrics != nil {
				o.metrics.IngestersConfigErrors.WithLabelValues(source, "create_adapter_failed").Inc()
				o.metrics.IngestersStatus.WithLabelValues(source).Set(-1) // -1 = error
			}
			logger.Error("Failed to create adapter",
				logger.Fields{
					Component: "orchestrator",
					Operation: "create_adapter",
					Source:    source,
					Error:     err,
				})
			continue
		}

		// Validate config
		if err := adapter.Validate(genericConfig); err != nil {
			if o.metrics != nil {
				o.metrics.IngestersConfigErrors.WithLabelValues(source, "validation_failed").Inc()
				o.metrics.IngestersStatus.WithLabelValues(source).Set(-1) // -1 = error
			}
			logger.Error("Adapter validation failed",
				logger.Fields{
					Component: "orchestrator",
					Operation: "validate_adapter",
					Source:    source,
					Error:     err,
				})
			continue
		}

		// Start adapter
		events, err := adapter.Start(o.ctx, genericConfig)
		if err != nil {
			if o.metrics != nil {
				o.metrics.IngestersConfigErrors.WithLabelValues(source, "start_failed").Inc()
				o.metrics.IngestersStatus.WithLabelValues(source).Set(-1) // -1 = error
			}
			logger.Error("Failed to start adapter",
				logger.Fields{
					Component: "orchestrator",
					Operation: "start_adapter",
					Source:    source,
					Error:     err,
				})
			continue
		}

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

		logger.Info("Generic adapter started",
			logger.Fields{
				Component: "orchestrator",
				Operation: "adapter_started",
				Source:    source,
				Additional: map[string]interface{}{
					"ingester": genericConfig.Ingester,
				},
			})
	}

	// Stop adapters for sources that no longer exist
	for source, adapter := range o.activeAdapters {
		if !activeSources[source] {
			// Get config before deleting
			config := o.activeConfigs[source]
			adapter.Stop()
			delete(o.activeAdapters, source)
			delete(o.activeConfigs, source)
			delete(o.activeEventStreams, source)
			if o.metrics != nil {
				// Get ingester type from config before deletion
				if config != nil {
					// Find namespace from original CRD (we'd need to track this)
					// For now, use empty namespace
					o.metrics.IngestersActive.WithLabelValues(source, config.Ingester, "").Set(0)
				}
				o.metrics.IngestersStatus.WithLabelValues(source).Set(0) // 0 = inactive
			}
			logger.Info("Generic adapter stopped",
				logger.Fields{
					Component: "orchestrator",
					Operation: "adapter_stopped",
					Source:    source,
				})
		}
	}
}

// processEvents processes RawEvents from an adapter
func (o *GenericOrchestrator) processEvents(source string, config *generic.SourceConfig, events <-chan generic.RawEvent) {
	eventCount := 0
	lastRateUpdate := time.Now()

	if o.enableBatching && o.batchProcessor != nil {
		// Use batch processing for improved throughput
		for rawEvent := range events {
			processStart := time.Now()
			if err := o.batchProcessor.AddEvent(o.ctx, &rawEvent, config); err != nil {
				if o.metrics != nil {
					o.metrics.IngesterErrorsTotal.WithLabelValues(source, "batch_add_failed", "batch").Inc()
				}
				logger.Error("Failed to add event to batch",
					logger.Fields{
						Component: "orchestrator",
						Operation: "batch_add_event",
						Source:    source,
						Error:     err,
					})
			} else {
				eventCount++
				if o.metrics != nil {
					o.metrics.IngesterEventsProcessed.WithLabelValues(source, config.Ingester).Inc()
					o.metrics.IngesterProcessingLatency.WithLabelValues(source, "batch").Observe(time.Since(processStart).Seconds())

					// Update last event timestamp
					now := time.Now()
					o.lastEventMu.Lock()
					o.lastEventTimestamp[source] = now
					o.lastEventMu.Unlock()
					o.metrics.IngestersLastEventTimestamp.WithLabelValues(source).Set(float64(now.Unix()))

					// Update rate every second
					if time.Since(lastRateUpdate) >= time.Second {
						rate := float64(eventCount) / time.Since(lastRateUpdate).Seconds()
						o.metrics.IngesterEventsProcessedRate.WithLabelValues(source).Set(rate)
						eventCount = 0
						lastRateUpdate = time.Now()
					}
				}
			}
		}
	} else {
		// Process events one-by-one (original behavior)
		for rawEvent := range events {
			processStart := time.Now()
			if err := o.processor.ProcessEvent(o.ctx, &rawEvent, config); err != nil {
				if o.metrics != nil {
					o.metrics.IngesterErrorsTotal.WithLabelValues(source, "process_failed", "processor").Inc()
				}
				logger.Error("Failed to process event",
					logger.Fields{
						Component: "orchestrator",
						Operation: "process_event",
						Source:    source,
						Error:     err,
					})
			} else {
				eventCount++
				if o.metrics != nil {
					o.metrics.IngesterEventsProcessed.WithLabelValues(source, config.Ingester).Inc()
					o.metrics.IngesterProcessingLatency.WithLabelValues(source, "processor").Observe(time.Since(processStart).Seconds())

					// Update last event timestamp
					now := time.Now()
					o.lastEventMu.Lock()
					o.lastEventTimestamp[source] = now
					o.lastEventMu.Unlock()
					o.metrics.IngestersLastEventTimestamp.WithLabelValues(source).Set(float64(now.Unix()))

					// Update rate every second
					if time.Since(lastRateUpdate) >= time.Second {
						rate := float64(eventCount) / time.Since(lastRateUpdate).Seconds()
						o.metrics.IngesterEventsProcessedRate.WithLabelValues(source).Set(rate)
						eventCount = 0
						lastRateUpdate = time.Now()
					}
				}
			}
		}
	}
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
