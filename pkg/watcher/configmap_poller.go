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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
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
	logger.Debug("Checking ConfigMap-based reports",
		logger.Fields{
			Component: "watcher",
			Operation: "configmap_poll",
		})

	p.processKubeBench(ctx)
	p.processCheckov(ctx)

	// Update totals
	totalCount := p.eventProcessor.GetTotalCount() + p.webhookProcessor.GetTotalCount()
	logger.Debug("Total Observations",
		logger.Fields{
			Component: "watcher",
			Operation: "configmap_poll",
			Count:     int(totalCount),
		})
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
		logger.Debug("No kube-bench ConfigMaps found",
			logger.Fields{
				Component: "watcher",
				Operation: "process_kube_bench",
				Source:    "kube-bench",
				Namespace: kubeBenchNs,
				Reason:    "no_configmaps",
			})
		return
	}

	logger.Debug("Found kube-bench ConfigMaps",
		logger.Fields{
			Component: "watcher",
			Operation: "process_kube_bench",
			Source:    "kube-bench",
			Namespace: kubeBenchNs,
			Count:     len(configMaps.Items),
		})

	kubeBenchCount := 0
	for _, cm := range configMaps.Items {
		resultsJSON, found := cm.Data["results.json"]
		if !found {
			continue
		}

		var benchResults map[string]interface{}
		if err := json.Unmarshal([]byte(resultsJSON), &benchResults); err != nil {
			logger.Warn("Failed to parse kube-bench JSON",
				logger.Fields{
					Component: "watcher",
					Operation: "process_kube_bench",
					Source:    "kube-bench",
					Namespace: kubeBenchNs,
					Error:     err,
				})
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
						logger.Warn("Failed to create Observation",
							logger.Fields{
								Component: "watcher",
								Operation: "observation_create",
								Source:    "kube-bench",
								Namespace: kubeBenchNs,
								Error:     err,
							})
					} else {
						kubeBenchCount++
					}
				}
			}
		}
	}

	if kubeBenchCount > 0 {
		logger.Info("Created Observations from kube-bench CIS failures",
			logger.Fields{
				Component: "watcher",
				Operation: "process_kube_bench",
				Source:    "kube-bench",
				Count:     kubeBenchCount,
			})
	}
}

// processCheckov processes Checkov ConfigMaps
func (p *ConfigMapPoller) processCheckov(ctx context.Context) {
	checkovNs := os.Getenv("CHECKOV_NAMESPACE")
	if checkovNs == "" {
		checkovNs = "checkov"
	}

	checkovCMs, err := p.clientSet.CoreV1().ConfigMaps(checkovNs).List(ctx, metav1.ListOptions{
		LabelSelector: "app=checkov",
	})
	if err != nil || len(checkovCMs.Items) == 0 {
		logger.Debug("No Checkov ConfigMaps found",
			logger.Fields{
				Component: "watcher",
				Operation: "process_checkov",
				Source:    "checkov",
				Namespace: checkovNs,
				Reason:    "no_configmaps",
			})
		return
	}

	logger.Debug("Found Checkov ConfigMaps",
		logger.Fields{
			Component: "watcher",
			Operation: "process_checkov",
			Source:    "checkov",
			Namespace: checkovNs,
			Count:     len(checkovCMs.Items),
		})

	checkovCount := 0
	for _, cm := range checkovCMs.Items {
		resultsJSON, found := cm.Data["results.json"]
		if !found {
			continue
		}

		var checkovResults map[string]interface{}
		if err := json.Unmarshal([]byte(resultsJSON), &checkovResults); err != nil {
			logger.Warn("Failed to parse Checkov JSON",
				logger.Fields{
					Component: "watcher",
					Operation: "process_checkov",
					Source:    "checkov",
					Namespace: checkovNs,
					Error:     err,
				})
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
				logger.Warn("Failed to create Checkov Observation",
					logger.Fields{
						Component: "watcher",
						Operation: "observation_create",
						Source:    "checkov",
						Namespace: resourceNs,
						Error:     err,
					})
			} else {
				checkovCount++
			}
		}
	}

	if checkovCount > 0 {
		logger.Info("Created Observations from Checkov static analysis failures",
			logger.Fields{
				Component: "watcher",
				Operation: "process_checkov",
				Source:    "checkov",
				Count:     checkovCount,
			})
	}
}
