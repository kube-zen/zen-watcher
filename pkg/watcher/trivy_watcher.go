package watcher

import (
	"bufio"
	"context"
	"fmt"
	"strings"

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
	allPods, err := tw.clientSet.CoreV1().Pods(tw.namespace).List(ctx, metav1."ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods in namespace %s: %v", tw.namespace, err)
	}

	if len(allPods.Items) == 0 {
		return fmt.Errorf("no pods found in namespace %s", tw.namespace)
	}

	fmt.Printf("📋 Found %d pods in namespace %s\n", len(allPods.Items), tw.namespace)

	// Look for Trivy-related pods (more flexible matching)
	var trivyPods []corev1."Pod
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
	fmt.Printf("✅ Watching logs from Trivy pod %s in namespace %s\n", pod.Name, tw.namespace)

	return tw.watchPodLogs(ctx, pod.Name)
}

// WatchVulnerabilityReports watches for new VulnerabilityReport resources
func (tw *TrivyWatcher) WatchVulnerabilityReports(ctx context.Context) error {
	fmt.Printf("🔍 Watching VulnerabilityReport resources in namespace %s\n", tw.namespace)

	// List existing VulnerabilityReports using the Trivy operator API
	reports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1."ListOptions{
		LabelSelector: "trivy-operator.resource.kind=VulnerabilityReport",
	})
	if err != nil {
		return fmt.Errorf("failed to list VulnerabilityReports: %v", err)
	}

	fmt.Printf("📊 Found %d existing VulnerabilityReports\n", len(reports.Items))

	// Process existing reports
	for _, report := range reports.Items {
		if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
			fmt.Printf("❌ Failed to process report %s: %v\n", report.Name, err)
		}
	}

	// TODO: Implement watch for new VulnerabilityReports
	// This would require using the Kubernetes informer pattern
	// For now, we'll just process existing reports

	return nil
}

// WatchTrivyResources watches for Trivy security resources
func (tw *TrivyWatcher) WatchTrivyResources(ctx context.Context) error {
	fmt.Printf("🔍 Watching Trivy security resources in namespace %s\n", tw.namespace)

	// List VulnerabilityReports
	fmt.Printf("📊 Checking VulnerabilityReports...\n")
	vulnReports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1."ListOptions{
		LabelSelector: "trivy-operator.resource.kind=VulnerabilityReport",
	})
	if err != nil {
		fmt.Printf("❌ Failed to list VulnerabilityReports: %v\n", err)
	} else {
		fmt.Printf("📊 Found %d VulnerabilityReports\n", len(vulnReports.Items))
		for _, report := range vulnReports.Items {
			if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
				fmt.Printf("❌ Failed to process report %s: %v\n", report.Name, err)
			}
		}
	}

	// List ClusterVulnerabilityReports
	fmt.Printf("📊 Checking ClusterVulnerabilityReports...\n")
	clusterVulnReports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1."ListOptions{
		LabelSelector: "trivy-operator.resource.kind=ClusterVulnerabilityReport",
	})
	if err != nil {
		fmt.Printf("❌ Failed to list ClusterVulnerabilityReports: %v\n", err)
	} else {
		fmt.Printf("📊 Found %d ClusterVulnerabilityReports\n", len(clusterVulnReports.Items))
		for _, report := range clusterVulnReports.Items {
			if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
				fmt.Printf("❌ Failed to process cluster report %s: %v\n", report.Name, err)
			}
		}
	}

	// List ConfigAuditReports
	fmt.Printf("📊 Checking ConfigAuditReports...\n")
	configAuditReports, err := tw.clientSet.CoreV1().ConfigMaps(tw.namespace).List(ctx, metav1."ListOptions{
		LabelSelector: "trivy-operator.resource.kind=ConfigAuditReport",
	})
	if err != nil {
		fmt.Printf("❌ Failed to list ConfigAuditReports: %v\n", err)
	} else {
		fmt.Printf("📊 Found %d ConfigAuditReports\n", len(configAuditReports.Items))
		for _, report := range configAuditReports.Items {
			if err := tw.processVulnerabilityReport(ctx, &report); err != nil {
				fmt.Printf("❌ Failed to process config audit report %s: %v\n", report.Name, err)
			}
		}
	}

	return nil
}

// isTrivyPod checks if a pod is Trivy-related
func (tw *TrivyWatcher) isTrivyPod(pod corev1."Pod) bool {
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
	req := tw.clientSet.CoreV1().Pods(tw.namespace).GetLogs(podName, &corev1."PodLogOptions{
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
		fmt.Println("Trivy Log:", line)

		// Check for Trivy-specific patterns
		if tw.isTrivyUpdate(line) {
			fmt.Println(">>> Detected Trivy update! Triggering action...")
			if err := tw.actionHandler.HandleTrivyUpdate(ctx, line); err != nil {
				fmt.Printf("❌ Failed to handle Trivy update: %v\n", err)
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
func (tw *TrivyWatcher) processVulnerabilityReport(ctx context.Context, report *corev1."ConfigMap) error {
	fmt.Printf("🔍 Processing VulnerabilityReport: %s\n", report.Name)

	// Extract vulnerability data from the report
	if report.Data != nil {
		for key, value := range report.Data {
			if strings.Contains(key, "vulnerability") || strings.Contains(key, "report") {
				fmt.Printf("📄 Found vulnerability data in key: %s\n", key)

				// Parse the vulnerability data
				if err := tw.parseVulnerabilityData(ctx, value); err != nil {
					fmt.Printf("❌ Failed to parse vulnerability data: %v\n", err)
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
			fmt.Printf("🚨 Found CVE: %s\n", cve)

			// Trigger action for each CVE
			if err := tw.actionHandler.HandleTrivyUpdate(ctx, fmt.Sprintf("CVE detected: %s", cve)); err != nil {
				fmt.Printf("❌ Failed to handle CVE %s: %v\n", cve, err)
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
