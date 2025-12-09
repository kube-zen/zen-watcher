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

package watcher

import (
	"context"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// K8sEventsAdapter implements SourceAdapter for native Kubernetes Events API
// This adapter watches Kubernetes Events and filters them for security-relevant events
type K8sEventsAdapter struct {
	clientSet kubernetes.Interface
	watcher   watch.Interface
	stopCh    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewK8sEventsAdapter creates a new Kubernetes Events adapter
func NewK8sEventsAdapter(clientSet kubernetes.Interface) *K8sEventsAdapter {
	return &K8sEventsAdapter{
		clientSet: clientSet,
		stopCh:    make(chan struct{}),
	}
}

func (a *K8sEventsAdapter) Name() string {
	return "kubernetes-events"
}

func (a *K8sEventsAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	logger.Info("Starting Kubernetes Events watcher",
		logger.Fields{
			Component: "watcher",
			Operation: "k8s_events_adapter_start",
			Source:    "kubernetes-events",
		})

	// Create a field selector to filter events
	// Focus on Warning/Error type events which are more likely to be security-relevant
	fieldSelector := fields.AndSelectors(
		fields.OneTermNotEqualSelector("type", "Normal"), // Only Warning/Error events
	).String()

	// Watch events across all namespaces
	watcher, err := a.clientSet.CoreV1().Events("").Watch(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
		Watch:         true,
	})
	if err != nil {
		logger.Error("Failed to create Kubernetes Events watcher",
			logger.Fields{
				Component: "watcher",
				Operation: "k8s_events_watcher_create",
				Source:    "kubernetes-events",
				Error:     err,
			})
		return err
	}
	a.watcher = watcher

	// Process events from watcher
	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					logger.Warn("Kubernetes Events watcher channel closed",
						logger.Fields{
							Component: "watcher",
							Operation: "k8s_events_watcher_closed",
							Source:    "kubernetes-events",
						})
					return
				}

				// Convert Kubernetes Event to normalized Event
				if k8sEvent, ok := event.Object.(*corev1.Event); ok {
					normalizedEvent := a.convertToNormalizedEvent(k8sEvent)
					if normalizedEvent != nil {
						// Only send security-relevant events
						if a.isSecurityRelevant(k8sEvent) {
							select {
							case out <- normalizedEvent:
							case <-a.ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}()

	// Block until context cancelled
	<-a.ctx.Done()
	return a.ctx.Err()
}

func (a *K8sEventsAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.watcher != nil {
		a.watcher.Stop()
	}
	close(a.stopCh)
}

// isSecurityRelevant determines if a Kubernetes Event is security-relevant
// This filters out operational noise and focuses on security events
func (a *K8sEventsAdapter) isSecurityRelevant(event *corev1.Event) bool {
	reason := event.Reason
	message := event.Message
	involvedKind := event.InvolvedObject.Kind

	// Security-relevant reasons
	securityReasons := []string{
		"Failed",           // Failed authentication, failed mount, etc.
		"Unauthorized",     // Unauthorized access attempts
		"Forbidden",        // Forbidden operations
		"FailedCreate",     // Failed pod/container creation (potential security issue)
		"FailedDelete",     // Failed deletions (potential privilege escalation)
		"FailedMount",      // Failed volume mounts (security implications)
		"BackOff",          // Backoff events (could indicate DoS attempts)
		"FailedSync",       // Failed synchronization (could indicate security issues)
		"FailedAttach",     // Failed volume attachment
		"FailedPull",       // Failed image pulls (supply chain security)
		"FailedValidation", // Failed validation (policy violations)
	}

	reasonLower := strings.ToLower(reason)
	for _, securityReason := range securityReasons {
		if strings.Contains(reasonLower, strings.ToLower(securityReason)) {
			return true
		}
	}

	// Security-relevant message keywords
	securityKeywords := []string{
		"unauthorized",
		"forbidden",
		"authentication failed",
		"authorization failed",
		"access denied",
		"permission denied",
		"privilege escalation",
		"security policy",
		"pod security",
		"network policy",
		"rbac",
		"service account",
		"secret",
		"certificate",
		"tls",
		"quota exceeded", // Potential DoS
		"resource quota", // Potential DoS
	}

	messageLower := strings.ToLower(message)
	for _, keyword := range securityKeywords {
		if strings.Contains(messageLower, keyword) {
			return true
		}
	}

	// Security-relevant resource kinds
	securityKinds := []string{
		"Secret",
		"ServiceAccount",
		"Role",
		"RoleBinding",
		"ClusterRole",
		"ClusterRoleBinding",
		"NetworkPolicy",
		"PodSecurityPolicy",
		"Pod",
		"Deployment",
		"StatefulSet",
		"DaemonSet",
	}

	for _, securityKind := range securityKinds {
		if involvedKind == securityKind {
			return true
		}
	}

	return false
}

// convertToNormalizedEvent converts a Kubernetes Event to a normalized Event
func (a *K8sEventsAdapter) convertToNormalizedEvent(k8sEvent *corev1.Event) *Event {
	if k8sEvent == nil {
		return nil
	}

	// Determine severity based on event type and reason
	severity := a.determineSeverity(k8sEvent)

	// Determine event type based on reason and involved object
	eventType := a.determineEventType(k8sEvent)

	// Build details map
	details := map[string]interface{}{
		"reason":         k8sEvent.Reason,
		"message":        k8sEvent.Message,
		"type":           k8sEvent.Type,
		"firstTimestamp": k8sEvent.FirstTimestamp.Format(time.RFC3339),
		"lastTimestamp":  k8sEvent.LastTimestamp.Format(time.RFC3339),
		"count":          k8sEvent.Count,
		"source": map[string]interface{}{
			"component": k8sEvent.Source.Component,
			"host":      k8sEvent.Source.Host,
		},
	}

	// Add involved object details
	if k8sEvent.InvolvedObject.Kind != "" {
		details["involvedObject"] = map[string]interface{}{
			"kind":            k8sEvent.InvolvedObject.Kind,
			"name":            k8sEvent.InvolvedObject.Name,
			"namespace":       k8sEvent.InvolvedObject.Namespace,
			"apiVersion":      k8sEvent.InvolvedObject.APIVersion,
			"resourceVersion": k8sEvent.InvolvedObject.ResourceVersion,
			"uid":             string(k8sEvent.InvolvedObject.UID),
		}
	}

	// Build resource reference
	resource := &ResourceRef{
		Kind:      k8sEvent.InvolvedObject.Kind,
		Name:      k8sEvent.InvolvedObject.Name,
		Namespace: k8sEvent.InvolvedObject.Namespace,
	}

	if k8sEvent.InvolvedObject.APIVersion != "" {
		resource.APIVersion = k8sEvent.InvolvedObject.APIVersion
	}

	detectedAt := k8sEvent.LastTimestamp.Format(time.RFC3339)
	if detectedAt == "" {
		detectedAt = time.Now().Format(time.RFC3339)
	}

	return &Event{
		Source:     "kubernetes-events",
		Category:   "security", // K8s events are primarily security/compliance related
		Severity:   severity,
		EventType:  eventType,
		Resource:   resource,
		Details:    details,
		Namespace:  k8sEvent.InvolvedObject.Namespace,
		DetectedAt: detectedAt,
	}
}

// determineSeverity determines the severity level based on event type and reason
func (a *K8sEventsAdapter) determineSeverity(k8sEvent *corev1.Event) string {
	// Critical security events
	criticalReasons := []string{
		"Unauthorized",
		"Forbidden",
		"FailedDelete",
		"PrivilegeEscalation",
	}

	reason := strings.ToLower(k8sEvent.Reason)
	for _, criticalReason := range criticalReasons {
		if strings.Contains(reason, strings.ToLower(criticalReason)) {
			return "CRITICAL"
		}
	}

	// High severity events
	highReasons := []string{
		"Failed",
		"FailedCreate",
		"FailedMount",
		"FailedPull",
		"FailedValidation",
		"BackOff",
	}

	for _, highReason := range highReasons {
		if strings.Contains(reason, strings.ToLower(highReason)) {
			return "HIGH"
		}
	}

	// Default to MEDIUM for Warning events, LOW for others
	if k8sEvent.Type == corev1.EventTypeWarning {
		return "MEDIUM"
	}

	return "LOW"
}

// determineEventType determines the event type based on reason and involved object
func (a *K8sEventsAdapter) determineEventType(k8sEvent *corev1.Event) string {
	reason := strings.ToLower(k8sEvent.Reason)
	kind := strings.ToLower(k8sEvent.InvolvedObject.Kind)

	// Authentication/Authorization events
	if strings.Contains(reason, "unauthorized") || strings.Contains(reason, "forbidden") {
		return "access-control-violation"
	}

	// Policy violation events
	if strings.Contains(reason, "policy") || strings.Contains(reason, "validation") {
		return "policy-violation"
	}

	// Network policy events
	if strings.Contains(kind, "networkpolicy") || strings.Contains(reason, "network") {
		return "network-policy-violation"
	}

	// Pod security events
	if strings.Contains(reason, "pod") && strings.Contains(reason, "security") {
		return "pod-security-violation"
	}

	// Resource quota events (potential DoS)
	if strings.Contains(reason, "quota") || strings.Contains(reason, "resource") {
		return "resource-exhaustion"
	}

	// Image pull events (supply chain)
	if strings.Contains(reason, "pull") || strings.Contains(reason, "image") {
		return "image-pull-failure"
	}

	// Volume mount events
	if strings.Contains(reason, "mount") || strings.Contains(reason, "volume") {
		return "storage-access-failure"
	}

	// Default event type
	return "kubernetes-event"
}

// GetOptimizationMetrics returns optimization metrics (not implemented for K8s Events)
func (a *K8sEventsAdapter) GetOptimizationMetrics() interface{} {
	return nil
}

// ApplyOptimization applies optimization configuration (not implemented for K8s Events)
func (a *K8sEventsAdapter) ApplyOptimization(config interface{}) error {
	return nil
}

// ValidateOptimization validates optimization configuration (not implemented for K8s Events)
func (a *K8sEventsAdapter) ValidateOptimization(config interface{}) error {
	return nil
}

// ResetMetrics resets optimization metrics (not implemented for K8s Events)
func (a *K8sEventsAdapter) ResetMetrics() {
}
