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

// EventProcessor handles event creation with deduplication
type EventProcessor struct {
	dynClient               dynamic.Interface
	eventGVR                schema.GroupVersionResource
	mu                      sync.RWMutex
	eventsTotal             *prometheus.CounterVec
	eventProcessingDuration *prometheus.HistogramVec
	totalCount              int64
	observationCreator      *ObservationCreator
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(dynClient dynamic.Interface, eventGVR schema.GroupVersionResource, eventsTotal *prometheus.CounterVec, eventProcessingDuration *prometheus.HistogramVec, observationCreator *ObservationCreator) *EventProcessor {
	return &EventProcessor{
		dynClient:               dynClient,
		eventGVR:                eventGVR,
		eventsTotal:             eventsTotal,
		eventProcessingDuration: eventProcessingDuration,
		observationCreator:      observationCreator,
	}
}

// ProcessKyvernoPolicyReport processes a Kyverno PolicyReport
func (ep *EventProcessor) ProcessKyvernoPolicyReport(ctx context.Context, report *unstructured.Unstructured) {
	startTime := time.Now()
	defer func() {
		if ep.eventProcessingDuration != nil {
			ep.eventProcessingDuration.WithLabelValues("kyverno", "informer").Observe(time.Since(startTime).Seconds())
		}
	}()
	results, found, _ := unstructured.NestedSlice(report.Object, "results")
	if !found || len(results) == 0 {
		return
	}

	scope, scopeFound, _ := unstructured.NestedMap(report.Object, "scope")
	if !scopeFound {
		return
	}

	resourceKind := fmt.Sprintf("%v", scope["kind"])
	resourceName := fmt.Sprintf("%v", scope["name"])
	resourceNs := fmt.Sprintf("%v", scope["namespace"])
	if resourceNs == "" {
		resourceNs = report.GetNamespace()
	}

	count := 0
	for _, r := range results {
		result, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		resultStatus := fmt.Sprintf("%v", result["result"])

		// Only process failed policies
		if resultStatus != "fail" {
			continue
		}

		policy := fmt.Sprintf("%v", result["policy"])
		rule := fmt.Sprintf("%v", result["rule"])
		severity := fmt.Sprintf("%v", result["severity"])
		message := fmt.Sprintf("%v", result["message"])

		// Check if observation already exists in Kubernetes (same source AND same identifying fields)
		existingEvents, err := ep.dynClient.Resource(ep.eventGVR).Namespace(resourceNs).List(ctx, metav1.ListOptions{
			LabelSelector: "source=kyverno",
		})
		exists := false
		if err == nil {
			for _, ev := range existingEvents.Items {
				spec, ok := ev.Object["spec"].(map[string]interface{})
				if !ok {
					continue
				}
				evSource := fmt.Sprintf("%v", spec["source"])
				if evSource != "kyverno" {
					continue
				}
				details, ok := spec["details"].(map[string]interface{})
				if !ok {
					continue
				}
				evPolicy := fmt.Sprintf("%v", details["policy"])
				evRule := fmt.Sprintf("%v", details["rule"])
				evResource := spec["resource"].(map[string]interface{})
				if evResource == nil {
					continue
				}
				evNs := fmt.Sprintf("%v", evResource["namespace"])
				evKind := fmt.Sprintf("%v", evResource["kind"])
				evName := fmt.Sprintf("%v", evResource["name"])
				if evNs == resourceNs && evKind == resourceKind && evName == resourceName && evPolicy == policy && evRule == rule {
					exists = true
					break
				}
			}
		}

		if exists {
			continue
		}

		// Map severity to standard levels
		mappedSeverity := "MEDIUM"
		switch severity {
		case "high", "critical":
			mappedSeverity = "HIGH"
		case "low":
			mappedSeverity = "LOW"
		}

		event := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "zen.kube-zen.io/v1",
				"kind":       "Observation",
				"metadata": map[string]interface{}{
					"generateName": "kyverno-policy-",
					"namespace":    resourceNs,
					"labels": map[string]interface{}{
						"source":   "kyverno",
						"category": "security",
						"severity": mappedSeverity,
					},
				},
				"spec": map[string]interface{}{
					"source":     "kyverno",
					"category":   "security",
					"severity":   mappedSeverity,
					"eventType":  "policy-violation",
					"detectedAt": time.Now().Format(time.RFC3339),
					"resource": map[string]interface{}{
						"kind":      resourceKind,
						"name":      resourceName,
						"namespace": resourceNs,
					},
					"details": map[string]interface{}{
						"policy":  policy,
						"rule":    rule,
						"message": message,
						"result":  resultStatus,
					},
				},
			},
		}

		// Use centralized observation creator - metrics are incremented automatically
		err = ep.observationCreator.CreateObservation(ctx, event)
		if err != nil {
			log.Printf("  ⚠️  Failed to create Kyverno Observation: %v", err)
		} else {
			ep.mu.Lock()
			ep.totalCount++
			ep.mu.Unlock()
			count++
		}
	}

	if count > 0 {
		log.Printf("  ✅ Created %d NEW Observations from Kyverno policy violations", count)
	}
}

// ProcessTrivyVulnerabilityReport processes a Trivy VulnerabilityReport
func (ep *EventProcessor) ProcessTrivyVulnerabilityReport(ctx context.Context, report *unstructured.Unstructured) {
	startTime := time.Now()
	defer func() {
		if ep.eventProcessingDuration != nil {
			ep.eventProcessingDuration.WithLabelValues("trivy", "informer").Observe(time.Since(startTime).Seconds())
		}
	}()
	vulnerabilities, found, _ := unstructured.NestedSlice(report.Object, "report", "vulnerabilities")
	if !found || len(vulnerabilities) == 0 {
		return
	}

	resourceKind := report.GetLabels()["trivy-operator.resource.kind"]
	resourceName := report.GetLabels()["trivy-operator.resource.name"]

	count := 0
	for _, v := range vulnerabilities {
		vuln, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		severity := vuln["severity"]
		if severity != "HIGH" && severity != "CRITICAL" {
			continue
		}

		vulnID := fmt.Sprintf("%v", vuln["vulnerabilityID"])

		// Check if observation already exists in Kubernetes (same source AND same identifying fields)
		existingEvents, err := ep.dynClient.Resource(ep.eventGVR).Namespace(report.GetNamespace()).List(ctx, metav1.ListOptions{
			LabelSelector: "source=trivy",
		})
		exists := false
		if err == nil {
			for _, ev := range existingEvents.Items {
				spec, ok := ev.Object["spec"].(map[string]interface{})
				if !ok {
					continue
				}
				evSource := fmt.Sprintf("%v", spec["source"])
				if evSource != "trivy" {
					continue
				}
				details, ok := spec["details"].(map[string]interface{})
				if !ok {
					continue
				}
				evVulnID := fmt.Sprintf("%v", details["vulnerabilityID"])
				evResource := spec["resource"].(map[string]interface{})
				if evResource == nil {
					continue
				}
				evNs := fmt.Sprintf("%v", evResource["namespace"])
				evKind := fmt.Sprintf("%v", evResource["kind"])
				evName := fmt.Sprintf("%v", evResource["name"])
				if evNs == report.GetNamespace() && evKind == resourceKind && evName == resourceName && evVulnID == vulnID {
					exists = true
					break
				}
			}
		}

		if exists {
			continue
		}

		event := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "zen.kube-zen.io/v1",
				"kind":       "Observation",
				"metadata": map[string]interface{}{
					"generateName": "trivy-vuln-",
					"namespace":    report.GetNamespace(),
					"labels": map[string]interface{}{
						"source":   "trivy",
						"category": "security",
						"severity": fmt.Sprintf("%v", severity),
					},
				},
				"spec": map[string]interface{}{
					"source":     "trivy",
					"category":   "security",
					"severity":   fmt.Sprintf("%v", severity),
					"eventType":  "vulnerability",
					"detectedAt": time.Now().Format(time.RFC3339),
					"resource": map[string]interface{}{
						"kind":      resourceKind,
						"name":      resourceName,
						"namespace": report.GetNamespace(),
					},
					"details": map[string]interface{}{
						"vulnerabilityID":  vulnID,
						"title":            vuln["title"],
						"description":      vuln["description"],
						"score":            vuln["score"],
						"fixedVersion":     vuln["fixedVersion"],
						"installedVersion": vuln["installedVersion"],
					},
				},
			},
		}

		// Use centralized observation creator - metrics are incremented automatically
		err = ep.observationCreator.CreateObservation(ctx, event)
		if err != nil {
			log.Printf("  ⚠️  Failed to create Trivy Observation: %v", err)
		} else {
			ep.mu.Lock()
			ep.totalCount++
			ep.mu.Unlock()
			count++
		}
	}

	if count > 0 {
		log.Printf("  ✅ Created %d NEW Observations from Trivy vulnerabilities", count)
	}
}

// GetTotalCount returns the total number of events created
func (ep *EventProcessor) GetTotalCount() int64 {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	return ep.totalCount
}
