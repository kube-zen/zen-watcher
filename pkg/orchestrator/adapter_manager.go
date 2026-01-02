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

package orchestrator

import (
	"sync"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
)

// AdapterManager manages the lifecycle of generic adapters
type AdapterManager struct {
	activeAdapters     map[string]generic.GenericAdapter
	activeConfigs      map[string]*generic.SourceConfig
	activeEventStreams map[string]<-chan generic.RawEvent
	mu                 sync.RWMutex
}

// NewAdapterManager creates a new adapter manager
func NewAdapterManager() *AdapterManager {
	return &AdapterManager{
		activeAdapters:     make(map[string]generic.GenericAdapter),
		activeConfigs:      make(map[string]*generic.SourceConfig),
		activeEventStreams: make(map[string]<-chan generic.RawEvent),
	}
}

// GetAdapter returns an adapter by source
func (am *AdapterManager) GetAdapter(source string) (generic.GenericAdapter, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	adapter, ok := am.activeAdapters[source]
	return adapter, ok
}

// GetConfig returns a config by source
func (am *AdapterManager) GetConfig(source string) (*generic.SourceConfig, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	config, ok := am.activeConfigs[source]
	return config, ok
}

// GetEventStream returns an event stream by source
func (am *AdapterManager) GetEventStream(source string) (<-chan generic.RawEvent, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	stream, ok := am.activeEventStreams[source]
	return stream, ok
}

// AddAdapter adds an adapter with its config and event stream
func (am *AdapterManager) AddAdapter(source string, adapter generic.GenericAdapter, config *generic.SourceConfig, eventStream <-chan generic.RawEvent) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.activeAdapters[source] = adapter
	am.activeConfigs[source] = config
	am.activeEventStreams[source] = eventStream
}

// RemoveAdapter removes an adapter and its associated data
func (am *AdapterManager) RemoveAdapter(source string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.activeAdapters, source)
	delete(am.activeConfigs, source)
	delete(am.activeEventStreams, source)
}

// GetAllAdapters returns all active adapters (for iteration)
func (am *AdapterManager) GetAllAdapters() map[string]generic.GenericAdapter {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make(map[string]generic.GenericAdapter, len(am.activeAdapters))
	for k, v := range am.activeAdapters {
		result[k] = v
	}
	return result
}

// GetAllConfigs returns all active configs
func (am *AdapterManager) GetAllConfigs() map[string]*generic.SourceConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make(map[string]*generic.SourceConfig, len(am.activeConfigs))
	for k, v := range am.activeConfigs {
		result[k] = v
	}
	return result
}

// ListSources returns all active source identifiers
func (am *AdapterManager) ListSources() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	sources := make([]string, 0, len(am.activeAdapters))
	for source := range am.activeAdapters {
		sources = append(sources, source)
	}
	return sources
}

// Count returns the number of active adapters
func (am *AdapterManager) Count() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.activeAdapters)
}
