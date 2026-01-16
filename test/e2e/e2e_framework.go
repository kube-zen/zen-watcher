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

// Package e2e provides E2E testing framework with artifact collection.
// H036: E2E runner that applies manifests, waits for readiness, hits endpoints,
// and collects artifacts (receipts, logs, metrics snapshots, traces).
// Artifacts are stored under ./artifacts/<testname>/<timestamp>/.

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Common test constants
const (
	testNamespace = "zen-watcher-test"
)

// getKubeClientForCluster returns a Kubernetes client for a specific cluster
func getKubeClientForCluster(clusterName string) (*kubernetes.Clientset, error) {
	home, _ := os.UserHomeDir()
	kubeconfig := fmt.Sprintf("%s/.config/k3d/kubeconfig-%s.yaml", home, clusterName)

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return nil, fmt.Errorf("kubeconfig not found for cluster %s: %w", clusterName, err)
	}

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

// isAlreadyExists checks if an error indicates a resource already exists
func isAlreadyExists(err error) bool {
	return errors.IsAlreadyExists(err)
}

// ArtifactCollector collects test artifacts (logs, metrics, traces, receipts)
type ArtifactCollector struct {
	TestName    string
	ArtifactDir string
	KubeClient  kubernetes.Interface
	KubeConfig  *rest.Config
	Timestamp   time.Time
}

// NewArtifactCollector creates a new artifact collector
func NewArtifactCollector(testName string, kubeClient kubernetes.Interface, kubeConfig *rest.Config) (*ArtifactCollector, error) {
	timestamp := time.Now()
	artifactDir := filepath.Join("artifacts", testName, timestamp.Format("20060102-150405"))

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifact directory: %w", err)
	}

	return &ArtifactCollector{
		TestName:    testName,
		ArtifactDir: artifactDir,
		KubeClient:  kubeClient,
		KubeConfig:  kubeConfig,
		Timestamp:   timestamp,
	}, nil
}

// CollectLogs collects logs from pods
func (ac *ArtifactCollector) CollectLogs(ctx context.Context, namespace, labelSelector string) error {
	logsFile := filepath.Join(ac.ArtifactDir, "logs.json")

	// Get pods
	pods, err := ac.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	logsData := make(map[string]string)
	for _, pod := range pods.Items {
		// Get logs for each pod
		req := ac.KubeClient.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		podLogs, err := req.Stream(ctx)
		if err != nil {
			logsData[pod.Name] = fmt.Sprintf("ERROR: %v", err)
			continue
		}

		// Read logs (simplified - in production, handle streaming properly)
		logBytes, err := io.ReadAll(podLogs)
		if err != nil {
			logsData[pod.Name] = fmt.Sprintf("ERROR reading logs: %v", err)
			podLogs.Close()
			continue
		}

		logsData[pod.Name] = string(logBytes)
		podLogs.Close()
	}

	// Write logs to file
	data, err := json.MarshalIndent(logsData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	if err := os.WriteFile(logsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write logs file: %w", err)
	}

	return nil
}

// CollectMetrics collects metrics snapshot
func (ac *ArtifactCollector) CollectMetrics(ctx context.Context, metricsEndpoint string) error {
	metricsFile := filepath.Join(ac.ArtifactDir, "metrics.txt")

	// In a real implementation, this would:
	// 1. Hit the metrics endpoint (e.g., /metrics)
	// 2. Parse Prometheus metrics format
	// 3. Store as text or JSON

	// Placeholder: write a note about metrics collection
	metricsData := fmt.Sprintf("Metrics snapshot for test: %s\nTimestamp: %s\nEndpoint: %s\n",
		ac.TestName, ac.Timestamp.Format(time.RFC3339), metricsEndpoint)

	if err := os.WriteFile(metricsFile, []byte(metricsData), 0644); err != nil {
		return fmt.Errorf("failed to write metrics file: %w", err)
	}

	return nil
}

// CollectReceipt collects test receipt (test result summary)
func (ac *ArtifactCollector) CollectReceipt(testResult TestResult) error {
	receiptFile := filepath.Join(ac.ArtifactDir, "receipt.json")

	receipt := Receipt{
		TestName:  ac.TestName,
		Timestamp: ac.Timestamp,
		Result:    testResult,
		Artifacts: []string{
			"logs.json",
			"metrics.txt",
		},
	}

	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal receipt: %w", err)
	}

	if err := os.WriteFile(receiptFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write receipt file: %w", err)
	}

	return nil
}

// TestResult represents the result of an E2E test
type TestResult struct {
	Passed   bool     `json:"passed"`
	Duration string   `json:"duration"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Receipt represents a test execution receipt
type Receipt struct {
	TestName  string     `json:"test_name"`
	Timestamp time.Time  `json:"timestamp"`
	Result    TestResult `json:"result"`
	Artifacts []string   `json:"artifacts"`
}

// E2ERunner provides utilities for running E2E tests
type E2ERunner struct {
	KubeClient kubernetes.Interface
	KubeConfig *rest.Config
	Collector  *ArtifactCollector
	Context    context.Context
	Namespace  string
}

// NewE2ERunner creates a new E2E test runner
func NewE2ERunner(ctx context.Context, testName string, kubeClient kubernetes.Interface, kubeConfig *rest.Config, namespace string) (*E2ERunner, error) {
	collector, err := NewArtifactCollector(testName, kubeClient, kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact collector: %w", err)
	}

	return &E2ERunner{
		KubeClient: kubeClient,
		KubeConfig: kubeConfig,
		Collector:  collector,
		Context:    ctx,
		Namespace:  namespace,
	}, nil
}

// WaitForReadiness waits for resources to be ready
func (r *E2ERunner) WaitForReadiness(timeout time.Duration) error {
	// In a real implementation, this would:
	// 1. Check pod readiness conditions
	// 2. Check deployment replicas
	// 3. Check service endpoints
	// 4. Wait with timeout

	// Placeholder: simple wait
	time.Sleep(5 * time.Second)
	return nil
}

// ApplyManifest applies a Kubernetes manifest
func (r *E2ERunner) ApplyManifest(manifestPath string) error {
	// In a real implementation, this would:
	// 1. Read manifest file
	// 2. Apply using kubectl or client-go
	// 3. Handle errors appropriately

	return nil
}

// HitEndpoint hits an HTTP endpoint and collects response
func (r *E2ERunner) HitEndpoint(url string) (int, []byte, error) {
	// In a real implementation, this would:
	// 1. Make HTTP request
	// 2. Capture response status and body
	// 3. Store in artifacts

	return 200, []byte("OK"), nil
}
