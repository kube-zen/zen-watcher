package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// KyvernoWatcher watches for Kyverno policy violations and events
type KyvernoWatcher struct {
	clientSet     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	namespace     string
	actionHandler KyvernoActionHandler
	stopCh        chan struct{}
}

// KyvernoActionHandler interface for handling Kyverno events
type KyvernoActionHandler interface {
	HandleKyvernoPolicyViolation(ctx context.Context, violation *KyvernoViolation) error
	HandleKyvernoPolicyEvent(ctx context.Context, event *KyvernoEvent) error
	GetRecentEvents() []KyvernoViolation
}

// KyvernoViolation represents a Kyverno policy violation
type KyvernoViolation struct {
	PolicyName    string            `json:"policyName"`
	PolicyType    string            `json:"policyType"` // validate, mutate, generate
	ResourceKind  string            `json:"resourceKind"`
	ResourceName  string            `json:"resourceName"`
	Namespace     string            `json:"namespace"`
	ViolationType string            `json:"violationType"` // blocked, failed, warning
	Message       string            `json:"message"`
	Timestamp     time.Time         `json:"timestamp"`
	Details       map[string]string `json:"details"`
}

// KyvernoEvent represents a Kyverno policy event
type KyvernoEvent struct {
	EventType  string            `json:"eventType"` // policy_created, policy_updated, policy_deleted, violation_detected
	PolicyName string            `json:"policyName"`
	PolicyType string            `json:"policyType"`
	Namespace  string            `json:"namespace"`
	Message    string            `json:"message"`
	Timestamp  time.Time         `json:"timestamp"`
	Details    map[string]string `json:"details"`
}

// NewKyvernoWatcher creates a new Kyverno watcher
func NewKyvernoWatcher(clientSet *kubernetes.Clientset, dynamicClient dynamic.Interface, namespace string, actionHandler KyvernoActionHandler) *KyvernoWatcher {
	return &KyvernoWatcher{
		clientSet:     clientSet,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		actionHandler: actionHandler,
		stopCh:        make(chan struct{}),
	}
}

// Start starts watching Kyverno policies and violations
func (kw *KyvernoWatcher) Start(ctx context.Context) error {
	log.Printf("🔍 [KYVERNO-WATCHER] Starting Kyverno policy monitoring...")

	// Start watching PolicyReports (namespace-scoped)
	go kw.watchPolicyReports(ctx)

	// Start watching ClusterPolicyReports (cluster-scoped)
	go kw.watchClusterPolicyReports(ctx)

	// Start watching Kyverno policies
	go kw.watchKyvernoPolicies(ctx)

	log.Printf("✅ [KYVERNO-WATCHER] Kyverno monitoring started successfully")
	return nil
}

// Stop stops the Kyverno watcher
func (kw *KyvernoWatcher) Stop() {
	log.Printf("🛑 [KYVERNO-WATCHER] Stopping Kyverno monitoring...")
	close(kw.stopCh)
}

// watchPolicyReports watches for PolicyReport resources
func (kw *KyvernoWatcher) watchPolicyReports(ctx context.Context) {
	gvr := schema.GroupVersionResource{
		Group:    "wgpolicyk8s.io",
		Version:  "v1alpha2",
		Resource: "policyreports",
	}

	watcher, err := kw.dynamicClient.Resource(gvr).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("❌ [KYVERNO-WATCHER] Failed to watch PolicyReports: %v", err)
		return
	}
	defer watcher.Stop()

	log.Printf("👀 [KYVERNO-WATCHER] Watching PolicyReports...")

	for {
		select {
		case <-kw.stopCh:
			log.Printf("🛑 [KYVERNO-WATCHER] PolicyReport watcher stopped")
			return
		case event := <-watcher.ResultChan():
			if event.Object == nil {
				continue
			}

			unstructuredObj := event.Object.(*unstructured.Unstructured)
			kw.processPolicyReport(unstructuredObj, string(event.Type))
		}
	}
}

// watchClusterPolicyReports watches for ClusterPolicyReport resources
func (kw *KyvernoWatcher) watchClusterPolicyReports(ctx context.Context) {
	gvr := schema.GroupVersionResource{
		Group:    "wgpolicyk8s.io",
		Version:  "v1alpha2",
		Resource: "clusterpolicyreports",
	}

	watcher, err := kw.dynamicClient.Resource(gvr).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("❌ [KYVERNO-WATCHER] Failed to watch ClusterPolicyReports: %v", err)
		return
	}
	defer watcher.Stop()

	log.Printf("👀 [KYVERNO-WATCHER] Watching ClusterPolicyReports...")

	for {
		select {
		case <-kw.stopCh:
			log.Printf("🛑 [KYVERNO-WATCHER] ClusterPolicyReport watcher stopped")
			return
		case event := <-watcher.ResultChan():
			if event.Object == nil {
				continue
			}

			unstructuredObj := event.Object.(*unstructured.Unstructured)
			kw.processClusterPolicyReport(unstructuredObj, string(event.Type))
		}
	}
}

// watchKyvernoPolicies watches for Kyverno policy resources
func (kw *KyvernoWatcher) watchKyvernoPolicies(ctx context.Context) {
	// Watch ClusterPolicies
	clusterPolicyGVR := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}

	watcher, err := kw.dynamicClient.Resource(clusterPolicyGVR).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("❌ [KYVERNO-WATCHER] Failed to watch ClusterPolicies: %v", err)
		return
	}
	defer watcher.Stop()

	log.Printf("👀 [KYVERNO-WATCHER] Watching ClusterPolicies...")

	for {
		select {
		case <-kw.stopCh:
			log.Printf("🛑 [KYVERNO-WATCHER] ClusterPolicy watcher stopped")
			return
		case event := <-watcher.ResultChan():
			if event.Object == nil {
				continue
			}

			unstructuredObj := event.Object.(*unstructured.Unstructured)
			kw.processKyvernoPolicy(unstructuredObj, string(event.Type), "ClusterPolicy")
		}
	}
}

// processPolicyReport processes PolicyReport events
func (kw *KyvernoWatcher) processPolicyReport(obj *unstructured.Unstructured, eventType string) {
	metadata := obj.Object["metadata"].(map[string]interface{})
	name := metadata["name"].(string)
	namespace := metadata["namespace"].(string)

	log.Printf("📊 [KYVERNO-WATCHER] PolicyReport %s/%s event: %s", namespace, name, eventType)

	// Extract policy violations from the report
	if results, ok := obj.Object["results"].([]interface{}); ok {
		for _, result := range results {
			resultMap := result.(map[string]interface{})
			violation := kw.extractViolationFromResult(resultMap, namespace)
			if violation != nil {
				kw.actionHandler.HandleKyvernoPolicyViolation(context.Background(), violation)
			}
		}
	}
}

// processClusterPolicyReport processes ClusterPolicyReport events
func (kw *KyvernoWatcher) processClusterPolicyReport(obj *unstructured.Unstructured, eventType string) {
	metadata := obj.Object["metadata"].(map[string]interface{})
	name := metadata["name"].(string)

	log.Printf("📊 [KYVERNO-WATCHER] ClusterPolicyReport %s event: %s", name, eventType)

	// Extract policy violations from the report
	if results, ok := obj.Object["results"].([]interface{}); ok {
		for _, result := range results {
			resultMap := result.(map[string]interface{})
			violation := kw.extractViolationFromResult(resultMap, "")
			if violation != nil {
				kw.actionHandler.HandleKyvernoPolicyViolation(context.Background(), violation)
			}
		}
	}
}

// processKyvernoPolicy processes Kyverno policy events
func (kw *KyvernoWatcher) processKyvernoPolicy(obj *unstructured.Unstructured, eventType, policyType string) {
	metadata := obj.Object["metadata"].(map[string]interface{})
	name := metadata["name"].(string)

	log.Printf("📋 [KYVERNO-WATCHER] %s %s event: %s", policyType, name, eventType)

	// Create policy event
	event := &KyvernoEvent{
		EventType:  eventType,
		PolicyName: name,
		PolicyType: policyType,
		Namespace:  "",
		Message:    fmt.Sprintf("%s %s %s", policyType, name, eventType),
		Timestamp:  time.Now(),
		Details: map[string]string{
			"policyType": policyType,
			"eventType":  eventType,
		},
	}

	kw.actionHandler.HandleKyvernoPolicyEvent(context.Background(), event)
}

// extractViolationFromResult extracts violation information from a policy result
func (kw *KyvernoWatcher) extractViolationFromResult(result map[string]interface{}, namespace string) *KyvernoViolation {
	policy, ok := result["policy"].(string)
	if !ok {
		return nil
	}

	rule, _ := result["rule"].(string)
	resultType, _ := result["result"].(string)
	message, _ := result["message"].(string)

	// Safely extract resource information
	var resourceKind, resourceName string
	if resource, ok := result["resource"].(map[string]interface{}); ok && resource != nil {
		resourceKind, _ = resource["kind"].(string)
		resourceName, _ = resource["name"].(string)
	}

	// Map Kyverno result types to our violation types
	violationType := kw.mapResultToViolationType(resultType)

	return &KyvernoViolation{
		PolicyName:    policy,
		PolicyType:    "validate", // Most common type
		ResourceKind:  resourceKind,
		ResourceName:  resourceName,
		Namespace:     namespace,
		ViolationType: violationType,
		Message:       message,
		Timestamp:     time.Now(),
		Details: map[string]string{
			"rule":   rule,
			"result": resultType,
		},
	}
}

// mapResultToViolationType maps Kyverno result types to violation types
func (kw *KyvernoWatcher) mapResultToViolationType(result string) string {
	switch result {
	case "fail", "error":
		return "blocked"
	case "warn":
		return "warning"
	case "pass":
		return "passed"
	default:
		return "unknown"
	}
}
