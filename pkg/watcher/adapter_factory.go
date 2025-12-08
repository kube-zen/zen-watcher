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

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
)

// AdapterFactory creates SourceAdapter instances for all configured sources
type AdapterFactory struct {
	factory         dynamicinformer.DynamicSharedInformerFactory
	policyReportGVR schema.GroupVersionResource
	trivyReportGVR  schema.GroupVersionResource
	clientSet       kubernetes.Interface
	falcoChan       chan map[string]interface{}
	auditChan       chan map[string]interface{}
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(
	factory dynamicinformer.DynamicSharedInformerFactory,
	policyReportGVR, trivyReportGVR schema.GroupVersionResource,
	clientSet kubernetes.Interface,
	falcoChan, auditChan chan map[string]interface{},
) *AdapterFactory {
	return &AdapterFactory{
		factory:         factory,
		policyReportGVR: policyReportGVR,
		trivyReportGVR:  trivyReportGVR,
		clientSet:       clientSet,
		falcoChan:       falcoChan,
		auditChan:       auditChan,
	}
}

// CreateAdapters creates all enabled source adapters
// This returns a slice of adapters that implement the SourceAdapter interface
func (af *AdapterFactory) CreateAdapters() []SourceAdapter {
	var adapters []SourceAdapter

	// First-class adapters (explicit, battle-tested)
	// Informer-based adapters
	adapters = append(adapters, NewTrivyAdapter(af.factory, af.trivyReportGVR))
	adapters = append(adapters, NewKyvernoAdapter(af.factory, af.policyReportGVR))

	// Webhook-based adapters
	if af.falcoChan != nil {
		adapters = append(adapters, NewFalcoAdapter(af.falcoChan))
	}
	if af.auditChan != nil {
		adapters = append(adapters, NewAuditAdapter(af.auditChan))
	}

	// ConfigMap-based adapters
	if af.clientSet != nil {
		adapters = append(adapters, NewKubeBenchAdapter(af.clientSet))
		adapters = append(adapters, NewCheckovAdapter(af.clientSet))
		// Native Kubernetes Events adapter (watching v1.Event API)
		adapters = append(adapters, NewK8sEventsAdapter(af.clientSet))
	}

	// Generic CRD adapter (for ObservationMapping CRDs - covers long tail of tools)
	adapters = append(adapters, NewCRDSourceAdapter(af.factory, ObservationMappingGVR))

	return adapters
}

// AdapterLauncher manages running all source adapters
type AdapterLauncher struct {
	adapters           []SourceAdapter
	observationCreator *ObservationCreator
	eventCh            chan *Event
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
	}
}

// Start starts all adapters and processes events
func (al *AdapterLauncher) Start(ctx context.Context) error {
	// Start all adapters
	for _, adapter := range al.adapters {
		adapter := adapter // Capture for goroutine
		go func() {
			if err := adapter.Run(ctx, al.eventCh); err != nil {
				logger.Warn("Adapter stopped",
					logger.Fields{
						Component: "watcher",
						Operation: "adapter_stopped",
						Source:    adapter.Name(),
						Error:     err,
					})
			}
		}()
	}

	// Process events from all adapters
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-al.eventCh:
			// Convert Event to Observation and create via ObservationCreator
			observation := EventToObservation(event)
			if observation != nil {
				// Use centralized observation creator (handles filter, dedup, metrics)
				err := al.observationCreator.CreateObservation(ctx, observation)
				if err != nil {
					logger.Warn("Failed to create Observation from adapter event",
						logger.Fields{
							Component: "watcher",
							Operation: "adapter_observation_create",
							Source:    event.Source,
							Error:     err,
						})
				}
			}
		}
	}
}

// Stop stops all adapters gracefully
func (al *AdapterLauncher) Stop() {
	for _, adapter := range al.adapters {
		adapter.Stop()
	}
}
