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
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TrivyWatcher watches for Trivy updates and triggers actions
type TrivyWatcher struct {
	clientSet     *kubernetes.Clientset
	namespace     string
	actionHandler ActionHandler
}

// ActionHandler interface for handling detected events
type ActionHandler interface {
	HandleTrivyUpdate(ctx context.Context, logLine string) error
}

// NewTrivyWatcher creates a new Trivy watcher
func NewTrivyWatcher(clientSet *kubernetes.Clientset, namespace string, actionHandler ActionHandler) *TrivyWatcher {
	return &TrivyWatcher{
		clientSet:     clientSet,
		namespace:     namespace,
		actionHandler: actionHandler,
	}
}

// WatchPods watches for Trivy pods and their logs
func (tw *TrivyWatcher) WatchPods(ctx context.Context) error {
	// List all pods in the Trivy namespace first
	allPods, err := tw.clientSet.CoreV1().Pods(tw.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods in namespace %s: %v", tw.namespace, err)
	}

	if len(allPods.Items) == 0 {
		return fmt.Errorf("no pods found in namespace %s", tw.namespace)
	}

	logger.Debug("Found pods in namespace",
		logger.Fields{
			Component: "watcher",
			Operation: "watch_trivy_pods",
			Source:    "trivy",
			Namespace: tw.namespace,
			Count:     len(allPods.Items),
		})

	// Look for Trivy-related pods (more flexible matching)
	var trivyPods []corev1.Pod
	for _, pod := range allPods.Items {
		if tw.isTrivyPod(pod) {
			trivyPods = append(trivyPods, pod)
		}
	}

	if len(trivyPods) == 0 {
		return fmt.Errorf("no Trivy-related pods found in namespace %s", tw.namespace)
	}

	// Watch logs from the first Trivy pod
	pod := trivyPods[0]
	logger.Info("Watching logs from Trivy pod",
		logger.Fields{
			Component: "watcher",
			Operation: "watch_trivy_pods",
			Source:    "trivy",
			Namespace: tw.namespace,
			Additional: map[string]interface{}{
				"pod_name": pod.Name,
			},
		})

	return tw.watchPodLogs(ctx, pod.Name)
}

// WatchVulnerabilityReports watches for new VulnerabilityReport resources
func (tw *TrivyWatcher) WatchVulnerabilityReports(ctx context.Context) error {
	logger.Info("Watching VulnerabilityReport resources",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_vulnerability_reports",
			Source:       "trivy",
			Namespace:    tw.namespace,
			ResourceKind: "VulnerabilityReport",
		})

	// List existing VulnerabilityReports using the Trivy operator API
	reports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "trivy-operator.resource.kind=VulnerabilityReport",
	})
	if err != nil {
		return fmt.Errorf("failed to list VulnerabilityReports: %v", err)
	}

	logger.Debug("Found existing VulnerabilityReports",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_vulnerability_reports",
			Source:       "trivy",
			Namespace:    tw.namespace,
			ResourceKind: "VulnerabilityReport",
			Count:        len(reports.Items),
		})

	// Process existing reports
	for _, report := range reports.Items {
		if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
			logger.Error("Failed to process VulnerabilityReport",
				logger.Fields{
					Component:    "watcher",
					Operation:    "process_vulnerability_report",
					Source:       "trivy",
					Namespace:    tw.namespace,
					ResourceKind: "VulnerabilityReport",
					ResourceName: report.Name,
					Error:        err,
				})
		}
	}

	// Note: Real-time watching of VulnerabilityReports is implemented via informers
	// in internal/kubernetes/informers.go. This method processes existing reports
	// as a one-time operation. For continuous watching, use the informer-based approach.

	return nil
}

// WatchTrivyResources watches for Trivy security resources
func (tw *TrivyWatcher) WatchTrivyResources(ctx context.Context) error {
	logger.Info("Watching Trivy security resources",
		logger.Fields{
			Component: "watcher",
			Operation: "watch_trivy_resources",
			Source:    "trivy",
			Namespace: tw.namespace,
		})

	// List VulnerabilityReports
	logger.Debug("Checking VulnerabilityReports",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_trivy_resources",
			Source:       "trivy",
			ResourceKind: "VulnerabilityReport",
		})
	vulnReports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "trivy-operator.resource.kind=VulnerabilityReport",
	})
	if err != nil {
		logger.Error("Failed to list VulnerabilityReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_trivy_resources",
				Source:       "trivy",
				ResourceKind: "VulnerabilityReport",
				Error:        err,
			})
	} else {
		logger.Debug("Found VulnerabilityReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_trivy_resources",
				Source:       "trivy",
				ResourceKind: "VulnerabilityReport",
				Count:        len(vulnReports.Items),
			})
		for _, report := range vulnReports.Items {
			if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
				logger.Error("Failed to process VulnerabilityReport",
					logger.Fields{
						Component:    "watcher",
						Operation:    "process_vulnerability_report",
						Source:       "trivy",
						ResourceKind: "VulnerabilityReport",
						ResourceName: report.Name,
						Error:        err,
					})
			}
		}
	}

	// List ClusterVulnerabilityReports
	logger.Debug("Checking ClusterVulnerabilityReports",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_trivy_resources",
			Source:       "trivy",
			ResourceKind: "ClusterVulnerabilityReport",
		})
	clusterVulnReports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "trivy-operator.resource.kind=ClusterVulnerabilityReport",
	})
	if err != nil {
		logger.Error("Failed to list ClusterVulnerabilityReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_trivy_resources",
				Source:       "trivy",
				ResourceKind: "ClusterVulnerabilityReport",
				Error:        err,
			})
	} else {
		logger.Debug("Found ClusterVulnerabilityReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_trivy_resources",
				Source:       "trivy",
				ResourceKind: "ClusterVulnerabilityReport",
				Count:        len(clusterVulnReports.Items),
			})
		for _, report := range clusterVulnReports.Items {
			if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
				logger.Error("Failed to process ClusterVulnerabilityReport",
					logger.Fields{
						Component:    "watcher",
						Operation:    "process_vulnerability_report",
						Source:       "trivy",
						ResourceKind: "ClusterVulnerabilityReport",
						ResourceName: report.Name,
						Error:        err,
					})
			}
		}
	}

	// List ConfigAuditReports
	logger.Debug("Checking ConfigAuditReports",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_trivy_resources",
			Source:       "trivy",
			ResourceKind: "ConfigAuditReport",
		})
	configAuditReports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "trivy-operator.resource.kind=ConfigAuditReport",
	})
	if err != nil {
		logger.Error("Failed to list ConfigAuditReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_trivy_resources",
				Source:       "trivy",
				ResourceKind: "ConfigAuditReport",
				Error:        err,
			})
	} else {
		logger.Debug("Found ConfigAuditReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_trivy_resources",
				Source:       "trivy",
				ResourceKind: "ConfigAuditReport",
				Count:        len(configAuditReports.Items),
			})
		for _, report := range configAuditReports.Items {
			if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
				logger.Error("Failed to process ConfigAuditReport",
					logger.Fields{
						Component:    "watcher",
						Operation:    "process_vulnerability_report",
						Source:       "trivy",
						ResourceKind: "ConfigAuditReport",
						ResourceName: report.Name,
						Error:        err,
					})
			}
		}
	}

	return nil
}

// isTrivyPod checks if a pod is Trivy-related
func (tw *TrivyWatcher) isTrivyPod(pod corev1.Pod) bool {
	// Specifically look for the real Trivy operator pod
	podName := strings.ToLower(pod.Name)

	// Check for the actual Trivy operator pod name pattern
	if strings.Contains(podName, "my-trivy-operator") {
		return true
	}

	// Check for Trivy operator labels
	for key, value := range pod.Labels {
		keyLower := strings.ToLower(key)
		valueLower := strings.ToLower(value)

		if strings.Contains(keyLower, "trivy-operator") || strings.Contains(valueLower, "trivy-operator") {
			return true
		}
	}

	return false
}

// WatchPodLogs watches logs from a specific pod
func (tw *TrivyWatcher) WatchPodLogs(ctx context.Context, podName string) error {
	return tw.watchPodLogs(ctx, podName)
}

func (tw *TrivyWatcher) watchPodLogs(ctx context.Context, podName string) error {
	// Stream logs from the pod
	req := tw.clientSet.CoreV1().Pods(tw.namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})

	logsStream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream: %v", err)
	}
	defer logsStream.Close()

	scanner := bufio.NewScanner(logsStream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		logger.Debug("Trivy log line",
			logger.Fields{
				Component: "watcher",
				Operation: "watch_trivy_logs",
				Source:    "trivy",
				Additional: map[string]interface{}{
					"log_line": line,
				},
			})

		// Check for Trivy-specific patterns
		if tw.isTrivyUpdate(line) {
			logger.Info("Detected Trivy update, triggering action",
				logger.Fields{
					Component: "watcher",
					Operation: "watch_trivy_logs",
					Source:    "trivy",
					EventType: "trivy_update_detected",
				})
			if err := tw.actionHandler.HandleTrivyUpdate(ctx, line); err != nil {
				logger.Error("Failed to handle Trivy update",
					logger.Fields{
						Component: "watcher",
						Operation: "handle_trivy_update",
						Source:    "trivy",
						Error:     err,
					})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}

// isTrivyUpdate checks if a log line indicates a Trivy update
func (tw *TrivyWatcher) isTrivyUpdate(line string) bool {
	// Look for patterns that indicate Trivy updates
	// Adjust these patterns based on your Trivy setup
	updatePatterns := []string{
		"vulnerability found",
		"scan completed",
		"new vulnerability",
		"security issue",
		"cve-",
		"high severity",
		"critical severity",
		"scan result",
	}

	lineLower := strings.ToLower(line)
	for _, pattern := range updatePatterns {
		if strings.Contains(lineLower, pattern) {
			return true
		}
	}

	return false
}

// processVulnerabilityReport processes a VulnerabilityReport ConfigMap
func (tw *TrivyWatcher) processVulnerabilityReport(ctx context.Context, report *corev1.ConfigMap) error {
	logger.Debug("Processing VulnerabilityReport",
		logger.Fields{
			Component:    "watcher",
			Operation:    "process_vulnerability_report",
			Source:       "trivy",
			ResourceKind: "VulnerabilityReport",
			ResourceName: report.Name,
			Namespace:    report.Namespace,
		})

	// Extract vulnerability data from the report
	if report.Data != nil {
		for key, value := range report.Data {
			if strings.Contains(key, "vulnerability") || strings.Contains(key, "report") {
				logger.Debug("Found vulnerability data",
					logger.Fields{
						Component:    "watcher",
						Operation:    "process_vulnerability_report",
						Source:       "trivy",
						ResourceKind: "VulnerabilityReport",
						Additional: map[string]interface{}{
							"data_key": key,
						},
					})

				// Parse the vulnerability data
				if err := tw.parseVulnerabilityData(ctx, value); err != nil {
					logger.Error("Failed to parse vulnerability data",
						logger.Fields{
							Component: "watcher",
							Operation: "parse_vulnerability_data",
							Source:    "trivy",
							Error:     err,
						})
				}
			}
		}
	}

	return nil
}

// parseVulnerabilityData parses vulnerability data and triggers actions
func (tw *TrivyWatcher) parseVulnerabilityData(ctx context.Context, data string) error {
	// Look for CVE patterns in the data
	if strings.Contains(strings.ToUpper(data), "CVE-") {
		// Extract CVEs
		cves := tw.extractCVEs(data)
		for _, cve := range cves {
			logger.Info("Found CVE",
				logger.Fields{
					Component: "watcher",
					Operation: "parse_vulnerability_data",
					Source:    "trivy",
					Severity:  "HIGH",
					Additional: map[string]interface{}{
						"cve": cve,
					},
				})

			// Trigger action for each CVE
			if err := tw.actionHandler.HandleTrivyUpdate(ctx, fmt.Sprintf("CVE detected: %s", cve)); err != nil {
				logger.Error("Failed to handle CVE",
					logger.Fields{
						Component: "watcher",
						Operation: "handle_cve",
						Source:    "trivy",
						Additional: map[string]interface{}{
							"cve": cve,
						},
						Error: err,
					})
			}
		}
	}

	return nil
}

// extractCVEs extracts CVE identifiers from vulnerability data
func (tw *TrivyWatcher) extractCVEs(data string) []string {
	var cves []string
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		if strings.Contains(strings.ToUpper(line), "CVE-") {
			// Extract CVE from line
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(strings.ToUpper(part), "CVE-") {
					cves = append(cves, strings.ToUpper(part))
				}
			}
		}
	}

	return cves
}
