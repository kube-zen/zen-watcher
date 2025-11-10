package actions

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/watcher"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KyvernoActionHandler handles Kyverno policy violations and events
type KyvernoActionHandler struct {
	clientSet    *kubernetes.Clientset
	namespace    string
	recentEvents []watcher.KyvernoViolation
	maxEvents    int
}

// NewKyvernoActionHandler creates a new Kyverno action handler
func NewKyvernoActionHandler(clientSet *kubernetes.Clientset, namespace string) *KyvernoActionHandler {
	return &KyvernoActionHandler{
		clientSet:    clientSet,
		namespace:    namespace,
		recentEvents: make([]watcher.KyvernoViolation, 0),
		maxEvents:    100, // Keep last 100 events
	}
}

// HandleKyvernoPolicyViolation handles Kyverno policy violations
func (kah *KyvernoActionHandler) HandleKyvernoPolicyViolation(ctx context.Context, violation *watcher.KyvernoViolation) error {
	log.Printf("🚨 [KYVERNO-ACTION] Policy violation detected: %s/%s - %s",
		violation.PolicyName, violation.ResourceName, violation.Message)

	// Add to recent events
	kah.addRecentEvent(*violation)

	// Create ConfigMap for the violation
	configMapName := fmt.Sprintf("kyverno-violation-%s-%d", violation.PolicyName, time.Now().Unix())

	configMapData := map[string]string{
		"policyName":    violation.PolicyName,
		"policyType":    violation.PolicyType,
		"resourceKind":  violation.ResourceKind,
		"resourceName":  violation.ResourceName,
		"namespace":     violation.Namespace,
		"violationType": violation.ViolationType,
		"message":       violation.Message,
		"timestamp":     violation.Timestamp.Format(time.RFC3339),
		"rule":          violation.Details["rule"],
		"result":        violation.Details["result"],
	}

	// Add additional details
	for key, value := range violation.Details {
		configMapData[fmt.Sprintf("detail_%s", key)] = value
	}

	configMap := &corev1."ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: kah.namespace,
			Labels: map[string]string{
				"app":                    "zen-watcher",
				"source":                 "kyverno",
				"type":                   "policy-violation",
				"policy-name":            violation.PolicyName,
				"violation-type":         violation.ViolationType,
				"zen.kube-zen.com/event": "true",
			},
			Annotations: map[string]string{
				"zen.kube-zen.com/timestamp": violation.Timestamp.Format(time.RFC3339),
				"zen.kube-zen.com/source":    "kyverno",
				"zen.kube-zen.com/severity":  kah.mapViolationTypeToSeverity(violation.ViolationType),
			},
		},
		Data: configMapData,
	}

	_, err := kah.clientSet.CoreV1().ConfigMaps(kah.namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		log.Printf("❌ [KYVERNO-ACTION] Failed to create ConfigMap for violation: %v", err)
		return err
	}

	log.Printf("✅ [KYVERNO-ACTION] Created ConfigMap %s for Kyverno violation", configMapName)
	return nil
}

// HandleKyvernoPolicyEvent handles Kyverno policy events
func (kah *KyvernoActionHandler) HandleKyvernoPolicyEvent(ctx context.Context, event *watcher.KyvernoEvent) error {
	log.Printf("📋 [KYVERNO-ACTION] Policy event: %s %s - %s",
		event.EventType, event.PolicyName, event.Message)

	// Create ConfigMap for the policy event
	configMapName := fmt.Sprintf("kyverno-policy-%s-%d", event.PolicyName, time.Now().Unix())

	configMapData := map[string]string{
		"eventType":  event.EventType,
		"policyName": event.PolicyName,
		"policyType": event.PolicyType,
		"namespace":  event.Namespace,
		"message":    event.Message,
		"timestamp":  event.Timestamp.Format(time.RFC3339),
	}

	// Add additional details
	for key, value := range event.Details {
		configMapData[fmt.Sprintf("detail_%s", key)] = value
	}

	configMap := &corev1."ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: kah.namespace,
			Labels: map[string]string{
				"app":                    "zen-watcher",
				"source":                 "kyverno",
				"type":                   "policy-event",
				"policy-name":            event.PolicyName,
				"event-type":             event.EventType,
				"zen.kube-zen.com/event": "true",
			},
			Annotations: map[string]string{
				"zen.kube-zen.com/timestamp": event.Timestamp.Format(time.RFC3339),
				"zen.kube-zen.com/source":    "kyverno",
				"zen.kube-zen.com/severity":  "info",
			},
		},
		Data: configMapData,
	}

	_, err := kah.clientSet.CoreV1().ConfigMaps(kah.namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		log.Printf("❌ [KYVERNO-ACTION] Failed to create ConfigMap for policy event: %v", err)
		return err
	}

	log.Printf("✅ [KYVERNO-ACTION] Created ConfigMap %s for Kyverno policy event", configMapName)
	return nil
}

// GetRecentEvents returns recent Kyverno violations
func (kah *KyvernoActionHandler) GetRecentEvents() []watcher.KyvernoViolation {
	return kah.recentEvents
}

// addRecentEvent adds a violation to recent events
func (kah *KyvernoActionHandler) addRecentEvent(violation watcher.KyvernoViolation) {
	kah.recentEvents = append(kah.recentEvents, violation)

	// Keep only the most recent events
	if len(kah.recentEvents) > kah.maxEvents {
		kah.recentEvents = kah.recentEvents[len(kah.recentEvents)-kah.maxEvents:]
	}
}

// mapViolationTypeToSeverity maps violation types to severity levels
func (kah *KyvernoActionHandler) mapViolationTypeToSeverity(violationType string) string {
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
