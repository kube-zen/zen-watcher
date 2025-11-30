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
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// WebhookProcessor handles webhook-based events (Falco, Audit, etc.)
type WebhookProcessor struct {
	dynClient               dynamic.Interface
	eventGVR                schema.GroupVersionResource
	mu                      sync.RWMutex
	eventsTotal             *prometheus.CounterVec
	eventProcessingDuration *prometheus.HistogramVec
	totalCount              int64
	observationCreator      *ObservationCreator
}

// NewWebhookProcessor creates a new webhook processor
func NewWebhookProcessor(dynClient dynamic.Interface, eventGVR schema.GroupVersionResource, eventsTotal *prometheus.CounterVec, eventProcessingDuration *prometheus.HistogramVec, observationCreator *ObservationCreator) *WebhookProcessor {
	return &WebhookProcessor{
		dynClient:               dynClient,
		eventGVR:                eventGVR,
		eventsTotal:             eventsTotal,
		eventProcessingDuration: eventProcessingDuration,
		observationCreator:      observationCreator,
	}
}

// ProcessFalcoAlert processes a Falco webhook alert
func (wp *WebhookProcessor) ProcessFalcoAlert(ctx context.Context, alert map[string]interface{}) error {
	startTime := time.Now()
	defer func() {
		if wp.eventProcessingDuration != nil {
			wp.eventProcessingDuration.WithLabelValues("falco", "webhook").Observe(time.Since(startTime).Seconds())
		}
	}()
	priority := fmt.Sprintf("%v", alert["priority"])
	rule := fmt.Sprintf("%v", alert["rule"])
	output := fmt.Sprintf("%v", alert["output"])

	// Only process Warning, Error, Critical, Alert, Emergency
	if priority != "Warning" && priority != "Error" && priority != "Critical" && priority != "Alert" && priority != "Emergency" {
		logger.Debug("Skipping alert with low priority",
			logger.Fields{
				Component: "watcher",
				Operation: "process_falco",
				Source:    "falco",
				Reason:    "priority_below_threshold",
				Additional: map[string]interface{}{
					"priority": priority,
					"rule":     rule,
				},
			})
		return nil
	}

	logger.Debug("Processing Falco alert",
		logger.Fields{
			Component: "watcher",
			Operation: "process_falco",
			Source:    "falco",
			Additional: map[string]interface{}{
				"rule":     rule,
				"priority": priority,
			},
		})

	// Get K8s context if present (check output_fields first, then top level)
	var k8sPodName, k8sNs string
	if outputFields, ok := alert["output_fields"].(map[string]interface{}); ok {
		k8sPodName = fmt.Sprintf("%v", outputFields["k8s.pod.name"])
		k8sNs = fmt.Sprintf("%v", outputFields["k8s.ns.name"])
	}
	// Fallback to top-level fields if not in output_fields
	if k8sPodName == "" || k8sPodName == "<nil>" {
		k8sPodName = fmt.Sprintf("%v", alert["k8s.pod.name"])
	}
	if k8sNs == "" || k8sNs == "<nil>" {
		k8sNs = fmt.Sprintf("%v", alert["k8s.ns.name"])
	}
	if k8sNs == "<nil>" || k8sNs == "" {
		k8sNs = "default"
	}

	// Map priority to severity
	severity := "MEDIUM"
	if priority == "Critical" || priority == "Alert" || priority == "Emergency" {
		severity = "HIGH"
	}

	event := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"generateName": "falco-",
				"namespace":    k8sNs,
				"labels": map[string]interface{}{
					"source":   "falco",
					"category": "security",
					"severity": severity,
				},
			},
			"spec": map[string]interface{}{
				"source":     "falco",
				"category":   "security",
				"severity":   severity,
				"eventType":  "runtime-security",
				"detectedAt": time.Now().Format(time.RFC3339),
				"resource": map[string]interface{}{
					"kind":      "Pod",
					"name":      k8sPodName,
					"namespace": k8sNs,
				},
				"details": map[string]interface{}{
					"rule":         rule,
					"priority":     priority,
					"output":       output,
					"k8s_pod_name": k8sPodName,
					"k8s_ns_name":  k8sNs,
				},
			},
		},
	}

	// Use centralized observation creator - metrics are incremented automatically
	err := wp.observationCreator.CreateObservation(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to create Falco Observation: %v", err)
	}

	wp.mu.Lock()
	wp.totalCount++
	wp.mu.Unlock()

	logger.Info("Created Observation for Falco alert",
		logger.Fields{
			Component: "watcher",
			Operation: "observation_create",
			Source:    "falco",
			EventType: "runtime-security",
			Severity:  severity,
			Additional: map[string]interface{}{
				"rule":     rule,
				"priority": priority,
			},
		})
	return nil
}

// ProcessAuditEvent processes a Kubernetes audit webhook event
func (wp *WebhookProcessor) ProcessAuditEvent(ctx context.Context, auditEvent map[string]interface{}) error {
	startTime := time.Now()
	defer func() {
		if wp.eventProcessingDuration != nil {
			wp.eventProcessingDuration.WithLabelValues("audit", "webhook").Observe(time.Since(startTime).Seconds())
		}
	}()
	auditID := fmt.Sprintf("%v", auditEvent["auditID"])
	stage := fmt.Sprintf("%v", auditEvent["stage"])
	verb := fmt.Sprintf("%v", auditEvent["verb"])

	// Only process ResponseComplete stage
	if stage != "ResponseComplete" {
		logger.Debug("Skipping audit event with non-ResponseComplete stage",
			logger.Fields{
				Component: "watcher",
				Operation: "process_audit",
				Source:    "audit",
				Reason:    "stage_not_response_complete",
				Additional: map[string]interface{}{
					"stage":    stage,
					"audit_id": auditID,
				},
			})
		return nil
	}

	// Filter for important actions
	objectRef, ok := auditEvent["objectRef"].(map[string]interface{})
	if !ok {
		objectRef = make(map[string]interface{})
	}
	resource := fmt.Sprintf("%v", objectRef["resource"])
	namespace := fmt.Sprintf("%v", objectRef["namespace"])
	name := fmt.Sprintf("%v", objectRef["name"])
	apiGroup := fmt.Sprintf("%v", objectRef["apiGroup"])

	logger.Debug("Processing audit event",
		logger.Fields{
			Component: "watcher",
			Operation: "process_audit",
			Source:    "audit",
			Namespace: namespace,
			Additional: map[string]interface{}{
				"verb":     verb,
				"resource": resource,
				"name":     name,
				"audit_id": auditID,
			},
		})

	// Filter logic: only important events
	important := false
	category := "compliance"
	severity := "MEDIUM"
	eventType := "audit-event"

	// Delete operations (HIGH severity)
	if verb == "delete" {
		important = true
		severity = "HIGH"
		eventType = "resource-deletion"
	}

	// Secret/ConfigMap operations
	if resource == "secrets" || resource == "configmaps" {
		if verb == "create" || verb == "update" || verb == "patch" || verb == "delete" {
			important = true
			severity = "HIGH"
			eventType = "secret-access"
		}
	}

	// RBAC changes
	if apiGroup == "rbac.authorization.k8s.io" {
		if verb == "create" || verb == "update" || verb == "patch" || verb == "delete" {
			important = true
			severity = "HIGH"
			eventType = "rbac-change"
		}
	}

	// Privileged pod creation
	if resource == "pods" && verb == "create" {
		requestObject, ok := auditEvent["requestObject"].(map[string]interface{})
		if ok && requestObject != nil {
			spec, ok := requestObject["spec"].(map[string]interface{})
			if ok && spec != nil {
				containers, ok := spec["containers"].([]interface{})
				if ok {
					for _, c := range containers {
						container, ok := c.(map[string]interface{})
						if !ok {
							continue
						}
						securityContext, ok := container["securityContext"].(map[string]interface{})
						if ok && securityContext != nil {
							privileged, ok := securityContext["privileged"].(bool)
							if ok && privileged {
								important = true
								severity = "HIGH"
								eventType = "privileged-pod-creation"
								break
							}
						}
					}
				}
			}
		}
	}

	if !important {
		logger.Debug("Audit event filtered out (not important)",
			logger.Fields{
				Component: "watcher",
				Operation: "process_audit",
				Source:    "audit",
				Reason:    "not_important",
				Namespace: namespace,
				Additional: map[string]interface{}{
					"verb":     verb,
					"resource": resource,
					"name":     name,
				},
			})
		return nil
	}
	logger.Debug("Processing important audit event",
		logger.Fields{
			Component: "watcher",
			Operation: "process_audit",
			Source:    "audit",
			Severity:  severity,
			Namespace: namespace,
			Additional: map[string]interface{}{
				"verb":     verb,
				"resource": resource,
				"name":     name,
			},
		})

	// Extract user info
	user, ok := auditEvent["user"].(map[string]interface{})
	if !ok {
		user = make(map[string]interface{})
	}
	username := fmt.Sprintf("%v", user["username"])

	// Extract response code
	responseStatus, ok := auditEvent["responseStatus"].(map[string]interface{})
	if !ok {
		responseStatus = make(map[string]interface{})
	}
	statusCode := fmt.Sprintf("%v", responseStatus["code"])

	if namespace == "<nil>" || namespace == "" {
		namespace = "default"
	}

	event := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"generateName": "audit-",
				"namespace":    namespace,
				"labels": map[string]interface{}{
					"source":   "audit",
					"category": category,
					"severity": severity,
				},
			},
			"spec": map[string]interface{}{
				"source":     "audit",
				"category":   category,
				"severity":   severity,
				"eventType":  eventType,
				"detectedAt": time.Now().Format(time.RFC3339),
				"resource": map[string]interface{}{
					"kind":      resource,
					"name":      name,
					"namespace": namespace,
					"apiGroup":  apiGroup,
				},
				"details": map[string]interface{}{
					"auditID":    auditID,
					"verb":       verb,
					"user":       username,
					"stage":      stage,
					"statusCode": statusCode,
				},
			},
		},
	}

	// Use centralized observation creator - metrics are incremented automatically
	err := wp.observationCreator.CreateObservation(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to create Audit Observation: %v", err)
	}

	wp.mu.Lock()
	wp.totalCount++
	wp.mu.Unlock()

	logger.Info("Created Observation for Audit event",
		logger.Fields{
			Component: "watcher",
			Operation: "observation_create",
			Source:    "audit",
			EventType: eventType,
			Severity:  severity,
			Namespace: namespace,
			Additional: map[string]interface{}{
				"verb":     verb,
				"resource": resource,
				"name":     name,
			},
		})
	return nil
}

// GetTotalCount returns the total number of events created
func (wp *WebhookProcessor) GetTotalCount() int64 {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.totalCount
}
