package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
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
	logger.Info("Starting Kyverno policy monitoring",
		logger.Fields{
			Component: "watcher",
			Operation: "kyverno_watcher_start",
			Source:    "kyverno",
		})

	// Start watching PolicyReports (namespace-scoped)
	go kw.watchPolicyReports(ctx)

	// Start watching ClusterPolicyReports (cluster-scoped)
	go kw.watchClusterPolicyReports(ctx)

	// Start watching Kyverno policies
	go kw.watchKyvernoPolicies(ctx)

	logger.Info("Kyverno monitoring started successfully",
		logger.Fields{
			Component: "watcher",
			Operation: "kyverno_watcher_start",
			Source:    "kyverno",
		})
	return nil
}

// Stop stops the Kyverno watcher
func (kw *KyvernoWatcher) Stop() {
	logger.Info("Stopping Kyverno monitoring",
		logger.Fields{
			Component: "watcher",
			Operation: "kyverno_watcher_stop",
			Source:    "kyverno",
		})
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
		logger.Error("Failed to watch PolicyReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_policy_reports",
				Source:       "kyverno",
				ResourceKind: "PolicyReport",
				Error:        err,
			})
		return
	}
	defer watcher.Stop()

	logger.Info("Watching PolicyReports",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_policy_reports",
			Source:       "kyverno",
			ResourceKind: "PolicyReport",
		})

	for {
		select {
		case <-kw.stopCh:
			logger.Info("PolicyReport watcher stopped",
				logger.Fields{
					Component:    "watcher",
					Operation:    "watch_policy_reports",
					Source:       "kyverno",
					ResourceKind: "PolicyReport",
				})
			return
		case event := <-watcher.ResultChan():
			if event.Object == nil {
				continue
			}

			unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				logger.Warn("Unexpected object type in PolicyReport event",
					logger.Fields{
						Component:    "watcher",
						Operation:    "watch_policy_reports",
						Source:       "kyverno",
						ResourceKind: "PolicyReport",
						Additional: map[string]interface{}{
							"object_type": fmt.Sprintf("%T", event.Object),
						},
					})
				continue
			}
			kw.processPolicyReport(ctx, unstructuredObj, string(event.Type))
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
		logger.Error("Failed to watch ClusterPolicyReports",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_cluster_policy_reports",
				Source:       "kyverno",
				ResourceKind: "ClusterPolicyReport",
				Error:        err,
			})
		return
	}
	defer watcher.Stop()

	logger.Info("Watching ClusterPolicyReports",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_cluster_policy_reports",
			Source:       "kyverno",
			ResourceKind: "ClusterPolicyReport",
		})

	for {
		select {
		case <-kw.stopCh:
			logger.Info("ClusterPolicyReport watcher stopped",
				logger.Fields{
					Component:    "watcher",
					Operation:    "watch_cluster_policy_reports",
					Source:       "kyverno",
					ResourceKind: "ClusterPolicyReport",
				})
			return
		case event := <-watcher.ResultChan():
			if event.Object == nil {
				continue
			}

			unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				logger.Warn("Unexpected object type in ClusterPolicyReport event",
					logger.Fields{
						Component:    "watcher",
						Operation:    "watch_cluster_policy_reports",
						Source:       "kyverno",
						ResourceKind: "ClusterPolicyReport",
						Additional: map[string]interface{}{
							"object_type": fmt.Sprintf("%T", event.Object),
						},
					})
				continue
			}
			kw.processClusterPolicyReport(ctx, unstructuredObj, string(event.Type))
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
		logger.Error("Failed to watch ClusterPolicies",
			logger.Fields{
				Component:    "watcher",
				Operation:    "watch_cluster_policies",
				Source:       "kyverno",
				ResourceKind: "ClusterPolicy",
				Error:        err,
			})
		return
	}
	defer watcher.Stop()

	logger.Info("Watching ClusterPolicies",
		logger.Fields{
			Component:    "watcher",
			Operation:    "watch_cluster_policies",
			Source:       "kyverno",
			ResourceKind: "ClusterPolicy",
		})

	for {
		select {
		case <-kw.stopCh:
			logger.Info("ClusterPolicy watcher stopped",
				logger.Fields{
					Component:    "watcher",
					Operation:    "watch_cluster_policies",
					Source:       "kyverno",
					ResourceKind: "ClusterPolicy",
				})
			return
		case event := <-watcher.ResultChan():
			if event.Object == nil {
				continue
			}

			unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				logger.Warn("Unexpected object type in ClusterPolicy event",
					logger.Fields{
						Component:    "watcher",
						Operation:    "watch_cluster_policies",
						Source:       "kyverno",
						ResourceKind: "ClusterPolicy",
						Additional: map[string]interface{}{
							"object_type": fmt.Sprintf("%T", event.Object),
						},
					})
				continue
			}
			kw.processKyvernoPolicy(ctx, unstructuredObj, string(event.Type), "ClusterPolicy")
		}
	}
}

// processPolicyReport processes PolicyReport events
func (kw *KyvernoWatcher) processPolicyReport(ctx context.Context, obj *unstructured.Unstructured, eventType string) {
	// Safely extract metadata using unstructured helpers
	name, _, _ := unstructured.NestedString(obj.Object, "metadata", "name")
	namespace, _, _ := unstructured.NestedString(obj.Object, "metadata", "namespace")

	if name == "" {
		logger.Warn("PolicyReport missing name, skipping",
			logger.Fields{
				Component:    "watcher",
				Operation:    "process_policy_report",
				Source:       "kyverno",
				ResourceKind: "PolicyReport",
				Reason:       "missing_name",
			})
		return
	}
	if namespace == "" {
		namespace = obj.GetNamespace()
	}

	logger.Debug("Processing PolicyReport event",
		logger.Fields{
			Component:    "watcher",
			Operation:    "process_policy_report",
			Source:       "kyverno",
			ResourceKind: "PolicyReport",
			ResourceName: name,
			Namespace:    namespace,
			EventType:    eventType,
		})

	// Extract policy violations from the report
	results, found, _ := unstructured.NestedSlice(obj.Object, "results")
	if !found {
		return
	}

	for _, result := range results {
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			logger.Warn("Invalid result type in PolicyReport",
				logger.Fields{
					Component:    "watcher",
					Operation:    "process_policy_report",
					Source:       "kyverno",
					ResourceKind: "PolicyReport",
					ResourceName: name,
					Namespace:    namespace,
					Reason:       "invalid_result_type",
				})
			continue
		}
		violation := kw.extractViolationFromResult(resultMap, namespace)
		if violation != nil {
			kw.actionHandler.HandleKyvernoPolicyViolation(ctx, violation)
		}
	}
}

// processClusterPolicyReport processes ClusterPolicyReport events
func (kw *KyvernoWatcher) processClusterPolicyReport(ctx context.Context, obj *unstructured.Unstructured, eventType string) {
	// Safely extract metadata using unstructured helpers
	name, _, _ := unstructured.NestedString(obj.Object, "metadata", "name")

	if name == "" {
		logger.Warn("ClusterPolicyReport missing name, skipping",
			logger.Fields{
				Component:    "watcher",
				Operation:    "process_cluster_policy_report",
				Source:       "kyverno",
				ResourceKind: "ClusterPolicyReport",
				Reason:       "missing_name",
			})
		return
	}

	logger.Debug("Processing ClusterPolicyReport event",
		logger.Fields{
			Component:    "watcher",
			Operation:    "process_cluster_policy_report",
			Source:       "kyverno",
			ResourceKind: "ClusterPolicyReport",
			ResourceName: name,
			EventType:    eventType,
		})

	// Extract policy violations from the report
	results, found, _ := unstructured.NestedSlice(obj.Object, "results")
	if !found {
		return
	}

	for _, result := range results {
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			logger.Warn("Invalid result type in ClusterPolicyReport",
				logger.Fields{
					Component:    "watcher",
					Operation:    "process_cluster_policy_report",
					Source:       "kyverno",
					ResourceKind: "ClusterPolicyReport",
					ResourceName: name,
					Reason:       "invalid_result_type",
				})
			continue
		}
		violation := kw.extractViolationFromResult(resultMap, "")
		if violation != nil {
			kw.actionHandler.HandleKyvernoPolicyViolation(ctx, violation)
		}
	}
}

// processKyvernoPolicy processes Kyverno policy events
func (kw *KyvernoWatcher) processKyvernoPolicy(ctx context.Context, obj *unstructured.Unstructured, eventType, policyType string) {
	// Safely extract metadata using unstructured helpers
	name, _, _ := unstructured.NestedString(obj.Object, "metadata", "name")

	if name == "" {
		logger.Warn("Policy missing name, skipping",
			logger.Fields{
				Component:    "watcher",
				Operation:    "process_kyverno_policy",
				Source:       "kyverno",
				ResourceKind: policyType,
				Reason:       "missing_name",
			})
		return
	}

	logger.Debug("Processing Kyverno policy event",
		logger.Fields{
			Component:    "watcher",
			Operation:    "process_kyverno_policy",
			Source:       "kyverno",
			ResourceKind: policyType,
			ResourceName: name,
			EventType:    eventType,
		})

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

	kw.actionHandler.HandleKyvernoPolicyEvent(ctx, event)
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
