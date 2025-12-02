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
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
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
	factory            dynamicinformer.DynamicSharedInformerFactory
	trivyGVR           schema.GroupVersionResource
	informer           cache.SharedIndexInformer
	stopCh             chan struct{}
	ctx                context.Context
	cancel             context.CancelFunc
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
	factory            dynamicinformer.DynamicSharedInformerFactory
	policyReportGVR    schema.GroupVersionResource
	informer           cache.SharedIndexInformer
	stopCh             chan struct{}
	ctx                context.Context
	cancel             context.CancelFunc
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

