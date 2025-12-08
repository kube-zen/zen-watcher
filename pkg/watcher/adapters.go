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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// normalizeSeverity normalizes severity strings to uppercase standard levels
func normalizeSeverity(severity string) string {
	upper := strings.ToUpper(severity)
	switch upper {
	case "CRITICAL", "FATAL", "EMERGENCY":
		return "CRITICAL"
	case "HIGH", "ERROR", "ALERT":
		return "HIGH"
	case "MEDIUM", "WARNING", "WARN":
		return "MEDIUM"
	case "LOW", "INFO", "INFORMATIONAL":
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// TrivyAdapter implements SourceAdapter for Trivy VulnerabilityReports
type TrivyAdapter struct {
	factory  dynamicinformer.DynamicSharedInformerFactory
	trivyGVR schema.GroupVersionResource
	informer cache.SharedIndexInformer
	stopCh   chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewTrivyAdapter creates a new Trivy adapter
func NewTrivyAdapter(
	factory dynamicinformer.DynamicSharedInformerFactory,
	trivyGVR schema.GroupVersionResource,
) *TrivyAdapter {
	return &TrivyAdapter{
		factory:  factory,
		trivyGVR: trivyGVR,
		stopCh:   make(chan struct{}),
	}
}

func (a *TrivyAdapter) Name() string {
	return "trivy"
}

func (a *TrivyAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Get informer for Trivy VulnerabilityReports
	a.informer = a.factory.ForResource(a.trivyGVR).Informer()

	// Add event handlers
	a.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			report, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			events := a.processReport(report)
			for _, event := range events {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			report, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			events := a.processReport(report)
			for _, event := range events {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return
				}
			}
		},
	})

	// Start informer
	a.factory.Start(a.stopCh)

	// Wait for cache sync
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	if !cache.WaitForCacheSync(ctx.Done(), a.informer.HasSynced) {
		logger.Debug("Trivy informer cache did not sync within timeout (CRD may not be installed)",
			logger.Fields{
				Component: "watcher",
				Operation: "trivy_adapter_sync",
				Source:    "trivy",
			})
	}

	// Block until context cancelled
	<-a.ctx.Done()
	return a.ctx.Err()
}

func (a *TrivyAdapter) processReport(report *unstructured.Unstructured) []*Event {
	vulnerabilities, found, _ := unstructured.NestedSlice(report.Object, "report", "vulnerabilities")
	if !found || len(vulnerabilities) == 0 {
		return nil
	}

	resourceKind := report.GetLabels()["trivy-operator.resource.kind"]
	resourceName := report.GetLabels()["trivy-operator.resource.name"]
	if resourceKind == "" || resourceName == "" {
		return nil
	}

	var events []*Event
	for _, v := range vulnerabilities {
		vuln, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		severity := normalizeSeverity(fmt.Sprintf("%v", vuln["severity"]))

		event := &Event{
			Source:    "trivy",
			Category:  "security",
			Severity:  severity,
			EventType: "vulnerability",
			Resource: &ResourceRef{
				Kind:      resourceKind,
				Name:      resourceName,
				Namespace: report.GetNamespace(),
			},
			Namespace:  report.GetNamespace(),
			DetectedAt: time.Now().Format(time.RFC3339),
			Details: map[string]interface{}{
				"vulnerabilityID":  vuln["vulnerabilityID"],
				"title":            vuln["title"],
				"description":      vuln["description"],
				"score":            vuln["score"],
				"fixedVersion":     vuln["fixedVersion"],
				"installedVersion": vuln["installedVersion"],
			},
		}

		events = append(events, event)
	}

	return events
}

func (a *TrivyAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	close(a.stopCh)
}

// KyvernoAdapter implements SourceAdapter for Kyverno PolicyReports
type KyvernoAdapter struct {
	factory         dynamicinformer.DynamicSharedInformerFactory
	policyReportGVR schema.GroupVersionResource
	informer        cache.SharedIndexInformer
	stopCh          chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewKyvernoAdapter creates a new Kyverno adapter
func NewKyvernoAdapter(
	factory dynamicinformer.DynamicSharedInformerFactory,
	policyReportGVR schema.GroupVersionResource,
) *KyvernoAdapter {
	return &KyvernoAdapter{
		factory:         factory,
		policyReportGVR: policyReportGVR,
		stopCh:          make(chan struct{}),
	}
}

func (a *KyvernoAdapter) Name() string {
	return "kyverno"
}

func (a *KyvernoAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Get informer for Kyverno PolicyReports
	a.informer = a.factory.ForResource(a.policyReportGVR).Informer()

	// Add event handlers
	a.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			report, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			events := a.processReport(report)
			for _, event := range events {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			report, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			events := a.processReport(report)
			for _, event := range events {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return
				}
			}
		},
	})

	// Start informer
	a.factory.Start(a.stopCh)

	// Wait for cache sync
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	if !cache.WaitForCacheSync(ctx.Done(), a.informer.HasSynced) {
		logger.Debug("Kyverno informer cache did not sync within timeout (CRD may not be installed)",
			logger.Fields{
				Component: "watcher",
				Operation: "kyverno_adapter_sync",
				Source:    "kyverno",
			})
	}

	// Block until context cancelled
	<-a.ctx.Done()
	return a.ctx.Err()
}

func (a *KyvernoAdapter) processReport(report *unstructured.Unstructured) []*Event {
	results, found, _ := unstructured.NestedSlice(report.Object, "results")
	if !found || len(results) == 0 {
		return nil
	}

	// Try to get scope
	scope, scopeFound, _ := unstructured.NestedMap(report.Object, "scope")

	resourceNs := report.GetNamespace()
	resourceKind := ""
	resourceName := ""

	if scopeFound && scope != nil {
		resourceKind = fmt.Sprintf("%v", scope["kind"])
		resourceName = fmt.Sprintf("%v", scope["name"])
		if ns := fmt.Sprintf("%v", scope["namespace"]); ns != "" && ns != "<nil>" {
			resourceNs = ns
		}
	}

	var events []*Event
	for _, r := range results {
		result, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		resultStatus := fmt.Sprintf("%v", result["result"])
		if resultStatus != "fail" {
			continue
		}

		policy := fmt.Sprintf("%v", result["policy"])
		rule := fmt.Sprintf("%v", result["rule"])
		severity := normalizeSeverity(fmt.Sprintf("%v", result["severity"]))
		message := fmt.Sprintf("%v", result["message"])

		// Try to get resource info from result.resources if scope not available
		if resourceKind == "" || resourceName == "" {
			if resources, found, _ := unstructured.NestedSlice(result, "resources"); found && len(resources) > 0 {
				if res, ok := resources[0].(map[string]interface{}); ok {
					if k := fmt.Sprintf("%v", res["kind"]); k != "" && k != "<nil>" {
						resourceKind = k
					}
					if n := fmt.Sprintf("%v", res["name"]); n != "" && n != "<nil>" {
						resourceName = n
					}
					if ns := fmt.Sprintf("%v", res["namespace"]); ns != "" && ns != "<nil>" {
						resourceNs = ns
					}
				}
			}
		}

		if resourceKind == "" || resourceName == "" {
			continue
		}

		event := &Event{
			Source:    "kyverno",
			Category:  "security",
			Severity:  severity,
			EventType: "policy-violation",
			Resource: &ResourceRef{
				Kind:      resourceKind,
				Name:      resourceName,
				Namespace: resourceNs,
			},
			Namespace:  resourceNs,
			DetectedAt: time.Now().Format(time.RFC3339),
			Details: map[string]interface{}{
				"policy":  policy,
				"rule":    rule,
				"message": message,
				"result":  resultStatus,
			},
		}

		events = append(events, event)
	}

	return events
}

func (a *KyvernoAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	close(a.stopCh)
}

// FalcoAdapter implements SourceAdapter for Falco webhooks
// Reads from a channel populated by HTTP webhook handlers
type FalcoAdapter struct {
	alertChan chan map[string]interface{}
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
}

// NewFalcoAdapter creates a new Falco adapter that reads from the alert channel
func NewFalcoAdapter(alertChan chan map[string]interface{}) *FalcoAdapter {
	return &FalcoAdapter{
		alertChan: alertChan,
	}
}

func (a *FalcoAdapter) Name() string {
	return "falco"
}

func (a *FalcoAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	for {
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case alert := <-a.alertChan:
			event := a.processAlert(alert)
			if event != nil {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return a.ctx.Err()
				}
			}
		}
	}
}

func (a *FalcoAdapter) processAlert(alert map[string]interface{}) *Event {
	priority := fmt.Sprintf("%v", alert["priority"])
	rule := fmt.Sprintf("%v", alert["rule"])
	output := fmt.Sprintf("%v", alert["output"])

	// Only process Warning, Error, Critical, Alert, Emergency
	if priority != "Warning" && priority != "Error" && priority != "Critical" && priority != "Alert" && priority != "Emergency" {
		return nil
	}

	// Get K8s context if present
	var k8sPodName, k8sNs string
	if outputFields, ok := alert["output_fields"].(map[string]interface{}); ok {
		k8sPodName = fmt.Sprintf("%v", outputFields["k8s.pod.name"])
		k8sNs = fmt.Sprintf("%v", outputFields["k8s.ns.name"])
	}
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

	return &Event{
		Source:    "falco",
		Category:  "security",
		Severity:  severity,
		EventType: "runtime-security",
		Resource: &ResourceRef{
			Kind:      "Pod",
			Name:      k8sPodName,
			Namespace: k8sNs,
		},
		Namespace:  k8sNs,
		DetectedAt: time.Now().Format(time.RFC3339),
		Details: map[string]interface{}{
			"rule":         rule,
			"priority":     priority,
			"output":       output,
			"k8s_pod_name": k8sPodName,
			"k8s_ns_name":  k8sNs,
		},
	}
}

func (a *FalcoAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
}

// AuditAdapter implements SourceAdapter for Kubernetes audit webhooks
// Reads from a channel populated by HTTP webhook handlers
type AuditAdapter struct {
	eventChan chan map[string]interface{}
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
}

// NewAuditAdapter creates a new Audit adapter that reads from the event channel
func NewAuditAdapter(eventChan chan map[string]interface{}) *AuditAdapter {
	return &AuditAdapter{
		eventChan: eventChan,
	}
}

func (a *AuditAdapter) Name() string {
	return "audit"
}

func (a *AuditAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	for {
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case auditEvent := <-a.eventChan:
			event := a.processEvent(auditEvent)
			if event != nil {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return a.ctx.Err()
				}
			}
		}
	}
}

func (a *AuditAdapter) processEvent(auditEvent map[string]interface{}) *Event {
	stage := fmt.Sprintf("%v", auditEvent["stage"])

	// Only process ResponseComplete stage
	if stage != "ResponseComplete" {
		return nil
	}

	objectRef, ok := auditEvent["objectRef"].(map[string]interface{})
	if !ok {
		objectRef = make(map[string]interface{})
	}
	resource := fmt.Sprintf("%v", objectRef["resource"])
	namespace := fmt.Sprintf("%v", objectRef["namespace"])
	name := fmt.Sprintf("%v", objectRef["name"])
	apiGroup := fmt.Sprintf("%v", objectRef["apiGroup"])
	verb := fmt.Sprintf("%v", auditEvent["verb"])

	// Filter for important actions
	important := false
	category := "compliance"
	severity := "MEDIUM"
	eventType := "audit-event"

	if verb == "delete" {
		important = true
		severity = "HIGH"
		eventType = "resource-deletion"
	}

	if resource == "secrets" || resource == "configmaps" {
		if verb == "create" || verb == "update" || verb == "patch" || verb == "delete" {
			important = true
			severity = "HIGH"
			eventType = "secret-access"
		}
	}

	if apiGroup == "rbac.authorization.k8s.io" {
		if verb == "create" || verb == "update" || verb == "patch" || verb == "delete" {
			important = true
			severity = "HIGH"
			eventType = "rbac-change"
		}
	}

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

	if namespace == "<nil>" || namespace == "" {
		namespace = "default"
	}

	user, ok := auditEvent["user"].(map[string]interface{})
	if !ok {
		user = make(map[string]interface{})
	}
	username := fmt.Sprintf("%v", user["username"])

	responseStatus, ok := auditEvent["responseStatus"].(map[string]interface{})
	if !ok {
		responseStatus = make(map[string]interface{})
	}
	statusCode := fmt.Sprintf("%v", responseStatus["code"])

	return &Event{
		Source:    "audit",
		Category:  category,
		Severity:  severity,
		EventType: eventType,
		Resource: &ResourceRef{
			Kind:      resource,
			Name:      name,
			Namespace: namespace,
		},
		Namespace:  namespace,
		DetectedAt: time.Now().Format(time.RFC3339),
		Details: map[string]interface{}{
			"verb":       verb,
			"auditID":    fmt.Sprintf("%v", auditEvent["auditID"]),
			"username":   username,
			"statusCode": statusCode,
			"apiGroup":   apiGroup,
		},
	}
}

func (a *AuditAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
}

// KubeBenchAdapter implements SourceAdapter for kube-bench ConfigMap polling
type KubeBenchAdapter struct {
	clientSet kubernetes.Interface
	namespace string
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	ticker    *time.Ticker
}

// NewKubeBenchAdapter creates a new kube-bench adapter
func NewKubeBenchAdapter(clientSet kubernetes.Interface) *KubeBenchAdapter {
	namespace := os.Getenv("KUBE_BENCH_NAMESPACE")
	if namespace == "" {
		namespace = "kube-bench"
	}

	return &KubeBenchAdapter{
		clientSet: clientSet,
		namespace: namespace,
		interval:  5 * time.Minute,
	}
}

func (a *KubeBenchAdapter) Name() string {
	return "kubebench"
}

func (a *KubeBenchAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.ticker = time.NewTicker(a.interval)
	defer a.ticker.Stop()

	// Initial poll
	events := a.poll()
	for _, event := range events {
		select {
		case out <- event:
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}

	for {
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case <-a.ticker.C:
			events := a.poll()
			for _, event := range events {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return a.ctx.Err()
				}
			}
		}
	}
}

func (a *KubeBenchAdapter) poll() []*Event {
	configMaps, err := a.clientSet.CoreV1().ConfigMaps(a.namespace).List(a.ctx, metav1.ListOptions{
		LabelSelector: "app=kube-bench",
	})
	if err != nil || len(configMaps.Items) == 0 {
		return nil
	}

	var events []*Event
	for _, cm := range configMaps.Items {
		resultsJSON, found := cm.Data["results.json"]
		if !found {
			continue
		}

		var benchResults map[string]interface{}
		if err := json.Unmarshal([]byte(resultsJSON), &benchResults); err != nil {
			continue
		}

		controls, ok := benchResults["Controls"].([]interface{})
		if !ok {
			continue
		}

		for _, c := range controls {
			control, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			tests, ok := control["tests"].([]interface{})
			if !ok {
				continue
			}

			for _, t := range tests {
				test, ok := t.(map[string]interface{})
				if !ok {
					continue
				}
				results, ok := test["results"].([]interface{})
				if !ok {
					continue
				}
				section := fmt.Sprintf("%v", test["section"])

				for _, r := range results {
					result, ok := r.(map[string]interface{})
					if !ok {
						continue
					}
					status := fmt.Sprintf("%v", result["status"])

					if status != "FAIL" {
						continue
					}

					testNumber := fmt.Sprintf("%v", result["test_number"])
					testDesc := fmt.Sprintf("%v", result["test_desc"])
					remediation := fmt.Sprintf("%v", result["remediation"])
					scored := result["scored"] == true

					severity := "MEDIUM"
					if scored {
						severity = "HIGH"
					}

					event := &Event{
						Source:    "kubebench",
						Category:  "compliance",
						Severity:  severity,
						EventType: "cis-benchmark-fail",
						Resource: &ResourceRef{
							Kind: "Node",
							Name: "k3d-zen-agent-server-0", // Default, could be extracted from ConfigMap
						},
						Namespace:  a.namespace,
						DetectedAt: time.Now().Format(time.RFC3339),
						Details: map[string]interface{}{
							"testNumber":  testNumber,
							"section":     section,
							"testDesc":    testDesc,
							"remediation": remediation,
							"scored":      scored,
						},
					}

					events = append(events, event)
				}
			}
		}
	}

	return events
}

func (a *KubeBenchAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.ticker != nil {
		a.ticker.Stop()
	}
}

// CheckovAdapter implements SourceAdapter for Checkov ConfigMap polling
type CheckovAdapter struct {
	clientSet kubernetes.Interface
	namespace string
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	ticker    *time.Ticker
}

// NewCheckovAdapter creates a new Checkov adapter
func NewCheckovAdapter(clientSet kubernetes.Interface) *CheckovAdapter {
	namespace := os.Getenv("CHECKOV_NAMESPACE")
	if namespace == "" {
		namespace = "checkov"
	}

	return &CheckovAdapter{
		clientSet: clientSet,
		namespace: namespace,
		interval:  5 * time.Minute,
	}
}

func (a *CheckovAdapter) Name() string {
	return "checkov"
}

func (a *CheckovAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.ticker = time.NewTicker(a.interval)
	defer a.ticker.Stop()

	// Initial poll
	events := a.poll()
	for _, event := range events {
		select {
		case out <- event:
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}

	for {
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case <-a.ticker.C:
			events := a.poll()
			for _, event := range events {
				select {
				case out <- event:
				case <-a.ctx.Done():
					return a.ctx.Err()
				}
			}
		}
	}
}

func (a *CheckovAdapter) poll() []*Event {
	checkovCMs, err := a.clientSet.CoreV1().ConfigMaps(a.namespace).List(a.ctx, metav1.ListOptions{
		LabelSelector: "app=checkov",
	})
	if err != nil || len(checkovCMs.Items) == 0 {
		return nil
	}

	var events []*Event
	for _, cm := range checkovCMs.Items {
		resultsJSON, found := cm.Data["results.json"]
		if !found {
			continue
		}

		var checkovResults map[string]interface{}
		if err := json.Unmarshal([]byte(resultsJSON), &checkovResults); err != nil {
			continue
		}

		results, ok := checkovResults["results"].(map[string]interface{})
		if !ok {
			continue
		}

		failedChecks, ok := results["failed_checks"].([]interface{})
		if !ok {
			continue
		}

		for _, fc := range failedChecks {
			failedCheck, ok := fc.(map[string]interface{})
			if !ok {
				continue
			}

			checkID := fmt.Sprintf("%v", failedCheck["check_id"])
			checkName := fmt.Sprintf("%v", failedCheck["check_name"])
			resource := fmt.Sprintf("%v", failedCheck["resource"])
			guideline := fmt.Sprintf("%v", failedCheck["guideline"])

			// Parse resource (format: "Kind.namespace.name")
			resourceParts := strings.SplitN(resource, ".", 3)
			resourceKind := "Unknown"
			resourceNs := a.namespace
			resourceName := resource
			if len(resourceParts) >= 3 {
				resourceKind = resourceParts[0]
				resourceNs = resourceParts[1]
				resourceName = resourceParts[2]
			} else if len(resourceParts) == 2 {
				resourceKind = resourceParts[0]
				resourceName = resourceParts[1]
			} else if len(resourceParts) == 1 {
				resourceName = resourceParts[0]
			}

			category := "security"
			severity := "MEDIUM"
			if strings.HasPrefix(checkID, "CKV_K8S") {
				category = "security"
				if checkID == "CKV_K8S_20" || checkID == "CKV_K8S_23" || checkID == "CKV_K8S_16" {
					severity = "HIGH"
				}
			}

			event := &Event{
				Source:    "checkov",
				Category:  category,
				Severity:  severity,
				EventType: "static-analysis",
				Resource: &ResourceRef{
					Kind:      resourceKind,
					Name:      resourceName,
					Namespace: resourceNs,
				},
				Namespace:  resourceNs,
				DetectedAt: time.Now().Format(time.RFC3339),
				Details: map[string]interface{}{
					"checkId":   checkID,
					"checkName": checkName,
					"resource":  resource,
					"guideline": guideline,
				},
			}

			events = append(events, event)
		}
	}

	return events
}

func (a *CheckovAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.ticker != nil {
		a.ticker.Stop()
	}
}
