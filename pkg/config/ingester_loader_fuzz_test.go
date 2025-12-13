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

//go:build go1.18
// +build go1.18

package config

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicinformer "k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

// FuzzLoadIngesterConfig fuzzes the Ingester config loading with random/partially-corrupt specs
func FuzzLoadIngesterConfig(f *testing.F) {
	// Seed corpus with valid examples
	seedSpecs := []string{
		`{"apiVersion":"zen.kube-zen.io/v1","kind":"Ingester","spec":{"source":"trivy","ingester":"informer","destinations":[{"type":"crd","value":"observations"}]}}`,
		`{"apiVersion":"zen.kube-zen.io/v1alpha1","kind":"Ingester","spec":{"source":"falco","ingester":"webhook","destinations":[{"type":"crd","value":"observations"}]}}`,
	}

	for _, seed := range seedSpecs {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, specJSON string) {
		// Parse JSON (may fail, that's OK for fuzzing)
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(specJSON), &obj); err != nil {
			t.Skip() // Invalid JSON, skip
		}

		unstructuredObj := &unstructured.Unstructured{Object: obj}

		// Create a minimal IngesterInformer to test conversion
		store := NewIngesterStore()
		dynClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 0)
		informer := &IngesterInformer{
			store:     store,
			dynClient: dynClient,
			factory:   factory,
		}

		// Try to convert - should not panic
		// Errors are OK, but panics are not
		_ = informer.convertToIngesterConfig(unstructuredObj)
		// We don't check for nil - fuzzing is about finding panics
	})
}

// FuzzLoadIngesterConfig_MalformedYAML fuzzes with malformed YAML-like input
func FuzzLoadIngesterConfig_MalformedYAML(f *testing.F) {
	f.Fuzz(func(t *testing.T, yamlData string) {
		// Limit size to prevent excessive memory usage
		if len(yamlData) > 100000 {
			t.Skip()
		}

		// Try to parse as unstructured - should not panic
		// We're testing that malformed input is handled gracefully
		var obj map[string]interface{}
		_ = yaml.Unmarshal([]byte(yamlData), &obj)

		unstructuredObj := &unstructured.Unstructured{Object: obj}

		// Create a minimal IngesterInformer to test conversion
		store := NewIngesterStore()
		dynClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 0)
		informer := &IngesterInformer{
			store:     store,
			dynClient: dynClient,
			factory:   factory,
		}

		// Try to convert - should not panic even with malformed data
		_ = informer.convertToIngesterConfig(unstructuredObj)
	})
}
