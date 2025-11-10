package writer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/types"
	"github.com/kube-zen/zen-watcher/pkg/actions"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CRDWriter writes events as ZenEvent CRDs
type CRDWriter struct {
	zenClient *types.ZenClient
	namespace string
}

// NewCRDWriter creates a new CRD writer
	return &CRDWriter{
		zenClient: zenClient,
		namespace: namespace,
	}
}

// WriteSecurityEvents writes collected events as CRDs
func (cw *CRDWriter) WriteSecurityEvents(ctx context.Context, trivyHandler *actions.TrivyActionHandler, falcoHandler *actions.FalcoActionHandler, auditHandler *actions.AuditActionHandler, kyvernoHandler *actions.KyvernoActionHandler) error {
	log.Printf("📝 [WATCHER] Writing events to CRDs...")

	totalWritten := 0

	// Write Trivy events (security/vulnerability)
	trivyEvents := trivyHandler.GetRecentEvents()
	for _, event := range trivyEvents {
		metrics.RecordEvent(types.CategorySecurity, types.SourceTrivy, types.EventTypeVulnerability, event.Severity)
		if err := cw.writeSecurityEvent(ctx, event, types.CategorySecurity, types.SourceTrivy, types.EventTypeVulnerability); err != nil {
			log.Printf("❌ Failed to write Trivy event: %v", err)
			metrics.RecordEventFailure(types.CategorySecurity, types.SourceTrivy, "write_error")
		} else {
			totalWritten++
			metrics.RecordEventWritten(types.CategorySecurity, types.SourceTrivy)
		}
	}

	// Write Falco events (security/runtime-threat)
	falcoEvents := falcoHandler.GetRecentEvents()
	for _, event := range falcoEvents {
		metrics.RecordEvent(types.CategorySecurity, types.SourceFalco, types.EventTypeRuntimeThreat, event.Severity)
		if err := cw.writeSecurityEvent(ctx, event, types.CategorySecurity, types.SourceFalco, types.EventTypeRuntimeThreat); err != nil {
			log.Printf("❌ Failed to write Falco event: %v", err)
			metrics.RecordEventFailure(types.CategorySecurity, types.SourceFalco, "write_error")
		} else {
			totalWritten++
			metrics.RecordEventWritten(types.CategorySecurity, types.SourceFalco)
		}
	}

	// Write Audit events (compliance/audit-event)
	auditEvents := auditHandler.GetRecentEvents()
	for _, event := range auditEvents {
		metrics.RecordEvent(types.CategoryCompliance, types.SourceAudit, types.EventTypeAuditEvent, event.Severity)
		if err := cw.writeSecurityEvent(ctx, event, types.CategoryCompliance, types.SourceAudit, types.EventTypeAuditEvent); err != nil {
			log.Printf("❌ Failed to write Audit event: %v", err)
			metrics.RecordEventFailure(types.CategoryCompliance, types.SourceAudit, "write_error")
		} else {
			totalWritten++
			metrics.RecordEventWritten(types.CategoryCompliance, types.SourceAudit)
		}
	}

	// Write Kyverno events (security/policy-violation)
	kyvernoEvents := kyvernoHandler.GetRecentEvents()
	for _, v := range kyvernoEvents {
		// Convert Kyverno event to SecurityEvent format
		resource := v.ResourceName
		if v.ResourceKind != "" && v.ResourceName != "" {
			resource = strings.ToLower(v.ResourceKind) + "/" + v.ResourceName
		}
		event := models.SecurityEvent{
			ID:          fmt.Sprintf("kyverno-%d", time.Now().UnixNano()),
			Source:      "kyverno",
			Type:        "policy_violation",
			Severity:    cw.mapKyvernoViolationTypeToSeverity(v.ViolationType),
			Namespace:   v.Namespace,
			Resource:    resource,
			Description: v.Message,
			Details: map[string]interface{}{
				"policyName":    v.PolicyName,
				"policyType":    v.PolicyType,
				"resourceKind":  v.ResourceKind,
				"violationType": v.ViolationType,
				"rule":          v.Details["rule"],
				"result":        v.Details["result"],
			},
			Timestamp: time.Now().UTC(),
		}
		metrics.RecordEvent(types.CategorySecurity, types.SourceKyverno, types.EventTypePolicyViolation, event.Severity)
		if err := cw.writeSecurityEvent(ctx, event, types.CategorySecurity, types.SourceKyverno, types.EventTypePolicyViolation); err != nil {
			log.Printf("❌ Failed to write Kyverno event: %v", err)
			metrics.RecordEventFailure(types.CategorySecurity, types.SourceKyverno, "write_error")
		} else {
			totalWritten++
			metrics.RecordEventWritten(types.CategorySecurity, types.SourceKyverno)
		}
	}

	if totalWritten == 0 {
		log.Printf("📝 [WATCHER] No new events to write")
		return nil
	}

	log.Printf("✅ [WATCHER] Successfully wrote %d events as CRDs", totalWritten)
	return nil
}

// writeSecurityEvent writes a single event as a ZenEvent CRD
func (cw *CRDWriter) writeSecurityEvent(ctx context.Context, event models.SecurityEvent, category, source, eventType string) error {
	// Track CRD operation duration
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.CRDOperationDuration.WithLabelValues("create").Observe(duration)
	}()

	// Generate a unique name for the CRD
	name := fmt.Sprintf("%s-%s-%d", source, event.Type, time.Now().Unix())
	name = strings.ToLower(strings.ReplaceAll(name, "_", "-"))

	// Limit name length to 63 characters (Kubernetes limit)
	if len(name) > 63 {
		name = name[:63]
	}

	// Build affected resources
	affectedResources := []types.AffectedResource{}
	if event.Resource != "" {
		parts := strings.Split(event.Resource, "/")
		resource := types.AffectedResource{
			Namespace: event.Namespace,
		}
		if len(parts) == 2 {
			resource.Kind = parts[0]
			resource.Name = parts[1]
		} else {
			resource.Name = event.Resource
		}
		affectedResources = append(affectedResources, resource)
	}

	// Build metadata for observability
	metadata := make(map[string]string)
	metadata["event_id"] = event.ID
	metadata["source"] = source
	metadata["category"] = category
	metadata["event_type"] = eventType
	if event.Namespace != "" {
		metadata["namespace"] = event.Namespace
	}

	// Add custom details as metadata (convert to strings)
	for key, value := range event.Details {
		if strVal, ok := value.(string); ok {
			metadata[fmt.Sprintf("detail_%s", key)] = strVal
		}
	}

	// Build tags
	tags := []string{category, source, eventType}
	if event.Severity != "" {
		tags = append(tags, strings.ToLower(event.Severity))
	}

	// Calculate priority based on severity
	priority := cw.calculatePriority(event.Severity)

	// Create the ZenEvent CRD
	zenEvent := &types.ZenEvent{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "zen.kube-zen.com/v1",
			Kind:       "ZenEvent",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cw.namespace,
			Labels: map[string]string{
				"category":   category,
				"source":     source,
				"event-type": eventType,
				"severity":   strings.ToLower(event.Severity),
			},
			Annotations: map[string]string{
				"zen.kube-zen.com/event-id":   event.ID,
			},
		},
		Spec: types.ZenEventSpec{
			Category:          category,
			Source:            source,
			EventType:         eventType,
			Message:           event.Description,
			Severity:          strings.ToUpper(event.Severity),
			AffectedResources: affectedResources,
			Priority:          priority,
			Tags:              tags,
			Metadata:          metadata,
			Timestamp:         event.Timestamp.Format(time.RFC3339),
		},
		Status: types.ZenEventStatus{
			Phase: types.PhaseActive,
		},
	}

	// Create the CRD in the cluster
	_, err := cw.zenClient.CreateZenEvent(ctx, zenEvent)
	if err != nil {
		metrics.CRDOperations.WithLabelValues("create", "failure").Inc()
		return fmt.Errorf("failed to create ZenEvent CRD: %v", err)
	}

	metrics.CRDOperations.WithLabelValues("create", "success").Inc()
	log.Printf("📝 Created %s event CRD: %s/%s (severity: %s)", category, cw.namespace, name, event.Severity)
	return nil
}

// calculatePriority converts severity to priority (1=highest, 10=lowest)
func (cw *CRDWriter) calculatePriority(severity string) int {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return 1
	case "HIGH":
		return 3
	case "MEDIUM":
		return 5
	case "LOW":
		return 7
	default:
		return 8
	}
}

// mapKyvernoViolationTypeToSeverity maps Kyverno violation types to severity levels
func (cw *CRDWriter) mapKyvernoViolationTypeToSeverity(violationType string) string {
	switch violationType {
	case "blocked":
		return "high"
	case "warning":
		return "medium"
	case "passed":
		return "low"
	default:
		return "medium"
	}
}
