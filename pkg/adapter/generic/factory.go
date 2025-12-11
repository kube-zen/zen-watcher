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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
)

// Factory creates generic adapters based on adapter type
type Factory struct {
	dynClient      dynamic.Interface
	dynFactory     dynamicinformer.DynamicSharedInformerFactory // Deprecated: use informerManager
	informerManager *informers.Manager                          // Preferred: use manager
	clientSet      kubernetes.Interface
	webhookPorts   map[string]int // Track used ports
}

// NewFactory creates a new adapter factory
// Deprecated: Use NewFactoryWithManager for new code
func NewFactory(
	dynClient dynamic.Interface,
	dynFactory dynamicinformer.DynamicSharedInformerFactory,
	clientSet kubernetes.Interface,
) *Factory {
	return &Factory{
		dynClient:    dynClient,
		dynFactory:   dynFactory,
		clientSet:    clientSet,
		webhookPorts: make(map[string]int),
	}
}

// NewFactoryWithManager creates a new adapter factory using InformerManager
func NewFactoryWithManager(
	dynClient dynamic.Interface,
	informerManager *informers.Manager,
	clientSet kubernetes.Interface,
) *Factory {
	return &Factory{
		dynClient:       dynClient,
		informerManager: informerManager,
		clientSet:       clientSet,
		webhookPorts:    make(map[string]int),
	}
}

// NewAdapter creates a new generic adapter based on ingester type
func (f *Factory) NewAdapter(ingester string) (GenericAdapter, error) {
	switch ingester {
	case "informer":
		// Prefer manager, fallback to factory for backward compatibility
		if f.informerManager != nil {
			return NewInformerAdapterWithManager(f.informerManager), nil
		}
		return NewInformerAdapter(f.dynFactory), nil
	case "webhook":
		return NewWebhookAdapter(), nil
	case "logs":
		if f.clientSet == nil {
			return nil, fmt.Errorf("kubernetes client required for logs adapter")
		}
		return NewLogsAdapter(f.clientSet), nil
	case "cm", "configmap": // Support both "cm" (new) and "configmap" (legacy) for backward compatibility
		if f.clientSet == nil {
			return nil, fmt.Errorf("kubernetes client required for configmap adapter")
		}
		return NewConfigMapAdapter(f.clientSet), nil
	case "k8s-events":
		// k8s-events is handled by K8sEventsAdapter which is created separately
		// This factory only handles generic adapters
		return nil, fmt.Errorf("k8s-events ingester is handled by K8sEventsAdapter, not generic factory")
	default:
		return nil, fmt.Errorf("unknown ingester type: %s", ingester)
	}
}
