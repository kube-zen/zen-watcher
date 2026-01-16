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

package watcher

import (
	"context"
	"errors"
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestCRDCreator_RejectsDeniedGVRs(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	// Test that secrets are rejected even if passed directly
	secretsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	creator := NewCRDCreator(dynClient, secretsGVR, allowlist)

	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"source": "test-source",
			},
		},
	}

	err := creator.CreateCRD(context.Background(), observation)
	if err == nil {
		t.Errorf("Expected rejection for secrets GVR, but got nil error")
	}
	if !errors.Is(err, ErrGVRDenied) {
		// Check if error wraps ErrGVRDenied
		var gvrDeniedErr error = ErrGVRDenied
		if !errors.Is(err, gvrDeniedErr) {
			t.Errorf("Expected error to wrap ErrGVRDenied, got: %v", err)
		}
	}
}

func TestCRDCreator_RejectsNonAllowlistedGVRs(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	// Test non-allowlisted GVR
	customGVR := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "customresources",
	}

	creator := NewCRDCreator(dynClient, customGVR, allowlist)

	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"source": "test-source",
			},
		},
	}

	err := creator.CreateCRD(context.Background(), observation)
	if err == nil {
		t.Errorf("Expected rejection for non-allowlisted GVR, but got nil error")
	}
	if !errors.Is(err, ErrGVRNotAllowed) {
		t.Errorf("Expected error to wrap ErrGVRNotAllowed, got: %v", err)
	}
}

func TestCRDCreator_RejectsNonAllowlistedNamespaces(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	// Test allowed GVR
	observationsGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	creator := NewCRDCreator(dynClient, observationsGVR, allowlist)

	// Test with disallowed namespace
	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": "other-ns", // Not in allowlist
			},
			"spec": map[string]interface{}{
				"source": "test-source",
			},
		},
	}

	err := creator.CreateCRD(context.Background(), observation)
	if err == nil {
		t.Errorf("Expected rejection for non-allowlisted namespace, but got nil error")
	}
	if !errors.Is(err, ErrNamespaceNotAllowed) {
		t.Errorf("Expected error to wrap ErrNamespaceNotAllowed, got: %v", err)
	}
}

func TestCRDCreator_AllowsValidGVRAndNamespace(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	// Test allowed GVR
	observationsGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	creator := NewCRDCreator(dynClient, observationsGVR, allowlist)

	// Test with allowed namespace
	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"source": "test-source",
			},
		},
	}

	// This should pass allowlist check (may fail on actual create, but that's OK)
	err := creator.CreateCRD(context.Background(), observation)
	// We expect this to either succeed or fail on actual Kubernetes write,
	// but NOT fail on allowlist check
	if err != nil {
		// Check that error is NOT an allowlist error
		if errors.Is(err, ErrGVRNotAllowed) ||
			errors.Is(err, ErrGVRDenied) ||
			errors.Is(err, ErrNamespaceNotAllowed) ||
			errors.Is(err, ErrClusterScopedNotAllowed) {
			t.Errorf("Unexpected allowlist error: %v", err)
		}
		// Other errors (like "resource not found" from fake client) are acceptable
	}
}
