package watcher

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	log.Printf("  🔍 [KYVERNO] ProcessKyvernoPolicyReport called for: %s/%s", report.GetNamespace(), report.GetName())
	startTime := time.Now()
	defer func() {
		if ep.eventProcessingDuration != nil {
			ep.eventProcessingDuration.WithLabelValues("kyverno", "informer").Observe(time.Since(startTime).Seconds())
		}
	}()
	results, found, _ := unstructured.NestedSlice(report.Object, "results")
	log.Printf("  🔍 [KYVERNO] Results extraction: found=%v, count=%d", found, len(results))
	if !found || len(results) == 0 {
		log.Printf("  ⚠️  [KYVERNO] PolicyReport %s/%s has no results", report.GetNamespace(), report.GetName())
		return
	}
	log.Printf("  📋 [KYVERNO] Processing PolicyReport %s/%s with %d results", report.GetNamespace(), report.GetName(), len(results))

	// Try to get scope (for scoped PolicyReports)
	scope, scopeFound, _ := unstructured.NestedMap(report.Object, "scope")

	// Default resource info from report namespace
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

		// Skip if we still don't have resource info
		if resourceKind == "" || resourceName == "" {
			log.Printf("  ⚠️  [KYVERNO] Skipping result - missing resource info (kind=%s, name=%s)", resourceKind, resourceName)
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
		err := ep.observationCreator.CreateObservation(ctx, event)
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
	log.Printf("🔍 [TRIVY] ProcessTrivyVulnerabilityReport called for: %s/%s", report.GetNamespace(), report.GetName())
	startTime := time.Now()
	defer func() {
		if ep.eventProcessingDuration != nil {
			ep.eventProcessingDuration.WithLabelValues("trivy", "informer").Observe(time.Since(startTime).Seconds())
		}
	}()
	vulnerabilities, found, _ := unstructured.NestedSlice(report.Object, "report", "vulnerabilities")
	if !found || len(vulnerabilities) == 0 {
		log.Printf("  ⚠️  [TRIVY] No vulnerabilities found in report %s/%s", report.GetNamespace(), report.GetName())
		return
	}
	log.Printf("  📋 [TRIVY] Found %d vulnerabilities in report %s/%s", len(vulnerabilities), report.GetNamespace(), report.GetName())

	resourceKind := report.GetLabels()["trivy-operator.resource.kind"]
	resourceName := report.GetLabels()["trivy-operator.resource.name"]

	if resourceKind == "" || resourceName == "" {
		log.Printf("  ⚠️  [TRIVY] Missing resource labels: kind=%s, name=%s", resourceKind, resourceName)
		return
	}

	count := 0
	highCriticalCount := 0
	skippedLow := 0
	for _, v := range vulnerabilities {
		vuln, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		severity := vuln["severity"]
		severityStr := fmt.Sprintf("%v", severity)
		if severityStr != "HIGH" && severityStr != "CRITICAL" {
			skippedLow++
			continue
		}
		highCriticalCount++

		vulnID := fmt.Sprintf("%v", vuln["vulnerabilityID"])

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
		err := ep.observationCreator.CreateObservation(ctx, event)
		if err != nil {
			log.Printf("  ⚠️  Failed to create Trivy Observation: %v", err)
		} else {
			ep.mu.Lock()
			ep.totalCount++
			ep.mu.Unlock()
			count++
		}
	}

	log.Printf("  📊 [TRIVY] Summary: %d total vulnerabilities, %d HIGH/CRITICAL, %d skipped (low severity), %d observations created", len(vulnerabilities), highCriticalCount, skippedLow, count)
	if highCriticalCount > 0 {
		log.Printf("  📊 [TRIVY] Processed %d HIGH/CRITICAL vulnerabilities, created %d observations", highCriticalCount, count)
	}
	if count > 0 {
		log.Printf("  ✅ Created %d NEW Observations from Trivy vulnerabilities", count)
	} else if highCriticalCount > 0 {
		log.Printf("  ⚠️  [TRIVY] Found %d HIGH/CRITICAL vulnerabilities but created 0 observations (all duplicates?)", highCriticalCount)
	}
}

// GetTotalCount returns the total number of events created
func (ep *EventProcessor) GetTotalCount() int64 {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	return ep.totalCount
}
