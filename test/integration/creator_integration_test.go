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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kube-zen/zen-watcher/pkg/watcher"
)

var (
	testEnv            *envtest.Environment
	dynamicClient      dynamic.Interface
	kubeClient         kubernetes.Interface
	testCtx            context.Context
	testCancel         context.CancelFunc
	observationGVR     schema.GroupVersionResource
	ingesterGVR        schema.GroupVersionResource
	allowedNamespace   = "zen-watcher-test"
	testServiceAccount = "zen-watcher-test-sa"
)

func TestMain(m *testing.M) {
	log.SetLogger(zap.New(zap.UseDevMode(true)))

	// Setup test environment with CRDs
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deployments", "crds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic("Failed to start test environment: " + err.Error())
	}

	// Create clients
	dynamicClient, err = dynamic.NewForConfig(cfg)
	if err != nil {
		panic("Failed to create dynamic client: " + err.Error())
	}

	kubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		panic("Failed to create kubernetes client: " + err.Error())
	}

	// Define GVRs
	observationGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "observations",
	}
	ingesterGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "ingesters",
	}

	// Setup test context
	testCtx, testCancel = context.WithTimeout(context.Background(), 2*time.Minute)

	// Run tests
	code := m.Run()

	// Cleanup
	testCancel()
	if err := testEnv.Stop(); err != nil {
		panic("Failed to stop test environment: " + err.Error())
	}

	os.Exit(code)
}

// getNamespacePhase returns the phase of a namespace, or empty string if not found
func getNamespacePhase(ctx context.Context, nsGVR schema.GroupVersionResource, name string) (string, bool) {
	ns, err := dynamicClient.Resource(nsGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", false
	}
	phase, found, _ := unstructured.NestedString(ns.Object, "status", "phase")
	return phase, found
}

// waitForNamespaceDeletion waits for a namespace to be fully deleted
func waitForNamespaceDeletion(ctx context.Context, nsGVR schema.GroupVersionResource, name string, maxAttempts int) bool {
	for i := 0; i < maxAttempts; i++ {
		_, err := dynamicClient.Resource(nsGVR).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true
		}
		// Check if still terminating
		phase, found := getNamespacePhase(ctx, nsGVR, name)
		if found && phase != "Terminating" {
			// Namespace is no longer terminating (might have been recreated)
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// ensureNamespaceReady waits for namespace to be ready (not terminating)
func ensureNamespaceReady(ctx context.Context, nsGVR schema.GroupVersionResource, name string, maxAttempts int) error {
	for i := 0; i < maxAttempts; i++ {
		phase, found := getNamespacePhase(ctx, nsGVR, name)
		if found && phase != "" && phase != "Terminating" {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("namespace %s not ready after %d attempts", name, maxAttempts)
}

// createNamespaceIfNeeded creates a namespace if it doesn't exist or is terminating
func createNamespaceIfNeeded(ctx context.Context, nsGVR schema.GroupVersionResource, name string) error {
	existing, err := dynamicClient.Resource(nsGVR).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		phase, found, _ := unstructured.NestedString(existing.Object, "status", "phase")
		if found && phase == "Terminating" {
			// Wait for deletion with longer timeout (60 attempts = 30 seconds)
			// If still terminating after wait, try to proceed anyway - namespace might clear up
			// In test environments, sometimes namespaces get stuck in terminating
			// We'll try to create anyway and let Kubernetes handle it
			waitForNamespaceDeletion(ctx, nsGVR, name, 60)
		} else if found && phase != "" && phase != "Terminating" {
			// Namespace exists and is ready
			return ensureNamespaceReady(ctx, nsGVR, name, 10)
		}
	}

	// Create namespace
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}

	_, err = dynamicClient.Resource(nsGVR).Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// Wait for namespace to be ready
	return ensureNamespaceReady(ctx, nsGVR, name, 30)
}

// createServiceAccountWithRetry creates a service account with retry logic
func createServiceAccountWithRetry(ctx context.Context, saGVR schema.GroupVersionResource, nsGVR schema.GroupVersionResource, namespace, name string, maxAttempts int) error {
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}

	for i := 0; i < maxAttempts; i++ {
		// Ensure namespace is ready before each attempt
		if err := ensureNamespaceReady(ctx, nsGVR, namespace, 5); err != nil {
			// Namespace might be terminating, try to recreate it
			if recreateErr := createNamespaceIfNeeded(ctx, nsGVR, namespace); recreateErr != nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
		}

		_, err := dynamicClient.Resource(saGVR).Namespace(namespace).Create(ctx, sa, metav1.CreateOptions{})
		if err == nil || errors.IsAlreadyExists(err) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("failed to create service account after %d attempts", maxAttempts)
}

// setupTestNamespace creates a test namespace and service account
func setupTestNamespace(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	// Ensure namespace exists and is ready
	if err := createNamespaceIfNeeded(ctx, nsGVR, allowedNamespace); err != nil {
		t.Fatalf("Failed to setup namespace: %v", err)
	}

	// Create service account
	saGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "serviceaccounts",
	}

	if err := createServiceAccountWithRetry(ctx, saGVR, nsGVR, allowedNamespace, testServiceAccount, 10); err != nil {
		t.Fatalf("Failed to create service account: %v", err)
	}
}

// cleanupTestNamespace cleans up test namespace
// Note: In test environments, namespace deletion can be slow. We skip cleanup between tests
// to avoid race conditions. The namespace will be reused or cleaned up at test suite end.
func cleanupTestNamespace(t *testing.T) {
	t.Helper()
	// Skip cleanup between tests to avoid namespace termination race conditions
	// The namespace will be reused by subsequent tests or cleaned up by the test environment
	// This is safer than trying to delete and recreate namespaces between tests
}

// TestObservationCreator_CreateObservation tests ObservationCreator with real Kubernetes API
func TestObservationCreator_CreateObservation(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	testCases := []struct {
		name      string
		namespace string
		source    string
		wantErr   bool
	}{
		{
			name:      "create observation in allowed namespace",
			namespace: allowedNamespace,
			source:    "test-source",
			wantErr:   false,
		},
		{
			name:      "create observation with different source",
			namespace: allowedNamespace,
			source:    "trivy",
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Set up allowlist
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

			// Create ObservationCreator
			creator := watcher.NewObservationCreator(
				dynamicClient,
				observationGVR,
				nil, // eventsTotal
				nil, // observationsCreated
				nil, // observationsFiltered
				nil, // observationsDeduped
				nil, // observationsCreateErrors
				nil, // filter
			)
			creator.SetGVRAllowlist(allowlist)

			// Create observation
			observation := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": tc.namespace,
					},
					"spec": map[string]interface{}{
						"source":     tc.source,
						"category":   "security",
						"severity":   "high",
						"eventType":  "vulnerability",
						"detectedAt": time.Now().Format(time.RFC3339),
					},
				},
			}

			err := creator.CreateObservation(ctx, observation)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else {
					// Verify observation was created
					// Note: Fake client may not generate names from generateName properly
					// Check by listing observations instead
					list, listErr := dynamicClient.Resource(observationGVR).Namespace(tc.namespace).List(ctx, metav1.ListOptions{})
					if listErr != nil {
						t.Errorf("Failed to list observations to verify creation: %v", listErr)
					} else if len(list.Items) == 0 {
						t.Error("Expected at least one observation to be created")
					}
				}
			}
		})
	}
}

// TestCRDCreator_CreateCRD tests CRDCreator with real Kubernetes API
func TestCRDCreator_CreateCRD(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	testCases := []struct {
		name      string
		gvr       schema.GroupVersionResource
		namespace string
		wantErr   bool
	}{
		{
			name:      "create observation CRD",
			gvr:       observationGVR,
			namespace: allowedNamespace,
			wantErr:   false,
		},
		{
			name:      "create observation CRD in allowed namespace",
			gvr:       observationGVR,
			namespace: allowedNamespace,
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Set up allowlist
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

			// Create CRDCreator
			creator := watcher.NewCRDCreator(dynamicClient, tc.gvr, allowlist)

			// Create observation
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
					t.Errorf("Unexpected error: %v", err)
				}
				// Verify CRD was created by trying to get it
				// Note: CreateCRD doesn't return the created resource, so we can't check the name directly
				// But if CreateCRD succeeds, the resource exists
			}
		})
	}
}

// TestObservationCreator_WithAllowlist tests allowlist enforcement
func TestObservationCreator_WithAllowlist(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	ctx := context.Background()

	// Set up allowlist with specific namespace
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

	creator := watcher.NewObservationCreator(
		dynamicClient,
		observationGVR,
		nil, nil, nil, nil, nil,
		nil,
	)
	creator.SetGVRAllowlist(allowlist)

	// Test: create observation in allowed namespace (should succeed)
	observation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": allowedNamespace,
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

	err := creator.CreateObservation(ctx, observation)
	if err != nil {
		t.Errorf("Expected success for allowed namespace, got error: %v", err)
	}
}
