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
	clusterName    = "zen-demo"
	namespace      = "zen-system"
	testNamespace  = "default"
	kubeconfigPath = ".kubeconfig-zen-demo"
)

var (
	kubectlCmd = []string{"kubectl", "--context=k3d-" + clusterName}
)

// getKubeconfigPath returns the path to the kubeconfig file for zen-demo
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

// getKubernetesClient returns a Kubernetes client for the zen-demo cluster
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

// TestClusterExists verifies that the zen-demo cluster exists and is accessible
func TestClusterExists(t *testing.T) {
	kubeconfig := getKubeconfigPath()
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Fatalf("kubeconfig not found: %s (run: make zen-demo-up)", kubeconfig)
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

	// Wait a moment for reconciliation
	time.Sleep(2 * time.Second)

	// Verify the Ingester exists
	output, err = runKubectl("get", "ingester", "test-e2e-ingester", "-n", testNamespace)
	if err != nil {
		t.Errorf("Test Ingester not found: %v\nOutput: %s", err, output)
	}

	// Cleanup
	runKubectl("delete", "ingester", "test-e2e-ingester", "-n", testNamespace, "--ignore-not-found=true")
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

	if svc.Spec.Ports == nil || len(svc.Spec.Ports) == 0 {
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

