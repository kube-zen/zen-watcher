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
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
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
		dynamicClient: config.DynamicClient,
		factory:       factory,
		defaultResync: config.DefaultResync,
	}
}

// GetInformer returns a SharedIndexInformer for the given GVR
// If resyncPeriod is provided and > 0, it overrides the default resync period for this informer
func (m *Manager) GetInformer(gvr schema.GroupVersionResource, resyncPeriod time.Duration) cache.SharedIndexInformer {
	// If per-GVR resync is specified, create a new factory with that resync period
	// Otherwise use the default factory
	if resyncPeriod > 0 && resyncPeriod != m.defaultResync {
		// Create a separate factory for this GVR with custom resync
		customFactory := dynamicinformer.NewDynamicSharedInformerFactory(m.dynamicClient, resyncPeriod)
		return customFactory.ForResource(gvr).Informer()
	}

	// Use default factory
	return m.factory.ForResource(gvr).Informer()
}

// Start starts all informers in the factory
func (m *Manager) Start(ctx context.Context) {
	m.factory.Start(ctx.Done())
}

// WaitForCacheSync waits for all informer caches to sync
func (m *Manager) WaitForCacheSync(ctx context.Context) error {
	stopCh := ctx.Done()
	synced := m.factory.WaitForCacheSync(stopCh)
	for gvr, ok := range synced {
		if !ok {
			return fmt.Errorf("failed to sync informer cache for %v", gvr)
		}
	}
	return nil
}

// GetFactory returns the underlying factory
func (m *Manager) GetFactory() dynamicinformer.DynamicSharedInformerFactory {
	return m.factory
}
