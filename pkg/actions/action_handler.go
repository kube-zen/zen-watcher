package actions

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapActionHandler handles creating ConfigMaps for security events
type ConfigMapActionHandler struct {
	clientSet *kubernetes.Clientset
	namespace string
}

// NewConfigMapActionHandler creates a new ConfigMap action handler
func NewConfigMapActionHandler(clientSet *kubernetes.Clientset, namespace string) *ConfigMapActionHandler {
	return &ConfigMapActionHandler{
		clientSet: clientSet,
		namespace: namespace,
	}
}

// HandleTrivyUpdate processes Trivy updates by creating ConfigMaps
func (h *ConfigMapActionHandler) HandleTrivyUpdate(ctx context.Context, logLine string) error {
	log.Printf("🔍 [TRIVY] Processing Trivy Update: %s", logLine)

	// Create a ConfigMap with Trivy event data
	cmName := "trivy-event-" + fmt.Sprintf("%d", time.Now().Unix())
	cm := &corev1."ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: h.namespace,
			Labels: map[string]string{
				"app":                    "zen-watcher",
				"source":                 "trivy",
				"type":                   "vulnerability",
				"zen.kube-zen.com/event": "true",
			},
			Annotations: map[string]string{
				"zen.kube-zen.com/timestamp": time.Now().Format(time.RFC3339),
				"zen.kube-zen.com/source":    "trivy",
			},
		},
		Data: map[string]string{
			"event.yaml": fmt.Sprintf(`
event:
  source: "trivy"
  timestamp: %s
  log_line: "%s"
  status: "detected"
`, time.Now().UTC().Format("2006-01-02T15:04:05Z"), logLine),
		},
	}

	_, err := h.clientSet.CoreV1().ConfigMaps(h.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Trivy ConfigMap %s: %v", cmName, err)
	}

	log.Printf("✅ Created Trivy ConfigMap: %s", cmName)
	return nil
}

// HandleFalcoUpdate processes Falco updates by creating ConfigMaps
func (h *ConfigMapActionHandler) HandleFalcoUpdate(ctx context.Context, logLine string) error {
	log.Printf("🔍 [FALCO] Processing Falco Update: %s", logLine)

	// Create a ConfigMap with Falco event data
	cmName := "falco-event-" + fmt.Sprintf("%d", time.Now().Unix())
	cm := &corev1."ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: h.namespace,
			Labels: map[string]string{
				"app":                    "zen-watcher",
				"source":                 "falco",
				"type":                   "runtime",
				"zen.kube-zen.com/event": "true",
			},
			Annotations: map[string]string{
				"zen.kube-zen.com/timestamp": time.Now().Format(time.RFC3339),
				"zen.kube-zen.com/source":    "falco",
			},
		},
		Data: map[string]string{
			"event.yaml": fmt.Sprintf(`
event:
  source: "falco"
  timestamp: %s
  log_line: "%s"
  status: "detected"
`, time.Now().UTC().Format("2006-01-02T15:04:05Z"), logLine),
		},
	}

	_, err := h.clientSet.CoreV1().ConfigMaps(h.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Falco ConfigMap %s: %v", cmName, err)
	}

	log.Printf("✅ Created Falco ConfigMap: %s", cmName)
	return nil
}

// HandleAuditUpdate processes Audit updates by creating ConfigMaps
func (h *ConfigMapActionHandler) HandleAuditUpdate(ctx context.Context, logLine string) error {
	log.Printf("🔍 [AUDIT] Processing Audit Update: %s", logLine)

	// Create a ConfigMap with Audit event data
	cmName := "audit-event-" + fmt.Sprintf("%d", time.Now().Unix())
	cm := &corev1."ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: h.namespace,
			Labels: map[string]string{
				"app":                    "zen-watcher",
				"source":                 "audit",
				"type":                   "audit",
				"zen.kube-zen.com/event": "true",
			},
			Annotations: map[string]string{
				"zen.kube-zen.com/timestamp": time.Now().Format(time.RFC3339),
				"zen.kube-zen.com/source":    "audit",
			},
		},
		Data: map[string]string{
			"event.yaml": fmt.Sprintf(`
event:
  source: "audit"
  timestamp: %s
  log_line: "%s"
  status: "detected"
`, time.Now().UTC().Format("2006-01-02T15:04:05Z"), logLine),
		},
	}

	_, err := h.clientSet.CoreV1().ConfigMaps(h.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Audit ConfigMap %s: %v", cmName, err)
	}

	log.Printf("✅ Created Audit ConfigMap: %s", cmName)
	return nil
}

// HandleKyvernoPolicyViolation handles Kyverno policy violations by creating ConfigMaps
func (h *ConfigMapActionHandler) HandleKyvernoPolicyViolation(ctx context.Context, policyName, resourceName, namespace, message string) error {
	log.Printf("🔍 [KYVERNO] Processing Policy Violation: %s/%s", policyName, resourceName)

	// Create a ConfigMap with Kyverno event data
	cmName := "kyverno-violation-" + fmt.Sprintf("%d", time.Now().Unix())
	cm := &corev1."ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: h.namespace,
			Labels: map[string]string{
				"app":                    "zen-watcher",
				"source":                 "kyverno",
				"type":                   "policy-violation",
				"policy-name":            policyName,
				"zen.kube-zen.com/event": "true",
			},
			Annotations: map[string]string{
				"zen.kube-zen.com/timestamp": time.Now().Format(time.RFC3339),
				"zen.kube-zen.com/source":    "kyverno",
			},
		},
		Data: map[string]string{
			"event.yaml": fmt.Sprintf(`
event:
  source: "kyverno"
  timestamp: %s
  policy_name: "%s"
  resource_name: "%s"
  namespace: "%s"
  message: "%s"
  status: "violation_detected"
`, time.Now().UTC().Format("2006-01-02T15:04:05Z"), policyName, resourceName, namespace, message),
		},
	}

	_, err := h.clientSet.CoreV1().ConfigMaps(h.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Kyverno ConfigMap %s: %v", cmName, err)
	}

	log.Printf("✅ Created Kyverno ConfigMap: %s", cmName)
	return nil
}
