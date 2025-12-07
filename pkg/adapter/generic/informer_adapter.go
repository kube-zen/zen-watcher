// Copyright 2024 The Zen Watcher Authors
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
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// InformerAdapter handles ALL CRD-based sources via dynamic informers
type InformerAdapter struct {
	factory dynamicinformer.DynamicSharedInformerFactory
	stopCh  chan struct{}
}

// NewInformerAdapter creates a new generic informer adapter
func NewInformerAdapter(factory dynamicinformer.DynamicSharedInformerFactory) *InformerAdapter {
	return &InformerAdapter{
		factory: factory,
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
	if config.Informer.GVR.Group == "" || config.Informer.GVR.Version == "" || config.Informer.GVR.Resource == "" {
		return fmt.Errorf("informer.gvr.group, version, and resource are required")
	}
	return nil
}

// Start starts the informer adapter
func (a *InformerAdapter) Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}

	events := make(chan RawEvent, 100)

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
			logger.Warn("Invalid resync period, using default",
				logger.Fields{
					Component: "adapter",
					Operation: "informer_start",
					Source:    config.Source,
					Error:     err,
				})
		}
	}

	// Create filtered informer
	var informer cache.SharedIndexInformer
	if config.Informer.Namespace != "" {
		// Namespace-scoped
		informer = a.factory.ForResource(gvr).Informer()
	} else {
		// Cluster-scoped or all namespaces
		informer = a.factory.ForResource(gvr).Informer()
	}

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			event := RawEvent{
				Source:    config.Source,
				Timestamp: time.Now(),
				RawData:   u.Object, // ALL data preserved
				Metadata: map[string]interface{}{
					"event":     "add",
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
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			event := RawEvent{
				Source:    config.Source,
				Timestamp: time.Now(),
				RawData:   u.Object, // ALL data preserved
				Metadata: map[string]interface{}{
					"event":     "update",
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
				Source:    config.Source,
				Timestamp: time.Now(),
				RawData:   u.Object, // ALL data preserved
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
	})

	// Start informer
	a.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, fmt.Errorf("failed to sync informer cache for %s", gvr.String())
	}

	logger.Info("Informer adapter started",
		logger.Fields{
			Component: "adapter",
			Operation: "informer_start",
			Source:    config.Source,
			Additional: map[string]interface{}{
				"gvr":          gvr.String(),
				"namespace":    config.Informer.Namespace,
				"resyncPeriod": resyncPeriod.String(),
			},
		})

	return events, nil
}

// Stop stops the informer adapter
func (a *InformerAdapter) Stop() {
	close(a.stopCh)
}
