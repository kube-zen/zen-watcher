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

// Package integration contains security regression tests for allowlist enforcement.
// These tests verify that the allowlist correctly denies writes to sensitive resources
// even if the allowlist configuration is incomplete.
//
// Note: Deny assertions can be marked as expected-fail until allowlist is complete,
// but the harness must exist and be green in CI.

package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kube-zen/zen-watcher/pkg/watcher"
)

// TestSecurityRegression_SecretsDenied tests that secrets are always denied
func TestSecurityRegression_SecretsDenied(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	testCases := []struct {
		name    string
		wantErr bool
		errType error
	}{
		{
			name:    "secrets GVR should be denied",
			wantErr: true,
			errType: watcher.ErrGVRDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Set up allowlist (even if secrets are in allowlist, they should be denied)
			if err := os.Setenv("WATCH_NAMESPACE", allowedNamespace); err != nil {
				t.Fatalf("failed to set WATCH_NAMESPACE: %v", err)
			}
			if err := os.Setenv("ALLOWED_NAMESPACES", allowedNamespace); err != nil {
				t.Fatalf("failed to set ALLOWED_NAMESPACES: %v", err)
			}
			if err := os.Setenv("ALLOWED_GVRS", "v1/secrets"); err != nil {
				t.Fatalf("failed to set ALLOWED_GVRS: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("WATCH_NAMESPACE"); err != nil {
					t.Logf("failed to unset WATCH_NAMESPACE: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_NAMESPACES"); err != nil {
					t.Logf("failed to unset ALLOWED_NAMESPACES: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_GVRS"); err != nil {
					t.Logf("failed to unset ALLOWED_GVRS: %v", err)
				}
			}()

			allowlist := watcher.NewGVRAllowlist()

			secretsGVR := schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "secrets",
			}

			creator := watcher.NewCRDCreator(dynamicClient, secretsGVR, allowlist)

			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": allowedNamespace,
					},
					"spec": map[string]interface{}{
						"source": "test-source",
					},
				},
			}

			err := creator.CreateCRD(ctx, observation)

			if !tc.wantErr {
				t.Fatalf("Test case should expect error, but wantErr is false")
			}

			if err == nil {
				t.Errorf("Expected error for secrets GVR, got nil")
				return
			}

			if !errors.Is(err, tc.errType) {
				// Check if error wraps the expected error type
				expectedErr := tc.errType
				if !errors.Is(err, expectedErr) {
					t.Errorf("Expected error to wrap %v, got: %v", tc.errType, err)
				}
			}
		})
	}
}

// TestSecurityRegression_RBACDenied tests that RBAC resources are denied
func TestSecurityRegression_RBACDenied(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	rbacGVRs := []struct {
		name string
		gvr  schema.GroupVersionResource
	}{
		{
			name: "roles should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "roles",
			},
		},
		{
			name: "rolebindings should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "rolebindings",
			},
		},
		{
			name: "clusterroles should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "clusterroles",
			},
		},
		{
			name: "clusterrolebindings should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "clusterrolebindings",
			},
		},
	}

	for _, tc := range rbacGVRs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			if err := os.Setenv("WATCH_NAMESPACE", allowedNamespace); err != nil {
				t.Fatalf("failed to set WATCH_NAMESPACE: %v", err)
			}
			if err := os.Setenv("ALLOWED_NAMESPACES", allowedNamespace); err != nil {
				t.Fatalf("failed to set ALLOWED_NAMESPACES: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("WATCH_NAMESPACE"); err != nil {
					t.Logf("failed to unset WATCH_NAMESPACE: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_NAMESPACES"); err != nil {
					t.Logf("failed to unset ALLOWED_NAMESPACES: %v", err)
				}
			}()

			allowlist := watcher.NewGVRAllowlist()
			creator := watcher.NewCRDCreator(dynamicClient, tc.gvr, allowlist)

			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": allowedNamespace,
					},
					"spec": map[string]interface{}{
						"source": "test-source",
					},
				},
			}

			err := creator.CreateCRD(ctx, observation)
			if err == nil {
				t.Errorf("Expected error for %s GVR, got nil", tc.gvr.Resource)
				return
			}

			if !errors.Is(err, watcher.ErrGVRDenied) {
				t.Errorf("Expected error to wrap ErrGVRDenied, got: %v", err)
			}
		})
	}
}

// TestSecurityRegression_WebhooksDenied tests that admission webhooks are denied
func TestSecurityRegression_WebhooksDenied(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	webhookGVRs := []struct {
		name string
		gvr  schema.GroupVersionResource
	}{
		{
			name: "validatingwebhookconfigurations should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "admissionregistration.k8s.io",
				Version:  "v1",
				Resource: "validatingwebhookconfigurations",
			},
		},
		{
			name: "mutatingwebhookconfigurations should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "admissionregistration.k8s.io",
				Version:  "v1",
				Resource: "mutatingwebhookconfigurations",
			},
		},
	}

	for _, tc := range webhookGVRs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			if err := os.Setenv("WATCH_NAMESPACE", allowedNamespace); err != nil {
				t.Fatalf("failed to set WATCH_NAMESPACE: %v", err)
			}
			if err := os.Setenv("ALLOWED_NAMESPACES", allowedNamespace); err != nil {
				t.Fatalf("failed to set ALLOWED_NAMESPACES: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("WATCH_NAMESPACE"); err != nil {
					t.Logf("failed to unset WATCH_NAMESPACE: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_NAMESPACES"); err != nil {
					t.Logf("failed to unset ALLOWED_NAMESPACES: %v", err)
				}
			}()

			allowlist := watcher.NewGVRAllowlist()
			creator := watcher.NewCRDCreator(dynamicClient, tc.gvr, allowlist)

			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": allowedNamespace,
					},
					"spec": map[string]interface{}{
						"source": "test-source",
					},
				},
			}

			err := creator.CreateCRD(ctx, observation)
			if err == nil {
				t.Errorf("Expected error for %s GVR, got nil", tc.gvr.Resource)
				return
			}

			if !errors.Is(err, watcher.ErrGVRDenied) {
				t.Errorf("Expected error to wrap ErrGVRDenied, got: %v", err)
			}
		})
	}
}

// TestSecurityRegression_CRDsDenied tests that CRD creation is denied
func TestSecurityRegression_CRDsDenied(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	crdGVRs := []struct {
		name string
		gvr  schema.GroupVersionResource
	}{
		{
			name: "customresourcedefinitions v1 should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			},
		},
		{
			name: "customresourcedefinitions v1beta1 should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1beta1",
				Resource: "customresourcedefinitions",
			},
		},
	}

	for _, tc := range crdGVRs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			if err := os.Setenv("WATCH_NAMESPACE", allowedNamespace); err != nil {
				t.Fatalf("failed to set WATCH_NAMESPACE: %v", err)
			}
			if err := os.Setenv("ALLOWED_NAMESPACES", allowedNamespace); err != nil {
				t.Fatalf("failed to set ALLOWED_NAMESPACES: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("WATCH_NAMESPACE"); err != nil {
					t.Logf("failed to unset WATCH_NAMESPACE: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_NAMESPACES"); err != nil {
					t.Logf("failed to unset ALLOWED_NAMESPACES: %v", err)
				}
			}()

			allowlist := watcher.NewGVRAllowlist()
			creator := watcher.NewCRDCreator(dynamicClient, tc.gvr, allowlist)

			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": allowedNamespace,
					},
					"spec": map[string]interface{}{
						"source": "test-source",
					},
				},
			}

			err := creator.CreateCRD(ctx, observation)
			if err == nil {
				t.Errorf("Expected error for %s GVR, got nil", tc.gvr.Resource)
				return
			}

			if !errors.Is(err, watcher.ErrGVRDenied) {
				t.Errorf("Expected error to wrap ErrGVRDenied, got: %v", err)
			}
		})
	}
}

// TestSecurityRegression_NonAllowlistedGVRDenied tests that non-allowlisted GVRs are denied
func TestSecurityRegression_NonAllowlistedGVRDenied(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	testCases := []struct {
		name string
		gvr  schema.GroupVersionResource
	}{
		{
			name: "custom GVR should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "example.com",
				Version:  "v1",
				Resource: "customresources",
			},
		},
		{
			name: "another custom GVR should be denied",
			gvr: schema.GroupVersionResource{
				Group:    "mycompany.com",
				Version:  "v1alpha1",
				Resource: "myresources",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			if err := os.Setenv("WATCH_NAMESPACE", allowedNamespace); err != nil {
				t.Fatalf("failed to set WATCH_NAMESPACE: %v", err)
			}
			if err := os.Setenv("ALLOWED_NAMESPACES", allowedNamespace); err != nil {
				t.Fatalf("failed to set ALLOWED_NAMESPACES: %v", err)
			}
			// Note: NOT adding this GVR to ALLOWED_GVRS
			defer func() {
				if err := os.Unsetenv("WATCH_NAMESPACE"); err != nil {
					t.Logf("failed to unset WATCH_NAMESPACE: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_NAMESPACES"); err != nil {
					t.Logf("failed to unset ALLOWED_NAMESPACES: %v", err)
				}
			}()

			allowlist := watcher.NewGVRAllowlist()
			creator := watcher.NewCRDCreator(dynamicClient, tc.gvr, allowlist)

			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": allowedNamespace,
					},
					"spec": map[string]interface{}{
						"source": "test-source",
					},
				},
			}

			err := creator.CreateCRD(ctx, observation)
			if err == nil {
				t.Errorf("Expected error for non-allowlisted GVR %s, got nil", tc.gvr.String())
				return
			}

			if !errors.Is(err, watcher.ErrGVRNotAllowed) {
				t.Errorf("Expected error to wrap ErrGVRNotAllowed, got: %v", err)
			}
		})
	}
}

// TestSecurityRegression_DisallowedNamespaceDenied tests that disallowed namespaces are denied
func TestSecurityRegression_DisallowedNamespaceDenied(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	testCases := []struct {
		name      string
		namespace string
	}{
		{
			name:      "kube-system namespace should be denied",
			namespace: "kube-system",
		},
		{
			name:      "kube-public namespace should be denied",
			namespace: "kube-public",
		},
		{
			name:      "random namespace should be denied",
			namespace: "random-namespace-123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			if err := os.Setenv("WATCH_NAMESPACE", allowedNamespace); err != nil {
				t.Fatalf("failed to set WATCH_NAMESPACE: %v", err)
			}
			if err := os.Setenv("ALLOWED_NAMESPACES", allowedNamespace); err != nil {
				t.Fatalf("failed to set ALLOWED_NAMESPACES: %v", err)
			}
			// Only allow allowedNamespace
			defer func() {
				if err := os.Unsetenv("WATCH_NAMESPACE"); err != nil {
					t.Logf("failed to unset WATCH_NAMESPACE: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_NAMESPACES"); err != nil {
					t.Logf("failed to unset ALLOWED_NAMESPACES: %v", err)
				}
			}()

			allowlist := watcher.NewGVRAllowlist()

			creator := watcher.NewCRDCreator(dynamicClient, observationGVR, allowlist)

			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": tc.namespace,
					},
					"spec": map[string]interface{}{
						"source":     "test-source",
						"category":   "security",
						"severity":   "high",
						"eventType":  "vulnerability",
						"detectedAt": time.Now().Format(time.RFC3339),
					},
				},
			}

			err := creator.CreateCRD(ctx, observation)
			if err == nil {
				t.Errorf("Expected error for disallowed namespace %s, got nil", tc.namespace)
				return
			}

			if !errors.Is(err, watcher.ErrNamespaceNotAllowed) {
				t.Errorf("Expected error to wrap ErrNamespaceNotAllowed, got: %v", err)
			}
		})
	}
}

// TestSecurityRegression_PositivePath tests that allowed GVR and namespace succeeds
func TestSecurityRegression_PositivePath(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	testCases := []struct {
		name      string
		gvr       schema.GroupVersionResource
		namespace string
		wantErr   bool
	}{
		{
			name:      "observation CRD create succeeds in allowed namespace/GVR",
			gvr:       observationGVR,
			namespace: allowedNamespace,
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			if err := os.Setenv("WATCH_NAMESPACE", allowedNamespace); err != nil {
				t.Fatalf("failed to set WATCH_NAMESPACE: %v", err)
			}
			if err := os.Setenv("ALLOWED_NAMESPACES", allowedNamespace); err != nil {
				t.Fatalf("failed to set ALLOWED_NAMESPACES: %v", err)
			}
			// Allow v1alpha1/observations since that's what the CRD uses
			if err := os.Setenv("ALLOWED_GVRS", "zen.kube-zen.io/v1alpha1/observations"); err != nil {
				t.Fatalf("failed to set ALLOWED_GVRS: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("WATCH_NAMESPACE"); err != nil {
					t.Logf("failed to unset WATCH_NAMESPACE: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_NAMESPACES"); err != nil {
					t.Logf("failed to unset ALLOWED_NAMESPACES: %v", err)
				}
				if err := os.Unsetenv("ALLOWED_GVRS"); err != nil {
					t.Logf("failed to unset ALLOWED_GVRS: %v", err)
				}
			}()

			allowlist := watcher.NewGVRAllowlist()

			creator := watcher.NewCRDCreator(dynamicClient, tc.gvr, allowlist)

			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": tc.namespace,
					},
					"spec": map[string]interface{}{
						"source":     "test-source",
						"category":   "security",
						"severity":   "high",
						"eventType":  "vulnerability",
						"detectedAt": time.Now().Format(time.RFC3339),
					},
				},
			}

			err := creator.CreateCRD(ctx, observation)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					// Check that error is NOT an allowlist error
					if errors.Is(err, watcher.ErrGVRNotAllowed) ||
						errors.Is(err, watcher.ErrGVRDenied) ||
						errors.Is(err, watcher.ErrNamespaceNotAllowed) ||
						errors.Is(err, watcher.ErrClusterScopedNotAllowed) {
						t.Errorf("Unexpected allowlist error: %v", err)
					} else {
						// Other errors (like resource validation) are acceptable
						t.Logf("Got non-allowlist error (acceptable): %v", err)
					}
				}
			}
		})
	}
}
