package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// TrivyActionHandler handles Trivy CRD events with detailed logging
type TrivyActionHandler struct {
	clientSet    *kubernetes.Clientset
	namespace    string
	recentEvents []models.SecurityEvent // Store recent events for AI service
}

// NewTrivyActionHandler creates a new Trivy action handler
func NewTrivyActionHandler(clientSet *kubernetes.Clientset, namespace string) *TrivyActionHandler {
	return &TrivyActionHandler{
		clientSet: clientSet,
		namespace: namespace,
	}
}

// HandleTrivyConfigMap processes Trivy ConfigMap events
func (h *TrivyActionHandler) HandleTrivyConfigMap(ctx context.Context, configMap *corev1."ConfigMap) error {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	fmt.Printf("================================================================================\n")
	fmt.Printf("🔍 TRIVY REPORT DETECTED\n")
	fmt.Printf("================================================================================\n")
	fmt.Printf("📅 Timestamp: %s\n", timestamp)
	fmt.Printf("📄 ConfigMap Name: %s\n", configMap.Name)
	fmt.Printf("🏷️  Namespace: %s\n", configMap.Namespace)
	fmt.Printf("🎯 Resource: %s/%s\n", configMap.Labels["trivy-operator.resource.kind"], configMap.Labels["trivy-operator.resource.name"])

	// Extract vulnerability data from ConfigMap data
	vulnerabilityCount := 0
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0

	// Process all data in the ConfigMap
	for key, value := range configMap.Data {
		if strings.Contains(strings.ToLower(key), "vulnerability") || strings.Contains(strings.ToLower(key), "report") {
			fmt.Printf("📄 Found report data in key: %s\n", key)

			// Count vulnerabilities by severity
			lines := strings.Split(value, "\n")
			for _, line := range lines {
				lineLower := strings.ToLower(line)
				if strings.Contains(lineLower, "cve-") {
					vulnerabilityCount++

					// Check severity
					if strings.Contains(lineLower, "critical") {
						criticalCount++
					} else if strings.Contains(lineLower, "high") {
						highCount++
					} else if strings.Contains(lineLower, "medium") {
						mediumCount++
					} else if strings.Contains(lineLower, "low") {
						lowCount++
					}
				}
			}
		}
	}

	fmt.Printf("📊 Total Vulnerabilities: %d\n", vulnerabilityCount)
	fmt.Printf("📈 Severity Breakdown:\n")
	if criticalCount > 0 {
		fmt.Printf("   CRITICAL: %d\n", criticalCount)
	}
	if highCount > 0 {
		fmt.Printf("   HIGH: %d\n", highCount)
	}
	if mediumCount > 0 {
		fmt.Printf("   MEDIUM: %d\n", mediumCount)
	}
	if lowCount > 0 {
		fmt.Printf("   LOW: %d\n", lowCount)
	}

	// Show critical and high severity vulnerabilities
	if criticalCount > 0 || highCount > 0 {
		fmt.Printf("🚨 Critical/High Vulnerabilities Found:\n")
		for key, value := range configMap.Data {
			if strings.Contains(strings.ToLower(key), "vulnerability") || strings.Contains(strings.ToLower(key), "report") {
				lines := strings.Split(value, "\n")
				for _, line := range lines {
					lineLower := strings.ToLower(line)
					if strings.Contains(lineLower, "cve-") && (strings.Contains(lineLower, "critical") || strings.Contains(lineLower, "high")) {
						fmt.Printf("   - %s\n", line)
					}
				}
			}
		}
	}

	fmt.Printf("📊 Status: Processed successfully\n")
	fmt.Printf("================================================================================\n")

	// Store the event for AI service access
	h.storeTrivyEvent(configMap)

	return nil
}

// storeTrivyEvent stores a Trivy event for AI service access
func (h *TrivyActionHandler) storeTrivyEvent(configMap *corev1."ConfigMap) {
	// Extract vulnerability data and create a structured event
	resName := configMap.Labels["trivy-operator.resource.name"]
	resKind := configMap.Labels["trivy-operator.resource.kind"]
	resource := resName
	if resKind != "" && resName != "" {
		resource = strings.ToLower(resKind) + "/" + resName
	}

	event := models.SecurityEvent{
		ID:          fmt.Sprintf("trivy-%d", time.Now().UnixNano()),
		Source:      "trivy",
		Type:        "vulnerability",
		Timestamp:   time.Now().UTC(),
		Severity:    "medium", // will be updated based on analysis
		Namespace:   configMap.Namespace,
		Resource:    resource,
		Description: "Trivy scan found security vulnerabilities",
		Details: map[string]interface{}{
			"trivy.kind": resKind,
			"trivy.name": resName,
		},
	}

	// Analyze severity from ConfigMap data
	for key, value := range configMap.Data {
		if strings.Contains(strings.ToLower(key), "vulnerability") || strings.Contains(strings.ToLower(key), "report") {
			lines := strings.Split(value, "\n")
			for _, line := range lines {
				lineLower := strings.ToLower(line)
				if strings.Contains(lineLower, "critical") {
					event.Severity = "critical"
					break
				} else if strings.Contains(lineLower, "high") {
					event.Severity = "high"
				} else if strings.Contains(lineLower, "medium") && event.Severity != "high" {
					event.Severity = "medium"
				} else if strings.Contains(lineLower, "low") && event.Severity == "high" {
					event.Severity = "low"
				}
			}
		}
	}

	// Store the event (keep only last 10 events)
	h.recentEvents = append(h.recentEvents, event)
	if len(h.recentEvents) > 10 {
		h.recentEvents = h.recentEvents[1:]
	}
}

// GetRecentEvents returns recent Trivy events for AI service
func (h *TrivyActionHandler) GetRecentEvents() []models.SecurityEvent {
	return h.recentEvents
}
