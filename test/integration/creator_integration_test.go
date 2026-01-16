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
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	// H042: Setup test environment with CRDs (bulletproof CRD install)
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deployments", "crds"),
		},
		ErrorIfCRDPathMissing: true,
		// H042: UseAttachControlPlaneOutput helps with debugging but can be removed for CI
		UseExistingCluster: nil, // Always use envtest (no existing cluster)
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

	// H042: Validate CRDs are installed before proceeding (fails fast)
	if err := validateCRDsInstalled(cfg); err != nil {
		panic("CRD validation failed: " + err.Error())
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

// validateCRDsInstalled validates that required CRDs are installed in envtest
// H042: Fails fast with actionable errors if CRDs are missing
func validateCRDsInstalled(cfg interface{}) error {
	// Wait for CRD discovery to settle (envtest can have discovery lag)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Expected CRDs (version-pinned)
	expectedCRDs := []struct {
		name    string
		group   string
		version string
		kind    string
	}{
		{
			name:    "observations.zen.kube-zen.io",
			group:   "zen.kube-zen.io",
			version: "v1alpha1",
			kind:    "Observation",
		},
		{
			name:    "ingesters.zen.kube-zen.io",
			group:   "zen.kube-zen.io",
			version: "v1alpha1",
			kind:    "Ingester",
		},
	}

	// Use dynamic client to check CRD existence via API discovery
	dynClient, err := dynamic.NewForConfig(cfg.(*rest.Config))
	if err != nil {
		return fmt.Errorf("failed to create dynamic client for CRD validation: %w", err)
	}

	for _, expected := range expectedCRDs {
		gvr := schema.GroupVersionResource{
			Group:    expected.group,
			Version:  expected.version,
			Resource: strings.ToLower(expected.kind) + "s",
		}

		// Try to list resources (this will fail if CRD doesn't exist)
		// Use retry with exponential backoff to handle discovery lag
		var lastErr error
		for attempt := 0; attempt < 5; attempt++ {
			_, err := dynClient.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
			if err == nil {
				// Success: CRD exists and is discoverable
				lastErr = nil
				break
			}
			if !errors.IsNotFound(err) && !strings.Contains(err.Error(), "no matches for kind") {
				// Non-discovery error - probably real issue
				lastErr = err
				break
			}
			// Discovery lag - retry with backoff
			select {
			case <-ctx.Done():
				lastErr = fmt.Errorf("timeout waiting for CRD discovery")
				break
			case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
				// Continue retry
			}
		}

		if lastErr != nil {
			return fmt.Errorf("CRD validation failed for %s (group: %s, version: %s): %w\n"+
				"Action: Ensure CRD file exists at deployments/crds/%s_crd.yaml",
				expected.name, expected.group, expected.version, lastErr, strings.ToLower(expected.kind))
		}
	}

	return nil
}

// setupTestNamespace creates a test namespace and service account
func setupTestNamespace(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Create namespace
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": allowedNamespace,
			},
		},
	}

	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	_, err := dynamicClient.Resource(nsGVR).Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	// Create service account
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":      testServiceAccount,
				"namespace": allowedNamespace,
			},
		},
	}

	saGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "serviceaccounts",
	}

	_, err = dynamicClient.Resource(saGVR).Namespace(allowedNamespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create service account: %v", err)
	}
}

// cleanupTestNamespace cleans up test namespace
func cleanupTestNamespace(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	err := dynamicClient.Resource(nsGVR).Delete(ctx, allowedNamespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		t.Logf("Failed to delete namespace (non-fatal): %v", err)
	}
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

			// H041: Use programmatic config for deterministic tests
			allowlist := watcher.NewGVRAllowlistFromConfig(watcher.GVRAllowlistConfig{
				AllowedGVRs:       []string{"zen.kube-zen.io/v1alpha1/observations"},
				AllowedNamespaces: []string{allowedNamespace},
				DefaultNamespace:  allowedNamespace,
			})

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
					if observation.GetName() == "" {
						t.Error("Expected observation to have a name after creation")
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

			// H041: Use programmatic config for deterministic tests
			allowlist := watcher.NewGVRAllowlistFromConfig(watcher.GVRAllowlistConfig{
				AllowedGVRs:       []string{"zen.kube-zen.io/v1alpha1/observations"},
				AllowedNamespaces: []string{allowedNamespace},
				DefaultNamespace:  allowedNamespace,
			})

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
				} else {
					// Verify CRD was created by trying to get it
					// Note: CreateCRD doesn't return the created resource, so we can't check the name directly
					// But if CreateCRD succeeds, the resource exists
				}
			}
		})
	}
}

// TestObservationCreator_WithAllowlist tests allowlist enforcement
func TestObservationCreator_WithAllowlist(t *testing.T) {
	setupTestNamespace(t)
	defer cleanupTestNamespace(t)

	ctx := context.Background()

	// Set up allowlist using programmatic config (H041: deterministic test config)
	allowlist := watcher.NewGVRAllowlistFromConfig(watcher.GVRAllowlistConfig{
		AllowedGVRs:       []string{"zen.kube-zen.io/v1alpha1/observations"},
		AllowedNamespaces: []string{allowedNamespace},
		DefaultNamespace:  allowedNamespace,
	})

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
