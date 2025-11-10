package models

import "time"

// SecurityEvent represents a single security event from any source
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Source      string                 `json:"source"` // "trivy", "falco", "audit"
	Type        string                 `json:"type"`   // "vulnerability", "runtime", "audit"
	Timestamp   time.Time              `json:"timestamp"`
	Severity    string                 `json:"severity"` // "critical", "high", "medium", "low"
	Namespace   string                 `json:"namespace"`
	Resource    string                 `json:"resource"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details"`
}

// SecurityContext represents the overall security context
type SecurityContext struct {
	ClusterID      string    `json:"clusterId"`
	Timestamp      time.Time `json:"timestamp"`
	TotalEvents    int       `json:"totalEvents"`
	HighPriority   int       `json:"highPriority"`
	MediumPriority int       `json:"mediumPriority"`
	LowPriority    int       `json:"lowPriority"`
	CriticalVulns  int       `json:"criticalVulns"`
	HighVulns      int       `json:"highVulns"`
	FailedAuth     int       `json:"failedAuth"`
	PrivilegeEsc   int       `json:"privilegeEsc"`
	SecretAccess   int       `json:"secretAccess"`
	RBACChanges    int       `json:"rbacChanges"`
}

// SecurityEventsRequest represents a request to analyze security events
type SecurityEventsRequest struct {
	ClusterID string          `json:"clusterId"`
	Timestamp time.Time       `json:"timestamp"`
	Events    []SecurityEvent `json:"events"`
	Context   SecurityContext `json:"context"`
}
