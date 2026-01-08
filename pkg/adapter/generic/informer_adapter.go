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

package generic

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/internal/informers"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// InformerAdapter handles ALL Kubernetes resources via dynamic informers (GVR-capable)
// Can watch any Kubernetes resource including ConfigMaps, CRDs, Pods, etc.
type InformerAdapter struct {
	manager *informers.Manager
	stopCh  chan struct{}
	events  chan RawEvent                                   // Event channel - must be closed in Stop() to prevent goroutine leaks
	queue   workqueue.TypedRateLimitingInterface[*RawEvent] // Internal queue for backpressure (future use)
	mu      sync.Mutex
}

// NewInformerAdapterWithManager creates a new informer adapter using the informer manager
func NewInformerAdapterWithManager(manager *informers.Manager) *InformerAdapter {
	return &InformerAdapter{
		manager: manager,
		stopCh:  make(chan struct{}),
	}
}

// Type returns the adapter type
func (a *InformerAdapter) Type() string {
	return "informer"
}

// Validate validates the informer configuration
func (a *InformerAdapter) Validate(config *SourceConfig) error {
	if config.Informer == nil {
		return fmt.Errorf("informer config is required for informer adapter")
	}
	// Group can be empty for core Kubernetes resources (e.g., events, pods)
	// Version and resource are required
	if config.Informer.GVR.Version == "" || config.Informer.GVR.Resource == "" {
		return fmt.Errorf("informer.gvr.version and resource are required")
	}
	return nil
}

// Start starts the informer adapter
func (a *InformerAdapter) Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}

	// Bounded output channel (smaller than queue to provide backpressure signal)
	// Store in struct so we can close it in Stop() to prevent goroutine leaks
	a.mu.Lock()
	a.events = make(chan RawEvent, 100)
	events := a.events
	a.mu.Unlock()

	// Parse GVR
	gvr := schema.GroupVersionResource{
		Group:    config.Informer.GVR.Group,
		Version:  config.Informer.GVR.Version,
		Resource: config.Informer.GVR.Resource,
	}

	// Parse resync period
	var resyncPeriod time.Duration
	if config.Informer.ResyncPeriod != "" && config.Informer.ResyncPeriod != "0" {
		var err error
		resyncPeriod, err = time.ParseDuration(config.Informer.ResyncPeriod)
		if err != nil {
			logger := sdklog.NewLogger("zen-watcher-adapter")
			logger.Warn("Invalid resync period, using default",
				sdklog.Operation("informer_start"),
				sdklog.String("source", config.Source),
				sdklog.Error(err))
		}
	}

	// Get informer from manager (GVR-capable - can watch any Kubernetes resource)
	informer := a.manager.GetInformer(gvr, resyncPeriod)

	// Add event handlers
	_, err := informer.AddEventHandler(a.createEventHandlers(ctx, events, config.Source, gvr))
	if err != nil {
		return nil, fmt.Errorf("failed to add event handlers: %w", err)
	}

	// Start informer manager
	a.manager.Start(ctx)
	// Wait for cache sync via manager
	if err := a.manager.WaitForCacheSync(ctx); err != nil {
		return nil, fmt.Errorf("failed to sync informer cache for %s: %w", gvr.String(), err)
	}

	logger := sdklog.NewLogger("zen-watcher-adapter")
	logger.Info("Informer adapter started",
		sdklog.Operation("informer_start"),
		sdklog.String("source", config.Source),
		sdklog.String("gvr", gvr.String()),
		sdklog.String("namespace", config.Informer.Namespace),
		sdklog.Duration("resync_period", resyncPeriod))

	return events, nil
}

// createEventHandlers creates event handlers for the informer
func (a *InformerAdapter) createEventHandlers(ctx context.Context, events chan<- RawEvent, source string, gvr schema.GroupVersionResource) cache.ResourceEventHandlerFuncs {
	logger := sdklog.NewLogger("zen-watcher-adapter")
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			event := RawEvent{
				Source:    source,
				Timestamp: time.Now(),
				RawData:   u.Object,
				Metadata: map[string]interface{}{
					"event":     "add",
					"namespace": u.GetNamespace(),
					"name":      u.GetName(),
					"gvr":       gvr.String(),
				},
			}
			logger.Debug("Informer event received",
				sdklog.Operation("informer_event_add"),
				sdklog.String("source", source),
				sdklog.String("namespace", u.GetNamespace()),
				sdklog.String("name", u.GetName()),
				sdklog.String("gvr", gvr.String()))
			select {
			case events <- event:
				logger.Debug("Event sent to channel",
					sdklog.Operation("informer_event_sent"),
					sdklog.String("source", source))
			case <-ctx.Done():
				return
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			event := RawEvent{
				Source:    source,
				Timestamp: time.Now(),
				RawData:   u.Object,
				Metadata: map[string]interface{}{
					"event":     "update",
					"namespace": u.GetNamespace(),
					"name":      u.GetName(),
					"gvr":       gvr.String(),
				},
			}
			logger.Debug("Informer event received (update)",
				sdklog.Operation("informer_event_update"),
				sdklog.String("source", source),
				sdklog.String("namespace", u.GetNamespace()),
				sdklog.String("name", u.GetName()))
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					u, ok = tombstone.Obj.(*unstructured.Unstructured)
					if !ok {
						return
					}
				} else {
					return
				}
			}
			event := RawEvent{
				Source:    source,
				Timestamp: time.Now(),
				RawData:   u.Object,
				Metadata: map[string]interface{}{
					"event":     "delete",
					"namespace": u.GetNamespace(),
					"name":      u.GetName(),
					"gvr":       gvr.String(),
				},
			}
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		},
	}
}

// processQueue processes items from the workqueue and emits RawEvents
// NOTE: This function is currently unused but kept for future queue-based processing
// nolint:unused
func (a *InformerAdapter) processQueue(ctx context.Context, events chan<- RawEvent, source string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			item, shutdown := a.queue.Get()
			if shutdown {
				return
			}

			// Check for nil item (workqueue can return nil in some edge cases)
			if item == nil {
				a.queue.Done(item)
				continue
			}

			// Process the event (no type assertion needed with TypedRateLimitingInterface)
			event := *item

			// Emit to channel (with context cancellation check)
			select {
			case events <- event:
				a.queue.Done(item)
			case <-ctx.Done():
				a.queue.Done(item)
				return
			}
		}
	}
}

// Stop stops the informer adapter
func (a *InformerAdapter) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Use select to prevent double close panic
	select {
	case <-a.stopCh:
		// Already closed
	default:
		close(a.stopCh)
	}

	// Close events channel to unblock processEvents goroutine and prevent leak
	if a.events != nil {
		close(a.events)
		a.events = nil
	}

	if a.queue != nil {
		a.queue.ShutDown()
	}
}
