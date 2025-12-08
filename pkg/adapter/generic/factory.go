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

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
)

// Factory creates generic adapters based on adapter type
type Factory struct {
	dynClient    dynamic.Interface
	dynFactory   dynamicinformer.DynamicSharedInformerFactory
	clientSet    kubernetes.Interface
	webhookPorts map[string]int // Track used ports
}

// NewFactory creates a new adapter factory
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

// NewAdapter creates a new generic adapter based on adapterType
func (f *Factory) NewAdapter(adapterType string) (GenericAdapter, error) {
	switch adapterType {
	case "informer":
		return NewInformerAdapter(f.dynFactory), nil
	case "webhook":
		return NewWebhookAdapter(), nil
	case "logs":
		if f.clientSet == nil {
			return nil, fmt.Errorf("kubernetes client required for logs adapter")
		}
		return NewLogsAdapter(f.clientSet), nil
	case "configmap":
		if f.clientSet == nil {
			return nil, fmt.Errorf("kubernetes client required for configmap adapter")
		}
		return NewConfigMapAdapter(f.clientSet), nil
	default:
		return nil, fmt.Errorf("unknown adapter type: %s", adapterType)
	}
}
