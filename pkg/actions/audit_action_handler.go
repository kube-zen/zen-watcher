package actions

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/models"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
)

// AuditActionHandler handles Kubernetes audit security events
type AuditActionHandler struct {
	name         string
	recentEvents []models.SecurityEvent // Store recent events for AI service
}

// NewAuditActionHandler creates a new audit action handler
func NewAuditActionHandler(name string) *AuditActionHandler {
	return &AuditActionHandler{
		name: name,
	}
}

// HandleAuditEvent processes Kubernetes audit security events
func (h *AuditActionHandler) HandleAuditEvent(ctx context.Context, event *watcher.AuditSecurityEvent) error {
	log.Printf("🔍 [AUDIT] Security Event Detected:")
	log.Printf("   📅 Timestamp: %s", event.Timestamp)
	log.Printf("   🎯 Security Level: %s", event.SecurityLevel)
	log.Printf("   👤 User: %s", event.User)
	log.Printf("   🔧 Verb: %s", event.Verb)
	log.Printf("   📦 Resource: %s", event.Resource)
	log.Printf("   🏷️  Namespace: %s", event.Namespace)
	log.Printf("   🌐 IP: %s", event.IP)
	log.Printf("   📱 User Agent: %s", event.UserAgent)
	log.Printf("   📊 Response Code: %d", event.ResponseCode)
	log.Printf("   📝 Message: %s", event.Message)

	// Analyze the security event
	h.analyzeSecurityEvent(event)

	// Store the event for AI service access
	h.storeAuditEvent(event)

	return nil
}

// analyzeSecurityEvent provides additional analysis of the security event
func (h *AuditActionHandler) analyzeSecurityEvent(event *watcher.AuditSecurityEvent) {
	analysis := h.getSecurityAnalysis(event)

	if analysis != "" {
		log.Printf("   🔍 Analysis: %s", analysis)
	}

	// Log security recommendations
	recommendations := h.getZenRecommendations(event)
	if len(recommendations) > 0 {
		log.Printf("   💡 Security Recommendations:")
		for i, rec := range recommendations {
			log.Printf("      %d. %s", i+1, rec)
		}
	}
}

// getSecurityAnalysis provides security analysis of the event
func (h *AuditActionHandler) getSecurityAnalysis(event *watcher.AuditSecurityEvent) string {
	message := strings.ToLower(event.Message)

	// Authentication failures
	if strings.Contains(message, "forbidden") || strings.Contains(message, "unauthorized") {
		return "🚨 Authentication/Authorization failure detected - potential unauthorized access attempt"
	}

	// Privilege escalation attempts
	if strings.Contains(message, "escalate") || strings.Contains(message, "impersonate") {
		return "⚠️  Privilege escalation attempt detected - monitor for potential privilege abuse"
	}

	// Secret access
	if strings.Contains(message, "secret") {
		return "🔐 Secret access detected - verify if this is authorized access"
	}

	// RBAC changes
	if strings.Contains(message, "rbac") || strings.Contains(message, "role") || strings.Contains(message, "binding") {
		return "🛡️  RBAC configuration change detected - verify authorization changes are legitimate"
	}

	// Pod operations
	if strings.Contains(message, "pod") && (event.Verb == "create" || event.Verb == "delete") {
		return "📦 Pod lifecycle operation detected - monitor for unauthorized pod management"
	}

	// Namespace operations
	if strings.Contains(message, "namespace") && (event.Verb == "create" || event.Verb == "delete") {
		return "🏷️  Namespace operation detected - verify namespace management is authorized"
	}

	return ""
}

// getZenRecommendations provides security recommendations based on the event
func (h *AuditActionHandler) getZenRecommendations(event *watcher.AuditSecurityEvent) []string {
	var recommendations []string

	message := strings.ToLower(event.Message)

	// High security level events
	if event.SecurityLevel == "HIGH" {
		recommendations = append(recommendations, "🚨 HIGH PRIORITY: Investigate this event immediately")
		recommendations = append(recommendations, "🔍 Review user permissions and access patterns")
		recommendations = append(recommendations, "📊 Check for similar events from the same user/IP")
	}

	// Authentication failures
	if strings.Contains(message, "forbidden") || strings.Contains(message, "unauthorized") {
		recommendations = append(recommendations, "🔐 Verify user credentials and permissions")
	}

	// Privilege escalation attempts
	if strings.Contains(message, "escalate") || strings.Contains(message, "impersonate") {
		recommendations = append(recommendations, "🛡️  Review RBAC policies and role bindings")
	}

	// Secret access
	if strings.Contains(message, "secret") {
		recommendations = append(recommendations, "🔐 Audit secret access and rotate credentials if necessary")
	}

	return recommendations
}

// GetLastActivity returns the last activity timestamp
func (h *AuditActionHandler) GetLastActivity() time.Time {
	return time.Now()
}

// storeAuditEvent stores an audit event for AI service access
func (h *AuditActionHandler) storeAuditEvent(event *watcher.AuditSecurityEvent) {
	// Convert to models.SecurityEvent
	details := map[string]interface{}{
		"verb":        event.Verb,
		"user":        event.User,
		"ip":          event.IP,
		"user_agent":  event.UserAgent,
		"api_version": "v1",
		"resource_id": event.Resource,
	}
	auditEvent := models.SecurityEvent{
		ID:          fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Source:      "audit",
		Type:        "audit",
		Timestamp:   time.Now().UTC(),
		Severity:    "medium",
		Namespace:   event.Namespace,
		Resource:    event.Resource,
		Description: event.Message,
		Details:     details,
	}

	// Store the event (keep only last 10 events)
	h.recentEvents = append(h.recentEvents, auditEvent)
	if len(h.recentEvents) > 10 {
		h.recentEvents = h.recentEvents[1:]
	}
}

// GetRecentEvents returns recent audit events for AI service
func (h *AuditActionHandler) GetRecentEvents() []models.SecurityEvent {
	return h.recentEvents
}
