package types

import (
	"time"
)

// SecurityEvent represents a security event detected by watchers
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Source      string                 `json:"source"`
	Type        string                 `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	Severity    string                 `json:"severity"`
	Namespace   string                 `json:"namespace"`
	Resource    string                 `json:"resource"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details"`
}

// SecurityEventsRequest represents a request to send security events
type SecurityEventsRequest struct {
	Timestamp time.Time       `json:"timestamp"`
	Events    []SecurityEvent `json:"events"`
	Context   SecurityContext `json:"context"`
}

// SecurityContext provides context about the security state
type SecurityContext struct {
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

// TrivyEvent represents a Trivy vulnerability event
type TrivyEvent struct {
	ID            string    `json:"id"`
	Vulnerability string    `json:"vulnerability"`
	Severity      string    `json:"severity"`
	CVSS          float64   `json:"cvss"`
	Package       string    `json:"package"`
	Version       string    `json:"version"`
	Namespace     string    `json:"namespace"`
	ResourceName  string    `json:"resourceName"`
	ResourceType  string    `json:"resourceType"`
	Description   string    `json:"description"`
	Timestamp     time.Time `json:"timestamp"`
}

// FalcoEvent represents a Falco security event
type FalcoEvent struct {
	ID        string    `json:"id"`
	Rule      string    `json:"rule"`
	Priority  string    `json:"priority"`
	Message   string    `json:"message"`
	Namespace string    `json:"namespace"`
	Pod       string    `json:"pod"`
	Container string    `json:"container"`
	User      string    `json:"user"`
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
}

// AuditEvent represents a Kubernetes audit log event
type AuditEvent struct {
	ID         string    `json:"id"`
	Verb       string    `json:"verb"`
	User       string    `json:"user"`
	UserAgent  string    `json:"userAgent"`
	SourceIPs  []string  `json:"sourceIPs"`
	APIVersion string    `json:"apiVersion"`
	ResourceID string    `json:"resourceId"`
	Namespace  string    `json:"namespace"`
	Resource   string    `json:"resource"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}
