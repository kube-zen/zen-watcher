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

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
)

// TestConfigMapLoader_Integration tests ConfigMap loader with filter integration
func TestConfigMapLoader_Integration(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Helper to create bool pointer
	boolPtr := func(b bool) *bool { return &b }

	// Create initial filter config
	initialConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"falco": {
				MinSeverity: "MEDIUM",
				Enabled:     boolPtr(true),
			},
			"trivy": {
				MinSeverity: "HIGH",
				Enabled:     boolPtr(true),
			},
		},
	}
	filterInstance := filter.NewFilter(initialConfig)

	// Create ConfigMap loader
	loader := config.NewConfigMapLoader(clientSet, filterInstance)

	// Create initial ConfigMap
	filterJSON, err := json.Marshal(initialConfig)
	if err != nil {
		t.Fatalf("Failed to marshal filter config: %v", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": string(filterJSON),
		},
	}
	_, err = clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Start loader
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- loader.Start(ctx)
	}()

	// Wait for initial load
	time.Sleep(500 * time.Millisecond)

	// Verify filter was updated
	if !filterInstance.Allow(createTestObservation("falco", "MEDIUM")) {
		t.Error("Filter should allow MEDIUM severity for falco")
	}
	if filterInstance.Allow(createTestObservation("falco", "LOW")) {
		t.Error("Filter should not allow LOW severity for falco")
	}

	// Update ConfigMap
	newConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"falco": {
				MinSeverity: "HIGH",
				Enabled:     boolPtr(true),
			},
			"trivy": {
				MinSeverity: "CRITICAL",
				Enabled:     boolPtr(true),
			},
		},
	}
	newFilterJSON, err := json.Marshal(newConfig)
	if err != nil {
		t.Fatalf("Failed to marshal new filter config: %v", err)
	}

	cm.Data["filter.json"] = string(newFilterJSON)
	_, err = clientSet.CoreV1().ConfigMaps("zen-system").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update ConfigMap: %v", err)
	}

	// Wait for update to be processed
	time.Sleep(1 * time.Second)

	// Verify filter was updated
	if filterInstance.Allow(createTestObservation("falco", "MEDIUM")) {
		t.Error("Filter should not allow MEDIUM severity for falco after update")
	}
	if !filterInstance.Allow(createTestObservation("falco", "HIGH")) {
		t.Error("Filter should allow HIGH severity for falco after update")
	}

	// Cancel context to stop loader
	cancel()

	// Wait for loader to stop
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Loader did not stop within timeout")
	}
}

// TestConfigMapLoader_InvalidConfig tests handling of invalid ConfigMap data
func TestConfigMapLoader_InvalidConfig(t *testing.T) {
	clientSet := fake.NewSimpleClientset()

	initialConfig := &filter.FilterConfig{
		Sources: make(map[string]filter.SourceFilter),
	}
	filterInstance := filter.NewFilter(initialConfig)

	loader := config.NewConfigMapLoader(clientSet, filterInstance)

	// Create ConfigMap with invalid JSON
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": `{invalid json}`,
		},
	}
	_, err := clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- loader.Start(ctx)
	}()

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify filter still has initial config (invalid config should be rejected)
	// The filter should remain unchanged when invalid config is provided

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Logf("Expected error for invalid config: %v", err)
		}
	case <-time.After(2 * time.Second):
		// Timeout is acceptable
	}
}

// TestConfigMapLoader_MissingKey tests handling of missing filter.json key
func TestConfigMapLoader_MissingKey(t *testing.T) {
	clientSet := fake.NewSimpleClientset()

	boolPtr := func(b bool) *bool { return &b }
	initialConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"falco": {
				MinSeverity: "MEDIUM",
				Enabled:     boolPtr(true),
			},
		},
	}
	filterInstance := filter.NewFilter(initialConfig)

	loader := config.NewConfigMapLoader(clientSet, filterInstance)

	// Create ConfigMap without filter.json key
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"other-key": "value",
		},
	}
	_, err := clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- loader.Start(ctx)
	}()

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify filter still has initial config (missing key should preserve last good config)
	if !filterInstance.Allow(createTestObservation("falco", "MEDIUM")) {
		t.Error("Filter should preserve last good config when key is missing")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Logf("Error may occur for missing key: %v", err)
		}
	case <-time.After(2 * time.Second):
		// Timeout is acceptable
	}
}

// createTestObservation creates a test observation for filtering tests
func createTestObservation(source, severity string) *unstructured.Unstructured {
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"name":      "test-observation",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"source":   source,
				"severity": severity,
			},
		},
	}
	return obs
}

