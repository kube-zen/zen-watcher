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

	"k8s.io/client-go/kubernetes"
)

// ConfigMapAdapter handles ALL ConfigMap-based sources (kube-bench, checkov, etc.)
type ConfigMapAdapter struct {
	clientSet kubernetes.Interface
	events    chan RawEvent
	lastSeen  map[string]string // ConfigMap key -> last seen data hash
	mu        sync.RWMutex
}

// NewConfigMapAdapter creates a new generic ConfigMap adapter
func NewConfigMapAdapter(clientSet kubernetes.Interface) *ConfigMapAdapter {
	return &ConfigMapAdapter{
		clientSet: clientSet,
		events:    make(chan RawEvent, 100),
		lastSeen:  make(map[string]string),
	}
}

// Type returns the adapter type
func (a *ConfigMapAdapter) Type() string {
	return "configmap"
}

// Validate validates the configmap configuration
func (a *ConfigMapAdapter) Validate(config *SourceConfig) error {
	return fmt.Errorf("ConfigMap adapter is not supported. Use Informer adapter with GVR { group: \"\", version: \"v1\", resource: \"configmaps\" } instead")
}

// Start starts the ConfigMap adapter
func (a *ConfigMapAdapter) Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error) {
	return nil, fmt.Errorf("ConfigMap adapter is not supported. Use Informer adapter with GVR { group: \"\", version: \"v1\", resource: \"configmaps\" } instead")
}

// pollConfigMaps periodically polls ConfigMaps and emits events on changes
func (a *ConfigMapAdapter) pollConfigMaps(ctx context.Context, config *SourceConfig, pollInterval interface{}) {
	// Not implemented
}

// doPoll performs a single poll of ConfigMaps
func (a *ConfigMapAdapter) doPoll(ctx context.Context, config *SourceConfig) {
	// Not implemented
}

// Stop stops the ConfigMap adapter
func (a *ConfigMapAdapter) Stop() {
	close(a.events)
}
