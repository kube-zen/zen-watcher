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

package kubernetes

import (
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Clients holds Kubernetes client interfaces
type Clients struct {
	Dynamic  dynamic.Interface
	Standard kubernetes.Interface
	Config   *rest.Config
}

// GVRs holds GroupVersionResource definitions for security tools
type GVRs struct {
	Observations schema.GroupVersionResource
	PolicyReport schema.GroupVersionResource
}

// NewClients creates Kubernetes clients from in-cluster config
func NewClients() (*Clients, error) {
	logger.Info("Initializing Kubernetes client",
		logger.Fields{
			Component: "kubernetes",
			Operation: "client_init",
		})

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	logger.Info("Kubernetes client ready",
		logger.Fields{
			Component: "kubernetes",
			Operation: "client_init",
		})

	return &Clients{
		Dynamic:  dynClient,
		Standard: clientSet,
		Config:   config,
	}, nil
}

// NewGVRs returns the GroupVersionResource definitions
func NewGVRs() *GVRs {
	return &GVRs{
		Observations: schema.GroupVersionResource{
			Group:    "zen.kube-zen.io",
			Version:  "v1",
			Resource: "observations",
		},
		PolicyReport: schema.GroupVersionResource{
			Group:    "wgpolicyk8s.io",
			Version:  "v1alpha2",
			Resource: "policyreports",
		},
	}
}

// NewInformerFactory creates a dynamic informer factory with default resync period
func NewInformerFactory(dynClient dynamic.Interface) dynamicinformer.DynamicSharedInformerFactory {
	// Resync period: 30 minutes (periodic full resync for deduplication)
	resyncPeriod := 30 * time.Minute
	return dynamicinformer.NewDynamicSharedInformerFactory(dynClient, resyncPeriod)
}
