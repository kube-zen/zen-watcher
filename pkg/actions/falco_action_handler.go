package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/models"
)

// FalcoActionHandler handles Falco security events with detailed logging
type FalcoActionHandler struct {
	namespace    string
	recentEvents []models.SecurityEvent // Store recent events for AI service
}

// NewFalcoActionHandler creates a new Falco action handler
func NewFalcoActionHandler(namespace string) *FalcoActionHandler {
	return &FalcoActionHandler{
		namespace: namespace,
	}
}

// HandleFalcoEvent processes Falco security events
func (h *FalcoActionHandler) HandleFalcoEvent(ctx context.Context, event *models.SecurityEvent) error {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	fmt.Printf("================================================================================\n")
	fmt.Printf("🚨 FALCO SECURITY EVENT DETECTED\n")
	fmt.Printf("================================================================================\n")
	fmt.Printf("📅 Timestamp: %s\n", timestamp)
	fmt.Printf("🔍 Rule: %s\n", event.Type)
	fmt.Printf("⚠️  Severity: %s\n", strings.ToUpper(event.Severity))
	fmt.Printf("📍 Source: %s\n", event.Source)
	fmt.Printf("🏷️  Namespace: %s\n", event.Namespace)
	fmt.Printf("📦 Resource: %s\n", event.Resource)
	fmt.Printf("📄 Description: %s\n", event.Description)

	// Analyze the security event
	h.analyzeSecurityEvent(event)

	// Store the event for AI service access
	h.storeFalcoEvent(event)

	fmt.Printf("📊 Status: Processed successfully\n")
	fmt.Printf("================================================================================\n")

	return nil
}

// analyzeSecurityEvent provides analysis of the security event
func (h *FalcoActionHandler) analyzeSecurityEvent(event *models.SecurityEvent) {
	// Check for high-severity events
	sev := strings.ToUpper(event.Severity)
	if strings.Contains(sev, "CRITICAL") || strings.Contains(sev, "HIGH") {
		fmt.Printf("🚨 HIGH SEVERITY SECURITY EVENT DETECTED!\n")
		fmt.Printf("   This requires immediate attention\n")
	}

	// Check for specific security patterns
	messageLower := strings.ToLower(event.Description)

	// Network security events
	if strings.Contains(messageLower, "network") || strings.Contains(messageLower, "connection") {
		fmt.Printf("🌐 Network Security Event: Potential network-based attack detected\n")
	}

	// File system events
	if strings.Contains(messageLower, "file") || strings.Contains(messageLower, "directory") {
		fmt.Printf("📁 File System Event: Suspicious file system activity detected\n")
	}

	// Process events
	if strings.Contains(messageLower, "process") || strings.Contains(messageLower, "exec") {
		fmt.Printf("⚙️  Process Event: Suspicious process activity detected\n")
	}

	// Container events
	if strings.Contains(messageLower, "container") || strings.Contains(messageLower, "docker") {
		fmt.Printf("🐳 Container Event: Container security violation detected\n")
	}

	// Kubernetes API events
	if strings.Contains(messageLower, "k8s") || strings.Contains(messageLower, "kubernetes") {
		fmt.Printf("☸️  Kubernetes Event: K8s API security violation detected\n")
	}
}

// storeFalcoEvent stores a Falco event for AI service access
func (h *FalcoActionHandler) storeFalcoEvent(event *models.SecurityEvent) {
	// Store the event (keep only last 10 events)
	h.recentEvents = append(h.recentEvents, *event)
	if len(h.recentEvents) > 10 {
		h.recentEvents = h.recentEvents[1:]
	}
}

// GetRecentEvents returns recent Falco events for AI service
func (h *FalcoActionHandler) GetRecentEvents() []models.SecurityEvent {
	return h.recentEvents
}
