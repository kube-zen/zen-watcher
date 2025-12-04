// Copyright 2024 The Zen Watcher Authors
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

package watcher

import (
	"context"
	"testing"
	"time"
)

func TestFalcoAdapter(t *testing.T) {
	tests := []struct {
		name          string
		alert         map[string]interface{}
		expectEvent   bool
		expectedSev   string
		expectedType  string
	}{
		{
			name: "critical alert creates event",
			alert: map[string]interface{}{
				"priority": "Critical",
				"rule":     "Privileged container",
				"output":   "Privileged pod detected",
				"output_fields": map[string]interface{}{
					"k8s.pod.name": "test-pod",
					"k8s.ns.name":  "default",
				},
			},
			expectEvent:  true,
			expectedSev:  "HIGH",
			expectedType: "runtime-security",
		},
		{
			name: "warning alert creates event",
			alert: map[string]interface{}{
				"priority": "Warning",
				"rule":     "Shell spawned",
				"output":   "Shell detected",
				"output_fields": map[string]interface{}{
					"k8s.pod.name": "test-pod",
					"k8s.ns.name":  "default",
				},
			},
			expectEvent:  true,
			expectedSev:  "MEDIUM",
			expectedType: "runtime-security",
		},
		{
			name: "info alert filtered out",
			alert: map[string]interface{}{
				"priority": "Info",
				"rule":     "Debug event",
				"output":   "Info message",
			},
			expectEvent: false,
		},
		{
			name: "notice alert filtered out",
			alert: map[string]interface{}{
				"priority": "Notice",
				"rule":     "Notice event",
				"output":   "Notice message",
			},
			expectEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventChan := make(chan map[string]interface{}, 10)
			adapter := NewFalcoAdapter(eventChan)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Start adapter
			eventOut := make(chan *Event, 10)
			go adapter.Run(ctx, eventOut)

			// Send alert
			select {
			case eventChan <- tt.alert:
			case <-time.After(time.Second):
				t.Fatal("timeout sending alert")
			}

			// Check if event was created
			select {
			case event := <-eventOut:
				if !tt.expectEvent {
					t.Errorf("expected no event, got %+v", event)
					return
				}
				if event.Severity != tt.expectedSev {
					t.Errorf("severity = %v, want %v", event.Severity, tt.expectedSev)
				}
				if event.EventType != tt.expectedType {
					t.Errorf("eventType = %v, want %v", event.EventType, tt.expectedType)
				}
				if event.Source != "falco" {
					t.Errorf("source = %v, want falco", event.Source)
				}
			case <-time.After(100 * time.Millisecond):
				if tt.expectEvent {
					t.Error("expected event but none received")
				}
			}
		})
	}
}

func TestAuditAdapter(t *testing.T) {
	tests := []struct {
		name         string
		auditEvent   map[string]interface{}
		expectEvent  bool
		expectedType string
	}{
		{
			name: "secret deletion creates event",
			auditEvent: map[string]interface{}{
				"auditID": "test-1",
				"stage":   "ResponseComplete",
				"verb":    "delete",
				"user": map[string]interface{}{
					"username": "admin",
				},
				"objectRef": map[string]interface{}{
					"resource":  "secrets",
					"namespace": "default",
					"name":      "my-secret",
				},
				"responseStatus": map[string]interface{}{
					"code": 200,
				},
			},
			expectEvent:  true,
			expectedType: "resource-deletion",
		},
		{
			name: "rbac change creates event",
			auditEvent: map[string]interface{}{
				"auditID": "test-2",
				"stage":   "ResponseComplete",
				"verb":    "create",
				"user": map[string]interface{}{
					"username": "admin",
				},
				"objectRef": map[string]interface{}{
					"resource":  "rolebindings",
					"namespace": "default",
					"name":      "new-binding",
					"apiGroup":  "rbac.authorization.k8s.io",
				},
				"responseStatus": map[string]interface{}{
					"code": 201,
				},
			},
			expectEvent:  true,
			expectedType: "rbac-change",
		},
		{
			name: "configmap get filtered out",
			auditEvent: map[string]interface{}{
				"auditID": "test-3",
				"stage":   "ResponseComplete",
				"verb":    "get",
				"objectRef": map[string]interface{}{
					"resource":  "configmaps",
					"namespace": "default",
					"name":      "my-config",
				},
			},
			expectEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventChan := make(chan map[string]interface{}, 10)
			adapter := NewAuditAdapter(eventChan)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Start adapter
			eventOut := make(chan *Event, 10)
			go adapter.Run(ctx, eventOut)

			// Send audit event
			select {
			case eventChan <- tt.auditEvent:
			case <-time.After(time.Second):
				t.Fatal("timeout sending audit event")
			}

			// Check if event was created
			select {
			case event := <-eventOut:
				if !tt.expectEvent {
					t.Errorf("expected no event, got %+v", event)
					return
				}
				if event.EventType != tt.expectedType {
					t.Errorf("eventType = %v, want %v", event.EventType, tt.expectedType)
				}
				if event.Source != "audit" {
					t.Errorf("source = %v, want audit", event.Source)
				}
			case <-time.After(100 * time.Millisecond):
				if tt.expectEvent {
					t.Error("expected event but none received")
				}
			}
		})
	}
}

func TestKubeBenchAdapterName(t *testing.T) {
	clientSet := &fakeClientSet{}
	adapter := NewKubeBenchAdapter(clientSet)
	
	if adapter.Name() != "kubebench" {
		t.Errorf("Name() = %v, want kubebench", adapter.Name())
	}
}

func TestCheckovAdapterName(t *testing.T) {
	clientSet := &fakeClientSet{}
	adapter := NewCheckovAdapter(clientSet)
	
	if adapter.Name() != "checkov" {
		t.Errorf("Name() = %v, want checkov", adapter.Name())
	}
}

// Minimal fake ClientSet for name tests
type fakeClientSet struct{}

func (f *fakeClientSet) CoreV1() interface{} { return nil }

