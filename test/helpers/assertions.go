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

package helpers

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// AssertObservationCount asserts the number of observations in a namespace
func AssertObservationCount(t *testing.T, client dynamic.Interface, gvr schema.GroupVersionResource, namespace string, expectedCount int) {
	t.Helper()
	list, err := client.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list observations: %v", err)
	}
	if len(list.Items) != expectedCount {
		t.Errorf("Expected %d observations, got %d", expectedCount, len(list.Items))
	}
}

// AssertObservationExists asserts that an observation with the given name exists
func AssertObservationExists(t *testing.T, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) *unstructured.Unstructured {
	t.Helper()
	obj, err := client.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get observation %s/%s: %v", namespace, name, err)
	}
	return obj
}

// AssertObservationField asserts that an observation has a specific field value
func AssertObservationField(t *testing.T, obs *unstructured.Unstructured, fieldPath string, expectedValue interface{}) {
	t.Helper()
	value, found, err := unstructured.NestedFieldCopy(obs.Object, fieldPath)
	if err != nil {
		t.Fatalf("Failed to get field %s: %v", fieldPath, err)
	}
	if !found {
		t.Errorf("Field %s not found in observation", fieldPath)
		return
	}
	if value != expectedValue {
		t.Errorf("Field %s = %v, want %v", fieldPath, value, expectedValue)
	}
}
