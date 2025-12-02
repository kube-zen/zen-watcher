// Copyright 2024 The Zen Watcher Authors
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

	"github.com/kube-zen/zen-watcher/pkg/filter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestObservationFilterLoader_LoadSingleFilter(t *testing.T) {
	// Create a fake dynamic client with a single ObservationFilter
	observationFilter := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"minSeverity": "HIGH",
		"excludeNamespaces": []string{"kube-system"},
	})

	client := dynamicfake.NewSimpleDynamicClient(observationFilter)

	// Create filter and loader
	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{} // Minimal mock
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	// Load filters
	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	// Verify trivy filter exists
	trivyFilter, exists := config.Sources["trivy"]
	if !exists {
		t.Fatal("Expected trivy source in loaded config")
	}

	if trivyFilter.MinSeverity != "HIGH" {
		t.Errorf("Expected MinSeverity HIGH, got %s", trivyFilter.MinSeverity)
	}

	expectedNamespaces := []string{"kube-system"}
	if len(trivyFilter.ExcludeNamespaces) != 1 || trivyFilter.ExcludeNamespaces[0] != "kube-system" {
		t.Errorf("Expected ExcludeNamespaces %v, got %v", expectedNamespaces, trivyFilter.ExcludeNamespaces)
	}
}

func TestObservationFilterLoader_LoadMultipleFilters_SameSource(t *testing.T) {
	// Create multiple ObservationFilters targeting the same source
	filter1 := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"minSeverity":      "MEDIUM",
		"excludeNamespaces": []string{"kube-system"},
	})

	filter2 := createObservationFilter("filter-2", "default", "trivy", map[string]interface{}{
		"minSeverity":      "HIGH", // More restrictive
		"excludeNamespaces": []string{"kube-public"}, // Should union with kube-system
	})

	client := dynamicfake.NewSimpleDynamicClient(filter1, filter2)

	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{}
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	trivyFilter, exists := config.Sources["trivy"]
	if !exists {
		t.Fatal("Expected trivy source in loaded config")
	}

	// MinSeverity should be HIGH (most restrictive)
	if trivyFilter.MinSeverity != "HIGH" {
		t.Errorf("Expected MinSeverity HIGH (most restrictive), got %s", trivyFilter.MinSeverity)
	}

	// ExcludeNamespaces should be union of both
	expectedNamespaces := []string{"kube-system", "kube-public"}
	if !containsAll(trivyFilter.ExcludeNamespaces, expectedNamespaces) {
		t.Errorf("Expected ExcludeNamespaces to contain %v, got %v", expectedNamespaces, trivyFilter.ExcludeNamespaces)
	}
}

func TestObservationFilterLoader_LoadMultipleFilters_DifferentSources(t *testing.T) {
	filter1 := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"minSeverity": "HIGH",
	})

	filter2 := createObservationFilter("filter-2", "default", "falco", map[string]interface{}{
		"minSeverity": "CRITICAL",
	})

	client := dynamicfake.NewSimpleDynamicClient(filter1, filter2)

	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{}
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	if len(config.Sources) != 2 {
		t.Fatalf("Expected 2 sources, got %d", len(config.Sources))
	}

	if config.Sources["trivy"].MinSeverity != "HIGH" {
		t.Errorf("Expected trivy MinSeverity HIGH, got %s", config.Sources["trivy"].MinSeverity)
	}

	if config.Sources["falco"].MinSeverity != "CRITICAL" {
		t.Errorf("Expected falco MinSeverity CRITICAL, got %s", config.Sources["falco"].MinSeverity)
	}
}

func TestObservationFilterLoader_CrossNamespaceFilters(t *testing.T) {
	filter1 := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"minSeverity": "MEDIUM",
	})

	filter2 := createObservationFilter("filter-2", "production", "trivy", map[string]interface{}{
		"minSeverity": "HIGH",
	})

	client := dynamicfake.NewSimpleDynamicClient(filter1, filter2)

	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{}
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	// Both filters should be merged
	trivyFilter, exists := config.Sources["trivy"]
	if !exists {
		t.Fatal("Expected trivy source in loaded config")
	}

	// MinSeverity should be HIGH (most restrictive from production namespace)
	if trivyFilter.MinSeverity != "HIGH" {
		t.Errorf("Expected MinSeverity HIGH (most restrictive from cross-namespace merge), got %s", trivyFilter.MinSeverity)
	}
}

func TestObservationFilterLoader_InvalidCRD_SkipsInvalid(t *testing.T) {
	// Valid filter
	validFilter := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"minSeverity": "HIGH",
	})

	// Invalid filter - missing targetSource
	invalidFilter := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1alpha1",
			"kind":       "ObservationFilter",
			"metadata": map[string]interface{}{
				"name":      "filter-invalid",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				// Missing targetSource - should be skipped
				"minSeverity": "HIGH",
			},
		},
	}

	client := dynamicfake.NewSimpleDynamicClient(validFilter, invalidFilter)

	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{}
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	// Only valid filter should be loaded
	_, exists := config.Sources["trivy"]
	if !exists {
		t.Fatal("Expected trivy source from valid filter")
	}

	// Invalid filter should be skipped (no error, just logged)
	if len(config.Sources) != 1 {
		t.Errorf("Expected 1 source (invalid filter skipped), got %d", len(config.Sources))
	}
}

func TestObservationFilterLoader_EmptyTargetSource_Skips(t *testing.T) {
	filter1 := createObservationFilter("filter-1", "default", "", map[string]interface{}{ // Empty targetSource
		"minSeverity": "HIGH",
	})

	client := dynamicfake.NewSimpleDynamicClient(filter1)

	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{}
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	// Filter with empty targetSource should be skipped
	if len(config.Sources) != 0 {
		t.Errorf("Expected 0 sources (empty targetSource skipped), got %d", len(config.Sources))
	}
}

func TestObservationFilterLoader_MergeWithConfigMap(t *testing.T) {
	// Create ConfigMapLoader with a real client and set initial config via ConfigMap
	clientSet := kubernetesfake.NewSimpleClientset()
	
	// ConfigMap filter
	configMapConfig := &filter.FilterConfig{
		Sources: map[string]filter.SourceFilter{
			"trivy": {
				MinSeverity:       "MEDIUM",
				ExcludeNamespaces: []string{"kube-system"},
			},
		},
	}
	
	// Create ConfigMap with initial config
	filterJSON, _ := json.Marshal(configMapConfig)
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

	// Create ConfigMapLoader and load initial config
	filterInstance := filter.NewFilter(nil)
	configMapLoader := NewConfigMapLoader(clientSet, filterInstance)
	configMapLoader.handleConfigMapChange(cm) // This sets last good config

	// ObservationFilter CRD
	observationFilter := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"minSeverity":       "HIGH", // More restrictive
		"excludeNamespaces": []string{"kube-public"}, // Should union with kube-system
	})

	dynClient := dynamicfake.NewSimpleDynamicClient(observationFilter)

	loader := NewObservationFilterLoader(dynClient, filterInstance, configMapLoader)

	ctx := context.Background()
	crdConfig, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	// Simulate updateFilter which merges ConfigMap + CRD
	loader.updateFilter(crdConfig)

	// Verify merged config was applied
	trivyObs := createObservation("trivy", "security", "MEDIUM", "default", "Pod", "test")
	allowed, _ := filterInstance.AllowWithReason(trivyObs)

	// MEDIUM should be filtered out (merged MinSeverity is HIGH)
	if allowed {
		t.Error("Expected MEDIUM severity to be filtered out (merged MinSeverity is HIGH)")
	}

	// HIGH should pass
	highObs := createObservation("trivy", "security", "HIGH", "default", "Pod", "test")
	allowed, _ = filterInstance.AllowWithReason(highObs)
	if !allowed {
		t.Error("Expected HIGH severity to pass")
	}
}

func TestObservationFilterLoader_LastGoodConfig_Persistence(t *testing.T) {
	// Test that last good config is preserved after loading filters
	filter1 := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"minSeverity": "HIGH",
	})

	client := dynamicfake.NewSimpleDynamicClient(filter1)

	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{}
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	// Update filter which should set last good config
	loader.updateFilter(config)

	// Verify last good config is stored
	storedConfig := loader.GetLastGoodConfig()
	if storedConfig == nil {
		t.Fatal("Expected last good config to be stored after updateFilter")
	}

	if storedConfig.Sources["trivy"].MinSeverity != "HIGH" {
		t.Errorf("Expected stored MinSeverity HIGH, got %s", storedConfig.Sources["trivy"].MinSeverity)
	}
}

func TestObservationFilterLoader_EnabledFlag_ANDLogic(t *testing.T) {
	enabled := true
	disabled := false

	filter1 := createObservationFilter("filter-1", "default", "trivy", map[string]interface{}{
		"enabled":     enabled,
		"minSeverity": "MEDIUM",
	})

	filter2 := createObservationFilter("filter-2", "default", "trivy", map[string]interface{}{
		"enabled":     enabled,
		"minSeverity": "HIGH",
	})

	client := dynamicfake.NewSimpleDynamicClient(filter1, filter2)

	filterInstance := filter.NewFilter(nil)
	configMapLoader := &ConfigMapLoader{}
	loader := NewObservationFilterLoader(client, filterInstance, configMapLoader)

	ctx := context.Background()
	config, err := loader.loadAllObservationFilters(ctx)
	if err != nil {
		t.Fatalf("Failed to load ObservationFilters: %v", err)
	}

	trivyFilter := config.Sources["trivy"]
	if trivyFilter.Enabled == nil || !*trivyFilter.Enabled {
		t.Error("Expected Enabled to be true (both filters enabled)")
	}
}

// Helper functions

func createObservationFilter(name, namespace, targetSource string, spec map[string]interface{}) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "zen.kube-zen.io/v1alpha1",
		"kind":       "ObservationFilter",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"targetSource": targetSource,
		},
	}

	// Merge spec values
	for k, v := range spec {
		obj["spec"].(map[string]interface{})[k] = v
	}

	return &unstructured.Unstructured{Object: obj}
}

func createObservation(source, category, severity, namespace, kind, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"source":    source,
				"category":  category,
				"severity":  severity,
				"eventType": "test-event",
				"resource": map[string]interface{}{
					"kind":      kind,
					"name":      name,
					"namespace": namespace,
				},
			},
		},
	}
}

func containsAll(slice []string, items []string) bool {
	sliceMap := make(map[string]bool)
	for _, s := range slice {
		sliceMap[s] = true
	}
	for _, item := range items {
		if !sliceMap[item] {
			return false
		}
	}
	return true
}

