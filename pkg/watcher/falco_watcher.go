package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FalcoWatcher watches for Falco security events and triggers actions
type FalcoWatcher struct {
	clientSet     *kubernetes.Clientset
	namespace     string
	actionHandler FalcoActionHandler
}

// FalcoActionHandler interface for handling Falco security events
type FalcoActionHandler interface {
	HandleFalcoEvent(ctx context.Context, event *models.SecurityEvent) error
}

// NewFalcoWatcher creates a new Falco watcher
func NewFalcoWatcher(clientSet *kubernetes.Clientset, namespace string, actionHandler FalcoActionHandler) *FalcoWatcher {
	return &FalcoWatcher{
		clientSet:     clientSet,
		namespace:     namespace,
		actionHandler: actionHandler,
	}
}

// WatchFalcoPods watches for Falco pods and their security events
func (fw *FalcoWatcher) WatchFalcoPods(ctx context.Context) error {
	// List all pods in the Falco namespace first
	allPods, err := fw.clientSet.CoreV1().Pods(fw.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods in namespace %s: %v", fw.namespace, err)
	}

	if len(allPods.Items) == 0 {
		return fmt.Errorf("no pods found in namespace %s", fw.namespace)
	}

	fmt.Printf("📋 Found %d pods in namespace %s\n", len(allPods.Items), fw.namespace)

	// Look for Falco-related pods
	var falcoPods []corev1.Pod
	for _, pod := range allPods.Items {
		if fw.isFalcoPod(pod) {
			falcoPods = append(falcoPods, pod)
		}
	}

	if len(falcoPods) == 0 {
		return fmt.Errorf("no Falco-related pods found in namespace %s", fw.namespace)
	}

	// Watch logs from the first Falco pod
	pod := falcoPods[0]
	fmt.Printf("✅ Watching Falco logs from pod %s in namespace %s\n", pod.Name, fw.namespace)

	return fw.watchFalcoLogs(ctx, pod.Name)
}

// isFalcoPod checks if a pod is Falco-related
func (fw *FalcoWatcher) isFalcoPod(pod corev1.Pod) bool {
	// Check for Falco labels
	for key, value := range pod.Labels {
		keyLower := strings.ToLower(key)
		valueLower := strings.ToLower(value)

		if strings.Contains(keyLower, "falco") || strings.Contains(valueLower, "falco") {
			return true
		}
	}

	return false
}

// watchFalcoLogs watches logs from a specific Falco pod
func (fw *FalcoWatcher) watchFalcoLogs(ctx context.Context, podName string) error {
	// Stream logs from the Falco container
	req := fw.clientSet.CoreV1().Pods(fw.namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "falco",
		Follow:    true,
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
		fmt.Println("Falco Log:", line)

		// Check for Falco security events
		if fw.isFalcoSecurityEvent(line) {
			fmt.Println(">>> Detected Falco security event! Processing...")
			event := fw.parseFalcoEvent(line, podName)
			if event != nil {
				if err := fw.actionHandler.HandleFalcoEvent(ctx, event); err != nil {
					fmt.Printf("❌ Failed to handle Falco event: %v\n", err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}

// isFalcoSecurityEvent checks if a log line contains a Falco security event
func (fw *FalcoWatcher) isFalcoSecurityEvent(line string) bool {
	// Look for Falco security event patterns
	securityPatterns := []string{
		"Notice:",
		"Warning:",
		"Critical:",
		"Error:",
		"falco:",
		"Rule:",
		"Priority:",
		"Source:",
		"Time:",
		"User:",
		"Container:",
		"Process:",
	}

	lineLower := strings.ToLower(line)
	for _, pattern := range securityPatterns {
		if strings.Contains(lineLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// parseFalcoEvent parses a Falco security event from a log line
func (fw *FalcoWatcher) parseFalcoEvent(line string, podName string) *models.SecurityEvent {
	// Try JSON mapping first
	if ev, ok := fw.tryParseFalcoJSON(line); ok {
		return ev
	}

	// Extract rule name
	rule := fw.extractField(line, "Rule:")
	if rule == "" {
		rule = "Unknown"
	}

	// Extract priority
	priority := fw.extractField(line, "Priority:")
	if priority == "" {
		priority = "Unknown"
	}

	// Extract source
	source := fw.extractField(line, "Source:")
	if source == "" {
		source = "Falco"
	}

	// Extract user
	user := fw.extractField(line, "User:")
	if user == "" {
		user = "Unknown"
	}

	// Extract container
	container := fw.extractField(line, "Container:")
	if container == "" {
		container = "Unknown"
	}

	// Extract process
	process := fw.extractField(line, "Process:")
	if process == "" {
		process = "Unknown"
	}

	details := map[string]interface{}{
		"rule":      rule,
		"process":   process,
		"container": container,
		"user":      user,
		"raw":       line,
	}
	resource := ""
	if podName != "" && podName != "Unknown" {
		resource = "pod/" + podName
	}

	return &models.SecurityEvent{
		ID:          fmt.Sprintf("falco-%d", time.Now().UnixNano()),
		Source:      "falco",
		Type:        rule,
		Timestamp:   time.Now().UTC(),
		Severity:    normalizeFalcoSeverity(priority),
		Namespace:   fw.namespace,
		Resource:    resource,
		Description: line,
		Details:     details,
	}
}

// Minimal Falco JSON event shape
type falcoEvent struct {
	Rule     string                 `json:"rule"`
	Priority string                 `json:"priority"`
	Output   string                 `json:"output"`
	OutputTS *time.Time             `json:"output_ts,omitempty"`
	Time     *time.Time             `json:"time,omitempty"`
	Fields   map[string]interface{} `json:"output_fields"`
}

func (fw *FalcoWatcher) tryParseFalcoJSON(s string) (*models.SecurityEvent, bool) {
	var fe falcoEvent
	if err := json.Unmarshal([]byte(s), &fe); err != nil {
		return nil, false
	}
	if fe.Fields == nil {
		fe.Fields = map[string]interface{}{}
	}

	ns, _ := fe.Fields["k8s.ns.name"].(string)
	pod, _ := fe.Fields["k8s.pod.name"].(string)
	dep, _ := fe.Fields["k8s.deployment.name"].(string)

	resource := ""
	switch {
	case pod != "":
		resource = "pod/" + pod
	case dep != "":
		resource = "deployment/" + dep
	}

	ts := time.Now().UTC()
	if fe.Time != nil && !fe.Time.IsZero() {
		ts = fe.Time.UTC()
	} else if fe.OutputTS != nil && !fe.OutputTS.IsZero() {
		ts = fe.OutputTS.UTC()
	}

	// Redact a couple of risky fields if present
	delete(fe.Fields, "proc.cmdline")
	delete(fe.Fields, "user.password")

	ev := &models.SecurityEvent{
		ID:          fmt.Sprintf("falco-%d", time.Now().UnixNano()),
		Source:      "falco",
		Type:        fe.Rule,
		Severity:    normalizeFalcoSeverity(fe.Priority),
		Namespace:   firstNonEmpty(ns, fw.namespace),
		Resource:    resource,
		Description: fe.Output,
		Details:     fe.Fields,
		Timestamp:   ts,
	}
	return ev, true
}

func normalizeFalcoSeverity(p string) string {
	switch strings.ToLower(p) {
	case "emergency", "critical":
		return "critical"
	case "alert", "error":
		return "high"
	case "warning":
		return "medium"
	case "notice", "informational":
		return "low"
	default:
		return "info"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// extractField extracts a field value from a Falco log line
func (fw *FalcoWatcher) extractField(line, field string) string {
	start := strings.Index(line, field)
	if start == -1 {
		return ""
	}

	start += len(field)
	end := strings.Index(line[start:], " ")
	if end == -1 {
		end = len(line) - start
	}

	return strings.TrimSpace(line[start : start+end])
}

// WatchFalcoSecurityEvents watches for Falco security events
func (fw *FalcoWatcher) WatchFalcoSecurityEvents(ctx context.Context) error {
	fmt.Printf("🔍 Watching Falco security events in namespace %s\n", fw.namespace)

	// Start monitoring Falco pods
	return fw.WatchFalcoPods(ctx)
}
