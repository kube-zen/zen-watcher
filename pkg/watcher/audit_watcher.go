package watcher

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// AuditWatcher watches Kubernetes audit logs for security events
type AuditWatcher struct {
	clientSet     *kubernetes.Clientset
	namespace     string
	actionHandler AuditActionHandler
}

// AuditActionHandler interface for handling audit events
type AuditActionHandler interface {
	HandleAuditEvent(ctx context.Context, event *AuditSecurityEvent) error
}

// AuditSecurityEvent represents a security-relevant audit event
type AuditSecurityEvent struct {
	Timestamp     string            `json:"timestamp"`
	Level         string            `json:"level"`
	Verb          string            `json:"verb"`
	Resource      string            `json:"resource"`
	Namespace     string            `json:"namespace"`
	User          string            `json:"user"`
	UserAgent     string            `json:"userAgent"`
	IP            string            `json:"ip"`
	ResponseCode  int               `json:"responseCode"`
	Message       string            `json:"message"`
	SecurityLevel string            `json:"securityLevel"`
	RawEvent      string            `json:"rawEvent"`
	Metadata      map[string]string `json:"metadata"`
}

// NewAuditWatcher creates a new audit watcher
func NewAuditWatcher(clientSet *kubernetes.Clientset, namespace string, actionHandler AuditActionHandler) *AuditWatcher {
	return &AuditWatcher{
		clientSet:     clientSet,
		namespace:     namespace,
		actionHandler: actionHandler,
	}
}

// WatchAuditLogs watches Kubernetes audit logs for security events
func (aw *AuditWatcher) WatchAuditLogs(ctx context.Context) error {
	fmt.Printf("🔍 Watching Kubernetes audit logs in namespace %s\n", aw.namespace)

	// For now, we'll watch the API server logs since audit logs might not be available
	// In a production environment, you'd typically watch audit log files or use audit webhooks
	return aw.watchAPIServerLogs(ctx)
}

// watchAPIServerLogs watches API server logs for security-relevant events
func (aw *AuditWatcher) watchAPIServerLogs(ctx context.Context) error {
	// Get API server pods
	pods, err := aw.clientSet.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "component=kube-apiserver",
	})
	if err != nil {
		return fmt.Errorf("failed to list API server pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no API server pods found")
	}

	// Watch logs from the first API server pod
	pod := pods.Items[0]
	fmt.Printf("✅ Watching API server logs from pod %s\n", pod.Name)

	return aw.watchPodLogs(ctx, pod.Name, "kube-system")
}

// watchPodLogs watches logs from a specific pod
func (aw *AuditWatcher) watchPodLogs(ctx context.Context, podName, namespace string) error {
	req := aw.clientSet.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
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
		fmt.Println("Audit Log:", line)

		// Check for security-relevant audit events
		if aw.isSecurityRelevantEvent(line) {
			fmt.Println(">>> Detected security-relevant audit event! Processing...")
			event := aw.parseAuditEvent(line)
			if event != nil {
				if err := aw.actionHandler.HandleAuditEvent(ctx, event); err != nil {
					fmt.Printf("❌ Failed to handle audit event: %v\n", err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}

// isSecurityRelevantEvent checks if a log line contains security-relevant information
func (aw *AuditWatcher) isSecurityRelevantEvent(line string) bool {
	// Look for security-relevant patterns in API server logs
	securityPatterns := []string{
		"authentication",
		"authorization",
		"forbidden",
		"unauthorized",
		"token",
		"secret",
		"configmap",
		"rbac",
		"role",
		"binding",
		"pod",
		"namespace",
		"create",
		"delete",
		"update",
		"patch",
		"impersonate",
		"escalate",
		"bind",
		"escalate",
		"impersonate",
		"serviceaccount",
		"user",
		"group",
	}

	lineLower := strings.ToLower(line)
	for _, pattern := range securityPatterns {
		if strings.Contains(lineLower, pattern) {
			return true
		}
	}

	return false
}

// parseAuditEvent parses a security event from a log line
func (aw *AuditWatcher) parseAuditEvent(line string) *AuditSecurityEvent {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Extract basic information
	verb := aw.extractField(line, "verb=")
	resource := aw.extractField(line, "resource=")
	namespace := aw.extractField(line, "namespace=")
	user := aw.extractField(line, "user=")
	userAgent := aw.extractField(line, "userAgent=")
	ip := aw.extractField(line, "ip=")
	responseCode := aw.extractField(line, "responseCode=")

	// Determine security level
	securityLevel := aw.determineSecurityLevel(line, verb, resource)

	return &AuditSecurityEvent{
		Timestamp:     timestamp,
		Level:         "Request",
		Verb:          verb,
		Resource:      resource,
		Namespace:     namespace,
		User:          user,
		UserAgent:     userAgent,
		IP:            ip,
		ResponseCode:  aw.parseResponseCode(responseCode),
		Message:       line,
		SecurityLevel: securityLevel,
		RawEvent:      line,
		Metadata: map[string]string{
			"source":  "kubernetes-audit",
			"watcher": "audit-watcher",
		},
	}
}

// determineSecurityLevel determines the security level of an event
func (aw *AuditWatcher) determineSecurityLevel(line, verb, resource string) string {
	lineLower := strings.ToLower(line)

	// High security events
	if strings.Contains(lineLower, "forbidden") ||
		strings.Contains(lineLower, "unauthorized") ||
		strings.Contains(lineLower, "impersonate") ||
		strings.Contains(lineLower, "escalate") {
		return "HIGH"
	}

	// Medium security events
	if strings.Contains(lineLower, "secret") ||
		strings.Contains(lineLower, "configmap") ||
		strings.Contains(lineLower, "rbac") ||
		strings.Contains(lineLower, "role") {
		return "MEDIUM"
	}

	// Low security events
	if strings.Contains(lineLower, "get") ||
		strings.Contains(lineLower, "list") ||
		strings.Contains(lineLower, "watch") {
		return "LOW"
	}

	return "INFO"
}

// extractField extracts a field value from a log line
func (aw *AuditWatcher) extractField(line, field string) string {
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

// parseResponseCode parses response code from string
func (aw *AuditWatcher) parseResponseCode(codeStr string) int {
	if codeStr == "" {
		return 0
	}

	// Simple parsing - in real implementation you'd use strconv.Atoi
	if strings.Contains(codeStr, "200") {
		return 200
	}
	if strings.Contains(codeStr, "403") {
		return 403
	}
	if strings.Contains(codeStr, "401") {
		return 401
	}

	return 0
}

// WatchAuditSecurityEvents is the main entry point for watching audit events
func (aw *AuditWatcher) WatchAuditSecurityEvents(ctx context.Context) error {
	fmt.Printf("🔍 Starting Kubernetes audit security monitoring\n")
	return aw.WatchAuditLogs(ctx)
}
