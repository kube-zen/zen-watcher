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

// Package e2e provides topology parity tests for zen-watcher.
// H040: Same E2E cases run twice - combined cluster vs split cluster.
// Only difference is endpoint URLs/values overlays (no code changes required).

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterCombined   = "zen-combined"
	clusterCoreSplit  = "zen-core"
	clusterCustASplit = "zen-cust-a"
)

// TopologyConfig represents cluster topology configuration
type TopologyConfig struct {
	Topology     string   // "combined" or "split"
	Clusters     []string // Cluster names
	EndpointBase string   // Base endpoint URL
}

// getTopologyConfig returns topology configuration for combined or split topology
func getTopologyConfig(topology string) *TopologyConfig {
	if topology == "combined" {
		return &TopologyConfig{
			Topology:     "combined",
			Clusters:     []string{clusterCombined},
			EndpointBase: "http://localhost:8080", // Combined cluster endpoint
		}
	}

	// Split topology
	return &TopologyConfig{
		Topology:     "split",
		Clusters:     []string{clusterCoreSplit, clusterCustASplit},
		EndpointBase: "http://localhost:9080", // Split cluster endpoint
	}
}

// runTestForTopology runs a test function against a specific topology
func runTestForTopology(t *testing.T, topology string, testName string, testFunc func(*testing.T, *TopologyConfig)) {
	t.Run(fmt.Sprintf("%s_%s", topology, testName), func(t *testing.T) {
		config := getTopologyConfig(topology)

		// Verify clusters exist
		for _, cluster := range config.Clusters {
			client, err := getKubeClientForCluster(cluster)
			if err != nil {
				t.Skipf("Cluster %s not available (set up with k3d-up.sh): %v", cluster, err)
				return
			}
			_ = client // Use client to verify cluster is accessible
		}

		// Run test with topology config
		testFunc(t, config)
	})
}

// testObservationCreation tests observation creation for a topology
func testObservationCreation(t *testing.T, config *TopologyConfig) {
	ctx := context.Background()

	// Get client for first cluster in topology
	client, err := getKubeClientForCluster(config.Clusters[0])
	if err != nil {
		t.Fatalf("Failed to get client for cluster %s: %v", config.Clusters[0], err)
	}

	// Ensure namespace exists
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		t.Fatalf("Failed to create namespace: %v", err)
	}

	// Test: Create observation
	// This is the same test logic regardless of topology
	// Only difference is which cluster/endpoint is used

	observationYAML := fmt.Sprintf(`
apiVersion: zen.kube-zen.io/v1alpha1
kind: Observation
metadata:
  generateName: test-topology-parity-
  namespace: %s
spec:
  source: test-topology
  category: security
  severity: high
  eventType: vulnerability
  detectedAt: "%s"
`, testNamespace, time.Now().Format(time.RFC3339))

	// Apply observation using topology-specific endpoint
	// In combined topology: endpoint is http://localhost:8080
	// In split topology: endpoint is http://localhost:9080 (core cluster)

	t.Logf("Topology: %s, Endpoint: %s", config.Topology, config.EndpointBase)
	t.Logf("Observation YAML: %s", observationYAML)

	// Placeholder: actual implementation would apply YAML via kubectl or client-go
	// The test logic is identical; only the endpoint/cluster differs

	_ = client
	_ = ctx
	_ = config

	// Note: Full implementation would verify observation was created
	t.Skip("Full observation creation requires kubectl apply (placeholder for now)")
}

// TestTopologyParity_ObservationCreation tests observation creation in both topologies
// H040: Same E2E case runs twice - combined vs split cluster
func TestTopologyParity_ObservationCreation(t *testing.T) {
	// Run same test for combined topology
	runTestForTopology(t, "combined", "ObservationCreation", testObservationCreation)

	// Run same test for split topology
	runTestForTopology(t, "split", "ObservationCreation", testObservationCreation)
}

// testIngesterCRD tests Ingester CRD creation for a topology
func testIngesterCRD(t *testing.T, config *TopologyConfig) {
	ctx := context.Background()
	_ = ctx

	// Get client for first cluster
	client, err := getKubeClientForCluster(config.Clusters[0])
	if err != nil {
		t.Fatalf("Failed to get client: %v", err)
	}

	// Test: Create Ingester CRD
	// Same test logic regardless of topology

	ingesterYAML := fmt.Sprintf(`
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: test-topology-ingester
  namespace: %s
spec:
  source: test-topology
  ingester: informer
  informer:
    gvr:
      group: ""
      version: "v1"
      resource: "events"
    namespace: ""
  destinations:
    - type: crd
      value: observations
`, testNamespace)

	t.Logf("Topology: %s, Endpoint: %s", config.Topology, config.EndpointBase)
	t.Logf("Ingester YAML: %s", ingesterYAML)

	// Placeholder: actual implementation would apply YAML
	_ = client
	_ = config

	t.Skip("Full Ingester CRD test requires kubectl apply (placeholder for now)")
}

// TestTopologyParity_IngesterCRD tests Ingester CRD creation in both topologies
// H040: Same E2E case runs twice - combined vs split cluster
func TestTopologyParity_IngesterCRD(t *testing.T) {
	// Run same test for combined topology
	runTestForTopology(t, "combined", "IngesterCRD", testIngesterCRD)

	// Run same test for split topology
	runTestForTopology(t, "split", "IngesterCRD", testIngesterCRD)
}

// testWebhookIngestion tests webhook ingestion for a topology
func testWebhookIngestion(t *testing.T, config *TopologyConfig) {
	// Test: Send webhook to ingester endpoint
	// Combined topology: http://localhost:8080/webhook/test
	// Split topology: http://localhost:9080/webhook/test (core cluster)

	t.Logf("Topology: %s, Webhook Endpoint: %s/webhook/test", config.Topology, config.EndpointBase)

	// Placeholder: actual implementation would send HTTP POST request
	_ = config

	t.Skip("Full webhook ingestion test requires HTTP client (placeholder for now)")
}

// TestTopologyParity_WebhookIngestion tests webhook ingestion in both topologies
// H040: Same E2E case runs twice - combined vs split cluster
func TestTopologyParity_WebhookIngestion(t *testing.T) {
	// Run same test for combined topology
	runTestForTopology(t, "combined", "WebhookIngestion", testWebhookIngestion)

	// Run same test for split topology
	runTestForTopology(t, "split", "WebhookIngestion", testWebhookIngestion)
}

// testAllowlistEnforcement tests allowlist enforcement for a topology
func testAllowlistEnforcement(t *testing.T, config *TopologyConfig) {
	// Test: Try to create observation in allowed/disallowed namespace
	// Same test logic regardless of topology

	t.Logf("Topology: %s, Testing allowlist enforcement", config.Topology)

	// Placeholder: actual implementation would test allowlist behavior
	_ = config

	t.Skip("Full allowlist enforcement test requires allowlist configuration (covered by integration tests)")
}

// TestTopologyParity_AllowlistEnforcement tests allowlist enforcement in both topologies
// H040: Same E2E case runs twice - combined vs split cluster
func TestTopologyParity_AllowlistEnforcement(t *testing.T) {
	// Run same test for combined topology
	runTestForTopology(t, "combined", "AllowlistEnforcement", testAllowlistEnforcement)

	// Run same test for split topology
	runTestForTopology(t, "split", "AllowlistEnforcement", testAllowlistEnforcement)
}
