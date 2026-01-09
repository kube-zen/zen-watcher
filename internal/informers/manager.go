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

package informers

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// Manager manages dynamic informer factory and provides informer access
// This abstraction centralizes informer construction and configuration
type Manager struct {
	dynamicClient dynamic.Interface
	factory       dynamicinformer.DynamicSharedInformerFactory
	defaultResync time.Duration
	// Track custom factories created for specific GVRs with custom resync periods
	customFactories   map[string]dynamicinformer.DynamicSharedInformerFactory
	customFactoriesMu sync.RWMutex
}

// Config holds configuration for the informer manager
type Config struct {
	DynamicClient dynamic.Interface
	DefaultResync time.Duration // Default resync period for all informers (0 = watch-only)
}

// NewManager creates a new informer manager
func NewManager(config Config) *Manager {
	if config.DefaultResync == 0 {
		// Default to watch-only (no periodic resync) like zen-agent
		config.DefaultResync = 0
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(
		config.DynamicClient,
		config.DefaultResync,
	)

	return &Manager{
		dynamicClient:   config.DynamicClient,
		factory:         factory,
		defaultResync:   config.DefaultResync,
		customFactories: make(map[string]dynamicinformer.DynamicSharedInformerFactory),
	}
}

// GetInformer returns a SharedIndexInformer for the given GVR
// If resyncPeriod is provided and > 0, it overrides the default resync period for this informer
func (m *Manager) GetInformer(gvr schema.GroupVersionResource, resyncPeriod time.Duration) cache.SharedIndexInformer {
	// If per-GVR resync is specified, create a new factory with that resync period
	// Otherwise use the default factory
	if resyncPeriod > 0 && resyncPeriod != m.defaultResync {
		// Create a separate factory for this GVR with custom resync
		// Use GVR string as key to track custom factories
		gvrKey := gvr.String()
		m.customFactoriesMu.Lock()
		customFactory, exists := m.customFactories[gvrKey]
		if !exists {
			customFactory = dynamicinformer.NewDynamicSharedInformerFactory(m.dynamicClient, resyncPeriod)
			m.customFactories[gvrKey] = customFactory
		}
		m.customFactoriesMu.Unlock()
		return customFactory.ForResource(gvr).Informer()
	}

	// Use default factory
	return m.factory.ForResource(gvr).Informer()
}

// GetFilteredInformer returns a SharedIndexInformer for the given GVR with namespace filtering
// This is useful for namespace-scoped resources to reduce watch overhead
func (m *Manager) GetFilteredInformer(
	gvr schema.GroupVersionResource,
	namespace string,
	resyncPeriod time.Duration,
	tweakListOptions func(*metav1.ListOptions),
) cache.SharedIndexInformer {
	// Use filtered factory for namespace-scoped resources
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		m.dynamicClient,
		resyncPeriod,
		namespace,
		tweakListOptions,
	)
	return factory.ForResource(gvr).Informer()
}

// Start starts all informers in the factory
// Also starts any custom factories that were created for specific GVRs
func (m *Manager) Start(ctx context.Context) {
	// Start default factory
	m.factory.Start(ctx.Done())

	// Start all custom factories
	m.customFactoriesMu.RLock()
	for _, customFactory := range m.customFactories {
		customFactory.Start(ctx.Done())
	}
	m.customFactoriesMu.RUnlock()
}

// WaitForCacheSync waits for all informer caches to sync
// Waits for both default factory and all custom factories
func (m *Manager) WaitForCacheSync(ctx context.Context) error {
	stopCh := ctx.Done()

	// Wait for default factory caches to sync
	synced := m.factory.WaitForCacheSync(stopCh)
	for gvr, ok := range synced {
		if !ok {
			return fmt.Errorf("failed to sync informer cache for %v", gvr)
		}
	}

	// Wait for all custom factory caches to sync
	m.customFactoriesMu.RLock()
	for gvrKey, customFactory := range m.customFactories {
		synced := customFactory.WaitForCacheSync(stopCh)
		for gvr, ok := range synced {
			if !ok {
				m.customFactoriesMu.RUnlock()
				return fmt.Errorf("failed to sync custom informer cache for %v (key: %s)", gvr, gvrKey)
			}
		}
	}
	m.customFactoriesMu.RUnlock()

	return nil
}

// GetDefaultResync returns the default resync period
func (m *Manager) GetDefaultResync() time.Duration {
	return m.defaultResync
}

// GetFactory returns the underlying factory
func (m *Manager) GetFactory() dynamicinformer.DynamicSharedInformerFactory {
	return m.factory
}
