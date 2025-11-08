package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZenEvent represents a zen event CRD
type ZenEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZenEventSpec   `json:"spec"`
	Status ZenEventStatus `json:"status,omitempty"`
}

// ZenEventSpec defines the desired state of ZenEvent
type ZenEventSpec struct {
	Category          string             `json:"category"`                    // security, compliance, performance, observability, custom, etc (extensible)
	Source            string             `json:"source"`                      // trivy, falco, kyverno, audit, kube-bench, custom tools (extensible)
	EventType         string             `json:"eventType"`                   // vulnerability, runtime-threat, policy-violation, audit-event, benchmark, custom (extensible)
	Message           string             `json:"message"`                     // Event message/description
	Severity          string             `json:"severity"`                    // CRITICAL, HIGH, MEDIUM, LOW, INFO
	AffectedResources []AffectedResource `json:"affectedResources,omitempty"` // Resources affected by this event
	Priority          int                `json:"priority"`                    // 1 (highest) to 10 (lowest)
	Tags              []string           `json:"tags,omitempty"`              // Tags for categorization
	Metadata          map[string]string  `json:"metadata,omitempty"`          // Additional metadata for observability
	Timestamp         string             `json:"timestamp"`                   // When the event was detected (RFC3339)
}

// ZenEventStatus defines the observed state of ZenEvent
type ZenEventStatus struct {
	Phase      string           `json:"phase"`                // Active, Resolved, Acknowledged, Archived
	Conditions []EventCondition `json:"conditions,omitempty"` // Conditions of the event
	FirstSeen  string           `json:"firstSeen,omitempty"`  // First time this event was seen
	LastSeen   string           `json:"lastSeen,omitempty"`   // Last time this event was seen
	Count      int              `json:"count,omitempty"`      // Number of times this event occurred
}

// ZenEventList contains a list of ZenEvent
type ZenEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZenEvent `json:"items"`
}

// AffectedResource represents a Kubernetes resource affected by the event
type AffectedResource struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	APIVersion string `json:"apiVersion"`
}

// EventCondition represents a condition of the ZenEvent
type EventCondition struct {
	Type               string      `json:"type"`
	Status             string      `json:"status"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	Reason             string      `json:"reason,omitempty"`
	Message            string      `json:"message,omitempty"`
}

// Constants for ZenEvent (these are common values, but fields are extensible)
const (
	// Common category types (NOT exhaustive - users can add custom categories)
	CategorySecurity      = "security"
	CategoryCompliance    = "compliance"
	CategoryPerformance   = "performance"
	CategoryObservability = "observability"
	CategoryCustom        = "custom"

	// Common source types (NOT exhaustive - users can add custom sources)
	SourceTrivy     = "trivy"
	SourceFalco     = "falco"
	SourceKyverno   = "kyverno"
	SourceAudit     = "audit"
	SourceKubeBench = "kube-bench"

	// Common event types (NOT exhaustive - users can add custom event types)
	EventTypeVulnerability   = "vulnerability"
	EventTypeRuntimeThreat   = "runtime-threat"
	EventTypePolicyViolation = "policy-violation"
	EventTypeAuditEvent      = "audit-event"
	EventTypeBenchmark       = "benchmark"

	// Severity levels
	SeverityCritical = "CRITICAL"
	SeverityHigh     = "HIGH"
	SeverityMedium   = "MEDIUM"
	SeverityLow      = "LOW"
	SeverityInfo     = "INFO"

	// Phase values
	PhaseActive       = "Active"
	PhaseResolved     = "Resolved"
	PhaseAcknowledged = "Acknowledged"
	PhaseArchived     = "Archived"

	// Condition types
	ConditionAcknowledged = "Acknowledged"
	ConditionResolved     = "Resolved"
	ConditionEscalated    = "Escalated"

	// Condition status
	ConditionTrue    = "True"
	ConditionFalse   = "False"
	ConditionUnknown = "Unknown"
)
