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

package server

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KubernetesAPIChecker defines the interface for checking Kubernetes API connectivity
type KubernetesAPIChecker interface {
	CheckAPIHealth(ctx context.Context) error
}

// kubernetesAPIChecker implements KubernetesAPIChecker using a Kubernetes client
type kubernetesAPIChecker struct {
	client kubernetes.Interface
}

// NewKubernetesAPIChecker creates a new Kubernetes API health checker
func NewKubernetesAPIChecker(client kubernetes.Interface) KubernetesAPIChecker {
	return &kubernetesAPIChecker{client: client}
}

// CheckAPIHealth performs a lightweight Kubernetes API call to verify connectivity
// Uses a Lease read (lightweight, no side effects) to check API server reachability
func (c *kubernetesAPIChecker) CheckAPIHealth(ctx context.Context) error {
	// Use a lightweight API call: list namespaces (core resource, always available)
	// This is more reliable than /healthz endpoint which may not be accessible
	_, err := c.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		Limit: 1, // Only need to verify API is reachable, not get all namespaces
	})
	if err != nil {
		return err
	}
	return nil
}

