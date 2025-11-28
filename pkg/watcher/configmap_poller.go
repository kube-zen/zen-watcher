package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapPoller handles periodic polling of ConfigMaps for kube-bench and Checkov
type ConfigMapPoller struct {
	clientSet          kubernetes.Interface
	dynClient          dynamic.Interface
	eventGVR           schema.GroupVersionResource
	eventProcessor     *EventProcessor
	webhookProcessor   *WebhookProcessor
	interval           time.Duration
	eventsTotal        *prometheus.CounterVec
	observationCreator *ObservationCreator
}

// NewConfigMapPoller creates a new ConfigMap poller
func NewConfigMapPoller(
	clientSet kubernetes.Interface,
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	eventProcessor *EventProcessor,
	webhookProcessor *WebhookProcessor,
	eventsTotal *prometheus.CounterVec,
	observationCreator *ObservationCreator,
) *ConfigMapPoller {
	return &ConfigMapPoller{
		clientSet:          clientSet,
		dynClient:          dynClient,
		eventGVR:           eventGVR,
		eventProcessor:     eventProcessor,
		webhookProcessor:   webhookProcessor,
		interval:           5 * time.Minute,
		eventsTotal:        eventsTotal,
		observationCreator: observationCreator,
	}
}

// Start starts the ConfigMap polling loop
func (p *ConfigMapPoller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Initial run
	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// poll performs a single polling cycle
func (p *ConfigMapPoller) poll(ctx context.Context) {
	log.Println("🔍 Checking ConfigMap-based reports (kube-bench, checkov)...")

	p.processKubeBench(ctx)
	p.processCheckov(ctx)

	// Update totals
	totalCount := p.eventProcessor.GetTotalCount() + p.webhookProcessor.GetTotalCount()
	log.Printf("📊 Total Observations: %d", totalCount)
}

// processKubeBench processes kube-bench ConfigMaps
func (p *ConfigMapPoller) processKubeBench(ctx context.Context) {
	kubeBenchNs := os.Getenv("KUBE_BENCH_NAMESPACE")
	if kubeBenchNs == "" {
		kubeBenchNs = "kube-bench"
	}

	configMaps, err := p.clientSet.CoreV1().ConfigMaps(kubeBenchNs).List(ctx, metav1.ListOptions{
		LabelSelector: "app=kube-bench",
	})
	if err != nil || len(configMaps.Items) == 0 {
		log.Println("  ℹ️  No kube-bench ConfigMaps found (run kube-bench job to generate reports)")
		return
	}

	log.Printf("  ✓ Found %d kube-bench ConfigMaps", len(configMaps.Items))

	kubeBenchCount := 0
	for _, cm := range configMaps.Items {
		resultsJSON, found := cm.Data["results.json"]
		if !found {
			continue
		}

		var benchResults map[string]interface{}
		if err := json.Unmarshal([]byte(resultsJSON), &benchResults); err != nil {
			log.Printf("  ⚠️  Failed to parse kube-bench JSON: %v", err)
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

					event := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "zen.kube-zen.io/v1",
							"kind":       "Observation",
							"metadata": map[string]interface{}{
								"generateName": "kube-bench-",
								"namespace":    kubeBenchNs,
								"labels": map[string]interface{}{
									"source":   "kube-bench",
									"category": "compliance",
									"severity": severity,
								},
							},
							"spec": map[string]interface{}{
								"source":     "kube-bench",
								"category":   "compliance",
								"severity":   severity,
								"eventType":  "cis-benchmark-fail",
								"detectedAt": time.Now().Format(time.RFC3339),
								"resource": map[string]interface{}{
									"kind": "Node",
									"name": "k3d-zen-agent-server-0",
								},
								"details": map[string]interface{}{
									"testNumber":  testNumber,
									"section":     section,
									"testDesc":    testDesc,
									"remediation": remediation,
									"scored":      scored,
								},
							},
						},
					}

					// Use centralized observation creator - metrics are incremented automatically
					// Deduplication is handled by ObservationCreator
					err := p.observationCreator.CreateObservation(ctx, event)
					if err != nil {
						log.Printf("  ⚠️  Failed to create Observation: %v", err)
					} else {
						kubeBenchCount++
					}
				}
			}
		}
	}

	if kubeBenchCount > 0 {
		log.Printf("  ✅ Created %d NEW Observations from kube-bench CIS failures", kubeBenchCount)
	}
}

// processCheckov processes Checkov ConfigMaps
func (p *ConfigMapPoller) processCheckov(ctx context.Context) {
	log.Println("  → Checking Checkov reports...")
	checkovNs := os.Getenv("CHECKOV_NAMESPACE")
	if checkovNs == "" {
		checkovNs = "checkov"
	}

	checkovCMs, err := p.clientSet.CoreV1().ConfigMaps(checkovNs).List(ctx, metav1.ListOptions{
		LabelSelector: "app=checkov",
	})
	if err != nil || len(checkovCMs.Items) == 0 {
		log.Println("  ℹ️  No Checkov ConfigMaps found (run checkov scan to generate reports)")
		return
	}

	log.Printf("  ✓ Found %d checkov ConfigMaps", len(checkovCMs.Items))

	checkovCount := 0
	for _, cm := range checkovCMs.Items {
		resultsJSON, found := cm.Data["results.json"]
		if !found {
			continue
		}

		var checkovResults map[string]interface{}
		if err := json.Unmarshal([]byte(resultsJSON), &checkovResults); err != nil {
			log.Printf("  ⚠️  Failed to parse Checkov JSON: %v", err)
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

			// Parse resource (format: "Kind.namespace.name") using strings.SplitN
			resourceParts := strings.SplitN(resource, ".", 3)
			resourceKind := "Unknown"
			resourceNs := checkovNs
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

			// Deduplication is handled by ObservationCreator

			// Map by check prefix - use strings.HasPrefix instead of unsafe slicing
			category := "security"
			severity := "MEDIUM"
			if strings.HasPrefix(checkID, "CKV_K8S") {
				category = "security"
				if checkID == "CKV_K8S_20" || checkID == "CKV_K8S_23" || checkID == "CKV_K8S_16" {
					severity = "HIGH"
				}
			}

			event := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "zen.kube-zen.io/v1",
					"kind":       "Observation",
					"metadata": map[string]interface{}{
						"generateName": "checkov-",
						"namespace":    resourceNs,
						"labels": map[string]interface{}{
							"source":   "checkov",
							"category": category,
							"severity": severity,
						},
					},
					"spec": map[string]interface{}{
						"source":     "checkov",
						"category":   category,
						"severity":   severity,
						"eventType":  "static-analysis",
						"detectedAt": time.Now().Format(time.RFC3339),
						"resource": map[string]interface{}{
							"kind":      resourceKind,
							"name":      resourceName,
							"namespace": resourceNs,
						},
						"details": map[string]interface{}{
							"checkId":   checkID,
							"checkName": checkName,
							"resource":  resource,
							"guideline": guideline,
						},
					},
				},
			}

			// Use centralized observation creator - metrics are incremented automatically
			// Deduplication is handled by ObservationCreator
			err := p.observationCreator.CreateObservation(ctx, event)
			if err != nil {
				log.Printf("  ⚠️  Failed to create Checkov Observation: %v", err)
			} else {
				checkovCount++
			}
		}
	}

	if checkovCount > 0 {
		log.Printf("  ✅ Created %d NEW Observations from Checkov static analysis failures", checkovCount)
	}
}
