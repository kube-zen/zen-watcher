package watcher

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// WebhookProcessor handles webhook-based events (Falco, Audit, etc.)
type WebhookProcessor struct {
	dynClient               dynamic.Interface
	eventGVR                schema.GroupVersionResource
	dedupKeys               map[string]map[string]bool // source -> dedupKey -> bool
	mu                      sync.RWMutex
	eventsTotal             *prometheus.CounterVec
	eventProcessingDuration *prometheus.HistogramVec
	totalCount              int64
}

// NewWebhookProcessor creates a new webhook processor
func NewWebhookProcessor(dynClient dynamic.Interface, eventGVR schema.GroupVersionResource, eventsTotal *prometheus.CounterVec, eventProcessingDuration *prometheus.HistogramVec) *WebhookProcessor {
	return &WebhookProcessor{
		dynClient:               dynClient,
		eventGVR:                eventGVR,
		dedupKeys:               make(map[string]map[string]bool),
		eventsTotal:             eventsTotal,
		eventProcessingDuration: eventProcessingDuration,
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
		return nil
	}

	// Get K8s context if present
	k8sPodName := fmt.Sprintf("%v", alert["k8s.pod.name"])
	k8sNs := fmt.Sprintf("%v", alert["k8s.ns.name"])
	if k8sNs == "<nil>" || k8sNs == "" {
		k8sNs = "default"
	}

	// Dedup key: rule + pod + output (truncated)
	outputKey := output
	if len(output) > 50 {
		outputKey = output[:50]
	}
	dedupKey := fmt.Sprintf("%s/%s/%s", rule, k8sPodName, outputKey)

	wp.mu.RLock()
	if wp.dedupKeys["falco"] == nil {
		wp.dedupKeys["falco"] = make(map[string]bool)
	}
	exists := wp.dedupKeys["falco"][dedupKey]
	wp.mu.RUnlock()

	if exists {
		return nil
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

	_, err := wp.dynClient.Resource(wp.eventGVR).Namespace(k8sNs).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Falco Observation: %v", err)
	}

	wp.mu.Lock()
	wp.dedupKeys["falco"][dedupKey] = true
	wp.totalCount++
	wp.mu.Unlock()

	if wp.eventsTotal != nil {
		wp.eventsTotal.WithLabelValues("falco", "security", severity).Inc()
	}

	log.Printf("  ✅ Created Observation for Falco alert: %s (priority: %s)", rule, priority)
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
		return nil
	}

	// Dedup check
	wp.mu.RLock()
	if wp.dedupKeys["audit"] == nil {
		wp.dedupKeys["audit"] = make(map[string]bool)
	}
	exists := wp.dedupKeys["audit"][auditID]
	wp.mu.RUnlock()

	if exists {
		return nil
	}

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

	_, err := wp.dynClient.Resource(wp.eventGVR).Namespace(namespace).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Audit Observation: %v", err)
	}

	wp.mu.Lock()
	wp.dedupKeys["audit"][auditID] = true
	wp.totalCount++
	wp.mu.Unlock()

	if wp.eventsTotal != nil {
		wp.eventsTotal.WithLabelValues("audit", category, severity).Inc()
	}

	log.Printf("  ✅ Created Observation for Audit event: %s %s/%s", verb, resource, name)
	return nil
}

// GetTotalCount returns the total number of events created
func (wp *WebhookProcessor) GetTotalCount() int64 {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.totalCount
}
