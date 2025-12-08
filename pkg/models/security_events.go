// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
