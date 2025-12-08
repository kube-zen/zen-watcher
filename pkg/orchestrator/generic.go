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
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/kube-zen/zen-watcher/pkg/processor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

// GenericOrchestrator manages generic adapters based on ObservationSourceConfig CRDs
type GenericOrchestrator struct {
	adapterFactory     *generic.Factory
	dynClient          dynamic.Interface
	processor          *processor.Processor
	activeAdapters     map[string]generic.GenericAdapter // source -> adapter
	activeConfigs      map[string]*generic.SourceConfig   // source -> config
	activeEventStreams map[string]<-chan generic.RawEvent // source -> event stream
	mu                 sync.RWMutex
	ctx                context.Context
	cancel             context.CancelFunc
}

// NewGenericOrchestrator creates a new generic adapter orchestrator
func NewGenericOrchestrator(
	adapterFactory *generic.Factory,
	dynClient dynamic.Interface,
	proc *processor.Processor,
) *GenericOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	return &GenericOrchestrator{
		adapterFactory:     adapterFactory,
		dynClient:          dynClient,
		processor:          proc,
		activeAdapters:     make(map[string]generic.GenericAdapter),
		activeConfigs:      make(map[string]*generic.SourceConfig),
		activeEventStreams: make(map[string]<-chan generic.RawEvent),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start starts the orchestrator and watches for ObservationSourceConfig changes
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

// watchConfigChanges watches for ObservationSourceConfig changes
func (o *GenericOrchestrator) watchConfigChanges(ctx context.Context) {
	// This would integrate with SourceConfigLoader's informer
	// For now, we'll poll periodically
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

// reloadAdapters reloads all adapters based on current ObservationSourceConfigs
func (o *GenericOrchestrator) reloadAdapters() {
	// Load all ObservationSourceConfig CRDs
	configs, err := o.dynClient.Resource(config.ObservationSourceConfigGVR).List(o.ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to list ObservationSourceConfigs",
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
		// Convert to generic.SourceConfig
		genericConfig, err := config.ConvertToGenericSourceConfig(&item)
		if err != nil {
			logger.Warn("Failed to convert source config",
				logger.Fields{
					Component: "orchestrator",
					Operation: "convert_config",
					Source:    genericConfig.Source,
					Error:     err,
				})
			continue
		}

		source := genericConfig.Source
		activeSources[source] = true

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
		adapter, err := o.adapterFactory.NewAdapter(genericConfig.AdapterType)
		if err != nil {
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
			logger.Error("Failed to start adapter",
				logger.Fields{
					Component: "orchestrator",
					Operation: "start_adapter",
					Source:    source,
					Error:     err,
				})
			continue
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
					"adapterType": genericConfig.AdapterType,
				},
			})
	}

		// Stop adapters for sources that no longer exist
	for source, adapter := range o.activeAdapters {
		if !activeSources[source] {
			adapter.Stop()
			delete(o.activeAdapters, source)
			delete(o.activeConfigs, source)
			delete(o.activeEventStreams, source)
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
	for rawEvent := range events {
		if err := o.processor.ProcessEvent(o.ctx, &rawEvent, config); err != nil {
			logger.Error("Failed to process event",
				logger.Fields{
					Component: "orchestrator",
					Operation: "process_event",
					Source:    source,
					Error:     err,
				})
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
	return oldConfig.AdapterType != newConfig.AdapterType
}

// Stop stops all adapters
func (o *GenericOrchestrator) Stop() {
	o.cancel()
	o.mu.Lock()
	defer o.mu.Unlock()
	for source, adapter := range o.activeAdapters {
		adapter.Stop()
		delete(o.activeAdapters, source)
		delete(o.activeConfigs, source)
		delete(o.activeEventStreams, source)
	}
}

