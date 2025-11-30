package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
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
	startTime := time.Now()
	defer func() {
		if ep.eventProcessingDuration != nil {
			ep.eventProcessingDuration.WithLabelValues("kyverno", "informer").Observe(time.Since(startTime).Seconds())
		}
	}()
	results, found, _ := unstructured.NestedSlice(report.Object, "results")
	logger.Debug("Processing Kyverno PolicyReport",
		logger.Fields{
			Component:    "watcher",
			Operation:    "process_kyverno_report",
			Source:       "kyverno",
			ResourceKind: "PolicyReport",
			Namespace:    report.GetNamespace(),
			ResourceName: report.GetName(),
			Count:        len(results),
			Additional: map[string]interface{}{
				"results_found": found,
			},
		})
	if !found || len(results) == 0 {
		logger.Debug("PolicyReport has no results",
			logger.Fields{
				Component:    "watcher",
				Operation:    "process_kyverno_report",
				Source:       "kyverno",
				ResourceKind: "PolicyReport",
				Namespace:    report.GetNamespace(),
				ResourceName: report.GetName(),
			})
		return
	}

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
			logger.Debug("Skipping result - missing resource info",
				logger.Fields{
					Component:    "watcher",
					Operation:    "process_kyverno_report",
					Source:       "kyverno",
					ResourceKind: resourceKind,
					ResourceName: resourceName,
					Reason:       "missing_resource_info",
				})
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
			logger.Warn("Failed to create Kyverno Observation",
				logger.Fields{
					Component:    "watcher",
					Operation:    "observation_create",
					Source:       "kyverno",
					ResourceKind: resourceKind,
					ResourceName: resourceName,
					Namespace:    resourceNs,
					Error:        err,
				})
		} else {
			ep.mu.Lock()
			ep.totalCount++
			ep.mu.Unlock()
			count++
		}
	}

	if count > 0 {
		logger.Info("Created Observations from Kyverno policy violations",
			logger.Fields{
				Component: "watcher",
				Operation: "process_kyverno_report",
				Source:    "kyverno",
				Count:     count,
			})
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
		logger.Debug("No vulnerabilities found in report",
			logger.Fields{
				Component:    "watcher",
				Operation:    "process_trivy_report",
				Source:       "trivy",
				ResourceKind: "VulnerabilityReport",
				Namespace:    report.GetNamespace(),
				ResourceName: report.GetName(),
			})
		return
	}
	logger.Debug("Found vulnerabilities in report",
		logger.Fields{
			Component:    "watcher",
			Operation:    "process_trivy_report",
			Source:       "trivy",
			ResourceKind: "VulnerabilityReport",
			Namespace:    report.GetNamespace(),
			ResourceName: report.GetName(),
			Count:        len(vulnerabilities),
		})

	resourceKind := report.GetLabels()["trivy-operator.resource.kind"]
	resourceName := report.GetLabels()["trivy-operator.resource.name"]

	if resourceKind == "" || resourceName == "" {
		logger.Warn("Missing resource labels in VulnerabilityReport",
			logger.Fields{
				Component:    "watcher",
				Operation:    "process_trivy_report",
				Source:       "trivy",
				ResourceKind: resourceKind,
				ResourceName: resourceName,
				Namespace:    report.GetNamespace(),
				Reason:       "missing_labels",
			})
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
			logger.Warn("Failed to create Trivy Observation",
				logger.Fields{
					Component:    "watcher",
					Operation:    "observation_create",
					Source:       "trivy",
					ResourceKind: resourceKind,
					ResourceName: resourceName,
					Namespace:    report.GetNamespace(),
					Error:        err,
				})
		} else {
			ep.mu.Lock()
			ep.totalCount++
			ep.mu.Unlock()
			count++
		}
	}

	logger.Info("Processed Trivy VulnerabilityReport",
		logger.Fields{
			Component: "watcher",
			Operation: "process_trivy_report",
			Source:    "trivy",
			Count:     count,
			Additional: map[string]interface{}{
				"total_vulnerabilities": len(vulnerabilities),
				"high_critical_count":   highCriticalCount,
				"skipped_low":           skippedLow,
				"observations_created":  count,
			},
		})
	if count == 0 && highCriticalCount > 0 {
		logger.Warn("Found HIGH/CRITICAL vulnerabilities but created 0 observations (all duplicates?)",
			logger.Fields{
				Component: "watcher",
				Operation: "process_trivy_report",
				Source:    "trivy",
				Reason:    "all_duplicates",
				Count:     highCriticalCount,
			})
	}
}

// GetTotalCount returns the total number of events created
func (ep *EventProcessor) GetTotalCount() int64 {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	return ep.totalCount
}
