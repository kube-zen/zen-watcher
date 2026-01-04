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

package watcher

import (
	"context"
	"sync"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"k8s.io/client-go/kubernetes"
)

// WorkerPoolInterface defines the interface for worker pool integration
// This avoids circular dependencies between packages
type WorkerPoolInterface interface {
	EnqueueBlocking(ctx context.Context, work interface{}) error
	Start()
	Stop()
}

// AdapterFactory creates SourceAdapter instances for all configured sources
// All sources are configured via Ingester CRDs and handled by the GenericOrchestrator.
type AdapterFactory struct {
	clientSet kubernetes.Interface
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(
	clientSet kubernetes.Interface,
) *AdapterFactory {
	return &AdapterFactory{
		clientSet: clientSet,
	}
}

// CreateAdapters creates all enabled source adapters
// All sources are configured via Ingester CRDs and handled by GenericOrchestrator.
func (af *AdapterFactory) CreateAdapters() []SourceAdapter {
	// All sources are now configured via Ingester CRDs and handled by GenericOrchestrator
	// which creates generic adapters (informer, webhook, logs) based on YAML config.
	return []SourceAdapter{}
}

// AdapterLauncher manages running all source adapters
type AdapterLauncher struct {
	adapters           []SourceAdapter
	observationCreator *ObservationCreator
	eventCh            chan *Event
	workerPool         WorkerPoolInterface
	useWorkerPool      bool
	adapterWg          sync.WaitGroup // Tracks adapter goroutines
}

// NewAdapterLauncher creates a new adapter launcher
func NewAdapterLauncher(
	adapters []SourceAdapter,
	observationCreator *ObservationCreator,
) *AdapterLauncher {
	// Create buffered channel for events
	eventCh := make(chan *Event, 1000)

	return &AdapterLauncher{
		adapters:           adapters,
		observationCreator: observationCreator,
		eventCh:            eventCh,
		useWorkerPool:      false, // Can be enabled via SetWorkerPool
	}
}

// SetWorkerPool sets the worker pool for async dispatch
func (al *AdapterLauncher) SetWorkerPool(workerPool WorkerPoolInterface) {
	al.workerPool = workerPool
	al.useWorkerPool = workerPool != nil
}

// Start starts all adapters and processes events
func (al *AdapterLauncher) Start(ctx context.Context) error {
	// Start worker pool if configured
	if al.useWorkerPool && al.workerPool != nil {
		al.workerPool.Start()
	}

	// Start all adapters
	for _, adapter := range al.adapters {
		adapter := adapter // Capture for goroutine
		al.adapterWg.Add(1)
		go func() {
			defer al.adapterWg.Done()
			if err := adapter.Run(ctx, al.eventCh); err != nil {
				logger := sdklog.NewLogger("zen-watcher")
				logger.Warn("Adapter stopped",
					sdklog.Operation("adapter_stopped"),
					sdklog.String("source", adapter.Name()),
					sdklog.Error(err))
			}
		}()
	}

	// Process events from all adapters
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-al.eventCh:
			// Use worker pool if enabled, otherwise process synchronously
			if al.useWorkerPool && al.workerPool != nil {
				// Create work item for async processing
				workItem := &eventWorkItem{
					event:              event,
					observationCreator: al.observationCreator,
				}
				if err := al.workerPool.EnqueueBlocking(ctx, workItem); err != nil {
					logger := sdklog.NewLogger("zen-watcher")
					logger.Warn("Failed to enqueue event for processing",
						sdklog.Operation("adapter_event_enqueue"),
						sdklog.String("source", event.Source),
						sdklog.Error(err))
				}
			} else {
				// Process synchronously (original behavior)
				al.processEvent(ctx, event)
			}
		}
	}
}

// processEvent processes a single event
func (al *AdapterLauncher) processEvent(ctx context.Context, event *Event) {
	// Convert Event to Observation and create via ObservationCreator
	observation := EventToObservation(event)
	if observation != nil {
		// Use centralized observation creator (handles filter, dedup, metrics)
		err := al.observationCreator.CreateObservation(ctx, observation)
		if err != nil {
			logger := sdklog.NewLogger("zen-watcher")
			logger.Warn("Failed to create Observation from adapter event",
				sdklog.Operation("adapter_observation_create"),
				sdklog.String("source", event.Source),
				sdklog.Error(err))
		}
	}
}

// eventWorkItem implements WorkItem interface for worker pool
type eventWorkItem struct {
	event              *Event
	observationCreator *ObservationCreator
}

// Process processes the event work item
func (w *eventWorkItem) Process(ctx context.Context) error {
	observation := EventToObservation(w.event)
	if observation != nil {
		return w.observationCreator.CreateObservation(ctx, observation)
	}
	return nil
}

// Stop stops all adapters gracefully
func (al *AdapterLauncher) Stop() {
	// Stop worker pool if configured
	if al.useWorkerPool && al.workerPool != nil {
		al.workerPool.Stop()
	}

	// Stop all adapters
	for _, adapter := range al.adapters {
		adapter.Stop()
	}
}
