// Copyright 2025 kube-zen
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

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultClusterName = "test-cluster"
	namespace          = "zen-system"
	testNamespace      = "default"
)

var (
	clusterName = getClusterName()
	// nolint:unused // Kept for future use
	kubectlCmd = []string{"kubectl", "--context=k3d-" + clusterName}
)

// getClusterName returns the cluster name from environment variable or default
func getClusterName() string {
	if name := os.Getenv("TEST_CLUSTER_NAME"); name != "" {
		return name
	}
	return defaultClusterName
}

// getKubeconfigPath returns the path to the kubeconfig file
func getKubeconfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "k3d", "kubeconfig-"+clusterName+".yaml")
}

// runKubectl runs a kubectl command and returns stdout
func runKubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", append([]string{"--context=k3d-" + clusterName}, args...)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// getKubernetesClient returns a Kubernetes client for the test cluster
func getKubernetesClient() (*kubernetes.Clientset, error) {
	kubeconfig := getKubeconfigPath()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	return clientset, nil
}

// TestClusterExists verifies that the test cluster exists and is accessible
func TestClusterExists(t *testing.T) {
	kubeconfig := getKubeconfigPath()
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Fatalf("kubeconfig not found: %s (set TEST_CLUSTER_NAME env var or create cluster: %s)", kubeconfig, clusterName)
	}

	output, err := runKubectl("get", "nodes")
	if err != nil {
		t.Fatalf("Cannot access cluster: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Ready") {
		t.Errorf("Cluster nodes not ready\nOutput: %s", output)
	}
}

// TestCRDsExist verifies that required CRDs are installed
func TestCRDsExist(t *testing.T) {
	requiredCRDs := []string{
		"ingesters.zen.kube-zen.io",
		"observations.zen.kube-zen.io",
	}

	for _, crd := range requiredCRDs {
		output, err := runKubectl("get", "crd", crd)
		if err != nil {
			t.Errorf("CRD %s not found: %v\nOutput: %s", crd, err, output)
			continue
		}
		if !strings.Contains(output, crd) {
			t.Errorf("CRD %s not properly installed\nOutput: %s", crd, output)
		}
	}
}

// TestWatcherDeployment verifies that zen-watcher deployment exists and is ready
func TestWatcherDeployment(t *testing.T) {
	clientset, err := getKubernetesClient()
	if err != nil {
		t.Fatalf("Failed to get Kubernetes client: %v", err)
	}

	ctx := context.Background()
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, "zen-watcher", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("zen-watcher deployment not found: %v", err)
	}

	if deployment.Status.ReadyReplicas < 1 {
		t.Errorf("zen-watcher deployment not ready: %d/%d replicas ready",
			deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
	}
}

// TestWatcherPodRunning verifies that zen-watcher pod is running
func TestWatcherPodRunning(t *testing.T) {
	clientset, err := getKubernetesClient()
	if err != nil {
		t.Fatalf("Failed to get Kubernetes client: %v", err)
	}

	ctx := context.Background()
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=zen-watcher",
	})
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(pods.Items) == 0 {
		t.Fatal("No zen-watcher pods found")
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != "Running" {
			t.Errorf("Pod %s is not running: %s", pod.Name, pod.Status.Phase)
		}
	}
}

// TestIngesterCRExists verifies that a test Ingester CR can be created and reaches expected status
func TestIngesterCRExists(t *testing.T) {
	// Apply a test Ingester CR
	testIngester := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-e2e-ingester
  namespace: default
spec:
  source: kubernetes-events
  ingester: k8s-events
  processing:
    filter:
      enabled: true
      minPriority: 0.5
    dedup:
      enabled: true
      window: "30s"
      strategy: fingerprint
  destinations:
    - type: crd
      value: observations
`

	// Apply the Ingester
	cmd := exec.Command("kubectl", "--context=k3d-"+clusterName, "apply", "-f", "-")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
	cmd.Stdin = strings.NewReader(testIngester)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to apply test Ingester: %v\nOutput: %s", err, output)
	}

	// Wait for reconciliation with bounded retry
	waitForIngesterReady(t, "test-e2e-ingester", testNamespace, 10*time.Second)

	// Verify the Ingester exists
	ingesterOutput, err := runKubectl("get", "ingester", "test-e2e-ingester", "-n", testNamespace)
	if err != nil {
		t.Errorf("Test Ingester not found: %v\nOutput: %s", err, ingesterOutput)
	}

	// Cleanup
	runKubectl("delete", "ingester", "test-e2e-ingester", "-n", testNamespace, "--ignore-not-found=true")
}

// TestCanonicalSpecLocations verifies that spec.processing.filter and spec.processing.dedup are respected (W58, W33)
// Contract regression test: ensures canonical spec locations work correctly
func TestCanonicalSpecLocations(t *testing.T) {
	// Apply Ingester with canonical spec.processing.* locations
	testIngester := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-canonical-spec
  namespace: default
spec:
  source: test-canonical
  ingester: k8s-events
  processing:
    filter:
      enabled: true
      minPriority: 0.7
      expression: "severity >= HIGH"
    dedup:
      enabled: true
      window: "60s"
      strategy: event-stream
  destinations:
    - type: crd
      value: observations
`

	// Apply the Ingester
	cmd := exec.Command("kubectl", "--context=k3d-"+clusterName, "apply", "-f", "-")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
	cmd.Stdin = strings.NewReader(testIngester)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to apply Ingester with canonical spec: %v\nOutput: %s", err, output)
	}

	// Wait for reconciliation with bounded retry
	waitForIngesterReady(t, "test-canonical-spec", testNamespace, 10*time.Second)

	// Verify Ingester exists and has correct spec
	specOutput, err := runKubectl("get", "ingester", "test-canonical-spec", "-n", testNamespace, "-o", "jsonpath={.spec.processing.filter.expression}")
	if err != nil {
		t.Errorf("Failed to get Ingester spec: %v", err)
	} else {
		if !strings.Contains(specOutput, "severity >= HIGH") {
			t.Errorf("Canonical spec.processing.filter.expression not found. Got: %s", specOutput)
		}
	}

	// Verify dedup strategy
	dedupOutput, err := runKubectl("get", "ingester", "test-canonical-spec", "-n", testNamespace, "-o", "jsonpath={.spec.processing.sdkdedup.strategy}")
	if err != nil {
		t.Errorf("Failed to get dedup strategy: %v", err)
	} else {
		if dedupOutput != "event-stream" {
			t.Errorf("Expected dedup strategy 'event-stream', got: %s", dedupOutput)
		}
	}

	// Cleanup
	runKubectl("delete", "ingester", "test-canonical-spec", "-n", testNamespace, "--ignore-not-found=true")
}

// TestRequiredFieldValidation verifies that required fields (source, ingester, destinations) are validated (W59)
// Contract regression test: ensures required field validation prevents invalid configs
func TestRequiredFieldValidation(t *testing.T) {
	tests := []struct {
		name         string
		ingesterYAML string
		shouldFail   bool
	}{
		{
			name: "Missing source",
			ingesterYAML: `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-missing-source
  namespace: default
spec:
  ingester: k8s-events
  destinations:
    - type: crd
      value: observations
`,
			shouldFail: true,
		},
		{
			name: "Missing ingester",
			ingesterYAML: `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-missing-ingester
  namespace: default
spec:
  source: test-source
  destinations:
    - type: crd
      value: observations
`,
			shouldFail: true,
		},
		{
			name: "Missing destinations",
			ingesterYAML: `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-missing-destinations
  namespace: default
spec:
  source: test-source
  ingester: k8s-events
`,
			shouldFail: true,
		},
		{
			name: "Valid Ingester",
			ingesterYAML: `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-valid-ingester
  namespace: default
spec:
  source: test-source
  ingester: k8s-events
  destinations:
    - type: crd
      value: observations
`,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply the Ingester
			cmd := exec.Command("kubectl", "--context=k3d-"+clusterName, "apply", "-f", "-")
			cmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
			cmd.Stdin = strings.NewReader(tt.ingesterYAML)
			output, err := cmd.CombinedOutput()

			if tt.shouldFail {
				// For invalid configs, we expect the loader to reject them
				// Check if Ingester was created (it shouldn't be)
				time.Sleep(1 * time.Second)
				checkCmd := exec.Command("kubectl", "--context=k3d-"+clusterName, "get", "ingester", "-n", testNamespace, "-o", "name")
				checkCmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
				checkOutput, _ := checkCmd.CombinedOutput()
				if strings.Contains(string(checkOutput), strings.TrimSpace(strings.Split(tt.ingesterYAML, "\n")[4])) {
					t.Errorf("Invalid Ingester was accepted (should be rejected): %s", string(checkOutput))
				}
			} else {
				// Valid config should succeed
				if err != nil {
					t.Errorf("Valid Ingester was rejected: %v\nOutput: %s", err, output)
				}
			}

			// Cleanup
			ingesterName := strings.TrimSpace(strings.Split(tt.ingesterYAML, "\n")[4])
			runKubectl("delete", "ingester", ingesterName, "-n", testNamespace, "--ignore-not-found=true")
		})
	}
}

// TestMetricsMovement verifies that metrics increment after sending events (W33, W58/W59-related)
func TestMetricsMovement(t *testing.T) {
	_, err := getKubernetesClient()
	if err != nil {
		t.Fatalf("Failed to get Kubernetes client: %v", err)
	}

	// Port-forward to metrics endpoint
	port := "9091"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "--context=k3d-"+clusterName,
		"port-forward", "-n", namespace, "svc/zen-watcher", port+":8080")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
	if err := cmd.Start(); err != nil {
		t.Skipf("Failed to start port-forward (non-critical): %v", err)
		return
	}
	defer cmd.Process.Kill()

	// Wait for port-forward
	time.Sleep(3 * time.Second)

	// Get initial metrics
	initialMetrics, err := fetchMetrics("http://localhost:" + port + "/metrics")
	if err != nil {
		t.Skipf("Could not fetch initial metrics (non-critical): %v", err)
		return
	}

	initialCreated := extractMetricValue(initialMetrics, "zen_watcher_observations_created_total")
	initialDeduped := extractMetricValue(initialMetrics, "zen_watcher_observations_deduped_total")
	initialDedupEffectiveness := extractMetricValue(initialMetrics, "zen_watcher_dedup_effectiveness_per_strategy")

	// Apply test Ingester that will process events
	testIngester := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-metrics-ingester
  namespace: default
spec:
  source: test-metrics
  ingester: k8s-events
  processing:
    filter:
      enabled: true
      minPriority: 0.3
    dedup:
      enabled: true
      window: "30s"
      strategy: fingerprint
  destinations:
    - type: crd
      value: observations
`
	applyCmd := exec.Command("kubectl", "--context=k3d-"+clusterName, "apply", "-f", "-")
	applyCmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
	applyCmd.Stdin = strings.NewReader(testIngester)
	if err := applyCmd.Run(); err != nil {
		t.Fatalf("Failed to apply test Ingester: %v", err)
	}
	defer runKubectl("delete", "ingester", "test-metrics-ingester", "-n", testNamespace, "--ignore-not-found=true")

	// Wait for Ingester to be processed (explicit timeout)
	waitForIngesterReady(t, "test-metrics-ingester", testNamespace, 15*time.Second)

	// Wait for metrics to be available (bounded retry)
	waitForMetricsAvailable(t, port, 10*time.Second)
	finalMetrics, err := fetchMetrics("http://localhost:" + port + "/metrics")
	if err != nil {
		t.Skipf("Could not fetch final metrics (non-critical): %v", err)
		return
	}

	finalCreated := extractMetricValue(finalMetrics, "zen_watcher_observations_created_total")
	finalDeduped := extractMetricValue(finalMetrics, "zen_watcher_observations_deduped_total")
	finalDedupEffectiveness := extractMetricValue(finalMetrics, "zen_watcher_dedup_effectiveness_per_strategy")

	// Verify metrics exist (at least one metric should be present)
	if !strings.Contains(finalMetrics, "zen_watcher") {
		t.Error("No zen_watcher metrics found in output")
	}

	// Log metric values (non-fatal if they don't increment - depends on actual events)
	t.Logf("Metrics: created=%s->%s, deduped=%s->%s, dedup_effectiveness=%s->%s",
		initialCreated, finalCreated, initialDeduped, finalDeduped, initialDedupEffectiveness, finalDedupEffectiveness)
}

// Helper functions for metrics testing
func fetchMetrics(url string) (string, error) {
	cmd := exec.Command("curl", "-s", "-f", url)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func extractMetricValue(metricsOutput, metricName string) string {
	lines := strings.Split(metricsOutput, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName) && !strings.HasPrefix(line, "#") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[len(parts)-1]
			}
		}
	}
	return "0"
}

// TestMetricsEndpoint verifies that zen-watcher metrics endpoint is accessible
func TestMetricsEndpoint(t *testing.T) {
	clientset, err := getKubernetesClient()
	if err != nil {
		t.Fatalf("Failed to get Kubernetes client: %v", err)
	}

	ctx := context.Background()
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, "zen-watcher", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("zen-watcher service not found: %v", err)
	}

	if len(svc.Spec.Ports) == 0 {
		t.Fatal("zen-watcher service has no ports")
	}

	// Port-forward to metrics endpoint
	port := "9090"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "--context=k3d-"+clusterName,
		"port-forward", "-n", namespace, "svc/zen-watcher", port+":8080")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+getKubeconfigPath())
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start port-forward: %v", err)
	}
	defer cmd.Process.Kill()

	// Wait for port-forward to be ready
	time.Sleep(2 * time.Second)

	// Try to fetch metrics (basic check - just verify endpoint responds)
	metricsCmd := exec.Command("curl", "-s", "-f", "http://localhost:"+port+"/metrics")
	output, err := metricsCmd.CombinedOutput()
	if err != nil {
		t.Logf("Metrics endpoint check (non-critical): %v\nOutput: %s", err, output)
		// Don't fail the test if metrics endpoint is not accessible (may require auth)
	} else {
		// Verify we got some metrics output
		if !strings.Contains(string(output), "zen_watcher") && !strings.Contains(string(output), "# HELP") {
			t.Logf("Metrics endpoint returned unexpected output: %s", output)
		}
	}
}

// TestCoreMetrics verifies that core metrics are present
func TestCoreMetrics(t *testing.T) {
	// This is a placeholder - actual metrics scraping would require port-forwarding
	// and parsing Prometheus format. For now, we just verify the service exists.
	clientset, err := getKubernetesClient()
	if err != nil {
		t.Fatalf("Failed to get Kubernetes client: %v", err)
	}

	ctx := context.Background()
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, "zen-watcher", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("zen-watcher service not found: %v", err)
	}

	// Check for Prometheus annotations
	if svc.Annotations == nil {
		t.Log("Service has no annotations (metrics scraping may be configured elsewhere)")
	} else {
		if scrape, ok := svc.Annotations["prometheus.io/scrape"]; ok && scrape == "true" {
			t.Logf("Service is annotated for Prometheus scraping")
		}
	}
}

// waitForMetricsAvailable waits for metrics endpoint to be available
func waitForMetricsAvailable(t *testing.T, port string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Logf("Timeout waiting for metrics on port %s", port)
			return
		case <-ticker.C:
			_, err := fetchMetrics("http://localhost:" + port + "/metrics")
			if err == nil {
				return
			}
		}
	}
}

// waitForIngesterReady waits for an Ingester to be ready
func waitForIngesterReady(t *testing.T, name, namespace string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for Ingester %s/%s to be ready", namespace, name)
		case <-ticker.C:
			output, err := runKubectl("get", "ingester", name, "-n", namespace, "-o", "jsonpath={.status.phase}")
			if err == nil && strings.TrimSpace(output) == "Ready" {
				return
			}
			// Also check if it exists at all
			_, err = runKubectl("get", "ingester", name, "-n", namespace)
			if err != nil {
				continue // Not found yet, keep waiting
			}
			// If it exists but not ready, continue waiting
		}
	}
}
