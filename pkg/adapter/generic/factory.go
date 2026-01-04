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
	"fmt"

	"github.com/kube-zen/zen-watcher/internal/informers"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
)

// Factory creates generic adapters based on adapter type
type Factory struct {
	informerManager *informers.Manager
	clientSet       kubernetes.Interface
	webhookPorts    map[string]int // Track used ports
	webhookMetrics  *prometheus.CounterVec // Metrics for webhook requests (optional)
	webhookDropped  *prometheus.CounterVec // Metrics for webhook events dropped (optional)
}

// NewFactory creates a new adapter factory using InformerManager
func NewFactory(
	informerManager *informers.Manager,
	clientSet kubernetes.Interface,
) *Factory {
	return NewFactoryWithMetrics(informerManager, clientSet, nil, nil)
}

// NewFactoryWithMetrics creates a new adapter factory with metrics support
func NewFactoryWithMetrics(
	informerManager *informers.Manager,
	clientSet kubernetes.Interface,
	webhookMetrics *prometheus.CounterVec,
	webhookDropped *prometheus.CounterVec,
) *Factory {
	return &Factory{
		informerManager: informerManager,
		clientSet:       clientSet,
		webhookPorts:    make(map[string]int),
		webhookMetrics:  webhookMetrics,
		webhookDropped:  webhookDropped,
	}
}

// NewAdapter creates a new generic adapter based on ingester type
func (f *Factory) NewAdapter(ingester string) (GenericAdapter, error) {
	switch ingester {
	case "informer":
		if f.informerManager == nil {
			return nil, fmt.Errorf("informer manager is required")
		}
		return NewInformerAdapterWithManager(f.informerManager), nil
	case "webhook":
		if f.clientSet == nil {
			return nil, fmt.Errorf("kubernetes client required for webhook adapter")
		}
		return NewWebhookAdapterWithMetrics(f.clientSet, f.webhookMetrics, f.webhookDropped), nil
	case "logs":
		if f.clientSet == nil {
			return nil, fmt.Errorf("kubernetes client required for logs adapter")
		}
		return NewLogsAdapter(f.clientSet), nil
	default:
		return nil, fmt.Errorf("unsupported ingester type: %s (must be one of: informer, webhook, logs)", ingester)
	}
}
