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
	"crypto/rand"
	"encoding/hex"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// SetupTestEnvWithNameGeneration creates a fake dynamic client that supports generateName
// This improves on the standard fake client by generating names when generateName is used
// This is a standalone version that can be used from pkg/ packages without import cycles
func SetupTestEnvWithNameGeneration(t *testing.T) dynamic.Interface {
	t.Helper()
	scheme := runtime.NewScheme()
	observationGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}
	ingesterGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "ingesters",
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		observationGVR: "ObservationList",
		ingesterGVR:    "IngesterList",
	})

	// Wrap the client to handle generateName
	return &nameGeneratingClient{
		Interface: client,
		t:         t,
	}
}

// nameGeneratingClient wraps a dynamic client to handle generateName
type nameGeneratingClient struct {
	dynamic.Interface
	t *testing.T
}

// Resource implements dynamic.Interface
func (c *nameGeneratingClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &nameGeneratingResource{
		NamespaceableResourceInterface: c.Interface.Resource(resource),
		t:                              c.t,
	}
}

// nameGeneratingResource wraps NamespaceableResourceInterface to handle generateName
type nameGeneratingResource struct {
	dynamic.NamespaceableResourceInterface
	t *testing.T
}

// Create handles generateName by generating a unique name
func (r *nameGeneratingResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	// Check if generateName is set but name is not
	generateName, hasGenerateName, _ := unstructured.NestedString(obj.Object, "metadata", "generateName")
	name, hasName, _ := unstructured.NestedString(obj.Object, "metadata", "name")

	if hasGenerateName && generateName != "" && (!hasName || name == "") {
		// Generate a unique name
		randomBytes := make([]byte, 8)
		if _, err := rand.Read(randomBytes); err != nil {
			r.t.Logf("Warning: failed to generate random name, using timestamp: %v", err)
			// Fallback to timestamp-based name
			name = generateName + hex.EncodeToString(randomBytes)
		} else {
			name = generateName + hex.EncodeToString(randomBytes)
		}

		// Set the generated name
		if err := unstructured.SetNestedField(obj.Object, name, "metadata", "name"); err != nil {
			r.t.Logf("Warning: failed to set generated name: %v", err)
		}
	}

	// Use the underlying client to create
	return r.NamespaceableResourceInterface.Create(ctx, obj, options, subresources...)
}

// Namespace returns a namespaced resource interface
func (r *nameGeneratingResource) Namespace(namespace string) dynamic.ResourceInterface {
	return &nameGeneratingNamespacedResource{
		ResourceInterface: r.NamespaceableResourceInterface.Namespace(namespace),
		t:                 r.t,
	}
}

// nameGeneratingNamespacedResource wraps ResourceInterface for namespace-scoped resources
type nameGeneratingNamespacedResource struct {
	dynamic.ResourceInterface
	t *testing.T
}

// Create handles generateName for namespaced resources
func (r *nameGeneratingNamespacedResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	// Check if generateName is set but name is not
	generateName, hasGenerateName, _ := unstructured.NestedString(obj.Object, "metadata", "generateName")
	name, hasName, _ := unstructured.NestedString(obj.Object, "metadata", "name")

	if hasGenerateName && generateName != "" && (!hasName || name == "") {
		// Generate a unique name
		randomBytes := make([]byte, 8)
		if _, err := rand.Read(randomBytes); err != nil {
			r.t.Logf("Warning: failed to generate random name: %v", err)
			// Fallback: use generateName with a simple counter (not ideal but works)
			name = generateName + "generated"
		} else {
			name = generateName + hex.EncodeToString(randomBytes)
		}

		// Set the generated name
		if err := unstructured.SetNestedField(obj.Object, name, "metadata", "name"); err != nil {
			r.t.Logf("Warning: failed to set generated name: %v", err)
		}
	}

	// Use the underlying client to create
	return r.ResourceInterface.Create(ctx, obj, options, subresources...)
}
