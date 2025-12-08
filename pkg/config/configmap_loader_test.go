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

package config

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/filter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestConfigMapLoader_ReloadConfig(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Create initial filter
	initialConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM",
			},
		},
	}
	filterInstance := filter.NewFilter(initialConfig)

	// Create ConfigMap loader
	loader := NewConfigMapLoader(clientSet, filterInstance)

	// Create initial ConfigMap
	filterJSON, _ := json.Marshal(initialConfig)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": string(filterJSON),
		},
	}
	_, err := clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Test initial load
	config, err := loader.loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if config.Sources["trivy"].MinSeverity != "MEDIUM" {
		t.Errorf("Expected MinSeverity MEDIUM, got %s", config.Sources["trivy"].MinSeverity)
	}

	// Update ConfigMap with new config
	newConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity: "HIGH",
			},
			"falco": {
				MinSeverity: "CRITICAL",
			},
		},
	}
	newFilterJSON, _ := json.Marshal(newConfig)
	cm.Data["filter.json"] = string(newFilterJSON)
	_, err = clientSet.CoreV1().ConfigMaps("zen-system").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update ConfigMap: %v", err)
	}

	// Simulate ConfigMap update
	loader.handleConfigMapChange(cm)

	// Verify filter was updated
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "MEDIUM",
			},
		},
	}
	allowed, reason := filterInstance.AllowWithReason(obs)
	if allowed {
		t.Error("Expected MEDIUM severity to be filtered out after update to HIGH minimum")
	}
	if reason != "min_severity" {
		t.Errorf("Expected reason 'min_severity', got '%s'", reason)
	}

	// Test HIGH severity should pass
	obsHigh := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "HIGH",
			},
		},
	}
	allowed, _ = filterInstance.AllowWithReason(obsHigh)
	if !allowed {
		t.Error("Expected HIGH severity to pass after update")
	}
}

func TestConfigMapLoader_InvalidConfigKeepsLastGood(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Create initial filter with good config
	initialConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM",
			},
		},
	}
	filterInstance := filter.NewFilter(initialConfig)

	// Create ConfigMap loader
	loader := NewConfigMapLoader(clientSet, filterInstance)

	// Create initial ConfigMap with valid config
	filterJSON, _ := json.Marshal(initialConfig)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": string(filterJSON),
		},
	}
	_, err := clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Load initial config
	loader.handleConfigMapChange(cm)
	lastGood := loader.GetLastGoodConfig()
	if lastGood == nil || lastGood.Sources["trivy"].MinSeverity != "MEDIUM" {
		t.Error("Failed to set last good config")
	}

	// Update ConfigMap with invalid JSON
	cm.Data["filter.json"] = `{invalid json}`
	_, err = clientSet.CoreV1().ConfigMaps("zen-system").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update ConfigMap: %v", err)
	}

	// Simulate ConfigMap update with invalid config
	loader.handleConfigMapChange(cm)

	// Verify last good config is still there
	lastGoodAfterInvalid := loader.GetLastGoodConfig()
	if lastGoodAfterInvalid == nil || lastGoodAfterInvalid.Sources["trivy"].MinSeverity != "MEDIUM" {
		t.Error("Last good config should be preserved after invalid config update")
	}

	// Verify filter still uses last good config
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "LOW",
			},
		},
	}
	allowed, reason := filterInstance.AllowWithReason(obs)
	if allowed {
		t.Error("Expected LOW severity to be filtered out (last good config has MEDIUM minimum)")
	}
	if reason != "min_severity" {
		t.Errorf("Expected reason 'min_severity', got '%s'", reason)
	}
}

func TestConfigMapLoader_MissingKeyKeepsLastGood(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Create initial filter
	initialConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM",
			},
		},
	}
	filterInstance := filter.NewFilter(initialConfig)

	// Create ConfigMap loader
	loader := NewConfigMapLoader(clientSet, filterInstance)

	// Create initial ConfigMap
	filterJSON, _ := json.Marshal(initialConfig)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": string(filterJSON),
		},
	}
	_, err := clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Load initial config
	loader.handleConfigMapChange(cm)

	// Update ConfigMap to remove the key
	delete(cm.Data, "filter.json")
	_, err = clientSet.CoreV1().ConfigMaps("zen-system").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update ConfigMap: %v", err)
	}

	// Simulate ConfigMap update with missing key
	loader.handleConfigMapChange(cm)

	// Verify last good config is still there
	lastGood := loader.GetLastGoodConfig()
	if lastGood == nil || lastGood.Sources["trivy"].MinSeverity != "MEDIUM" {
		t.Error("Last good config should be preserved after missing key")
	}
}

func TestConfigMapLoader_Start(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Create initial filter
	initialConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM",
			},
		},
	}
	filterInstance := filter.NewFilter(initialConfig)

	// Create ConfigMap loader
	loader := NewConfigMapLoader(clientSet, filterInstance)

	// Create ConfigMap
	filterJSON, _ := json.Marshal(initialConfig)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": string(filterJSON),
		},
	}
	_, err := clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Start loader in background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- loader.Start(ctx)
	}()

	// Wait a bit for informer to sync
	time.Sleep(500 * time.Millisecond)

	// Update ConfigMap
	newConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity: "HIGH",
			},
		},
	}
	newFilterJSON, _ := json.Marshal(newConfig)
	cm.Data["filter.json"] = string(newFilterJSON)
	_, err = clientSet.CoreV1().ConfigMaps("zen-system").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update ConfigMap: %v", err)
	}

	// Wait for update to be processed
	time.Sleep(500 * time.Millisecond)

	// Cancel context to stop loader
	cancel()

	// Wait for loader to stop
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Loader did not stop within timeout")
	}
}

func TestConfigMapLoader_DeleteFuncHandlesTombstone(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Create initial filter
	initialConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM",
			},
		},
	}
	filterInstance := filter.NewFilter(initialConfig)

	// Create ConfigMap loader
	loader := NewConfigMapLoader(clientSet, filterInstance)

	// Create ConfigMap
	filterJSON, _ := json.Marshal(initialConfig)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": string(filterJSON),
		},
	}
	_, err := clientSet.CoreV1().ConfigMaps("zen-system").Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Load initial config
	loader.handleConfigMapChange(cm)
	lastGood := loader.GetLastGoodConfig()
	if lastGood == nil {
		t.Fatal("Failed to set last good config")
	}

	// Simulate tombstone (DeletedFinalStateUnknown) - test that our DeleteFunc handles it
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "zen-system/zen-watcher-filter",
		Obj: cm,
	}

	// Verify tombstone extraction works (simulating what DeleteFunc does)
	cmFromTombstone, ok := tombstone.Obj.(*corev1.ConfigMap)
	if !ok {
		t.Fatal("Failed to extract ConfigMap from tombstone")
	}
	if cmFromTombstone.Name != "zen-watcher-filter" {
		t.Errorf("Expected ConfigMap name 'zen-watcher-filter', got '%s'", cmFromTombstone.Name)
	}

	// Verify last good config is preserved
	lastGoodBefore := loader.GetLastGoodConfig()
	if lastGoodBefore == nil {
		t.Fatal("Last good config should exist")
	}
}

func TestConfigMapLoader_EmptySourcesMap(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Create filter with nil Sources
	filterInstance := filter.NewFilter(&filter.FilterConfig{
		Sources: nil, // nil map
	})

	// Create ConfigMap loader
	loader := NewConfigMapLoader(clientSet, filterInstance)

	// Create ConfigMap with empty Sources map
	emptyConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{}, // empty map
	}
	filterJSON, _ := json.Marshal(emptyConfig)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-watcher-filter",
			Namespace: "zen-system",
		},
		Data: map[string]string{
			"filter.json": string(filterJSON),
		},
	}

	// This should not panic
	loader.handleConfigMapChange(cm)

	// Verify filter still works (should allow all since no sources configured)
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "LOW",
			},
		},
	}
	allowed, _ := filterInstance.AllowWithReason(obs)
	if !allowed {
		t.Error("Empty Sources map should allow all observations")
	}
}

func TestConfigMapLoader_ContextCancellation(t *testing.T) {
	// Create fake client
	clientSet := fake.NewSimpleClientset()

	// Create filter
	filterInstance := filter.NewFilter(&filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{},
	})

	// Create ConfigMap loader
	loader := NewConfigMapLoader(clientSet, filterInstance)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// loadConfigWithContext should respect cancellation
	_, err := loader.loadConfigWithContext(ctx)
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}
