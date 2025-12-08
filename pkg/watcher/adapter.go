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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Event represents a normalized internal event model before it becomes an Observation.
// This is the standard interface that all source adapters must produce.
type Event struct {
	// Core required fields (map to Observation spec)
	Source    string                 `json:"source"`    // Tool name (e.g., "falco", "trivy", "opagatekeeper")
	Category  string                 `json:"category"`  // Event category (security, compliance, performance)
	Severity  string                 `json:"severity"`  // Severity level (CRITICAL, HIGH, MEDIUM, LOW)
	EventType string                 `json:"eventType"` // Type of event (vulnerability, runtime-threat, policy-violation)
	Resource  *ResourceRef           `json:"resource"`  // Affected Kubernetes resource
	Details   map[string]interface{} `json:"details"`   // Tool-specific details (preserved in spec.details)

	// Optional metadata
	Namespace  string `json:"namespace,omitempty"`  // Target namespace for Observation CRD
	DetectedAt string `json:"detectedAt,omitempty"` // RFC3339 timestamp
}

// ResourceRef represents a Kubernetes resource reference
type ResourceRef struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
}

// SourceAdapter is the formal interface for all event source adapters.
// This interface makes it easy for community contributors to add new sources.
//
// Implementations can:
// - Use informers (Kyverno, Trivy)
// - Tail logs (Falco, Trivy)
// - Poll ConfigMaps (kube-bench, Checkov)
// - Call external APIs (Kubecost, etc.)
//
// All adapters output normalized Event objects that are then processed
// through the centralized ObservationCreator (filter → dedup → create CRD).
type SourceAdapter interface {
	// Name returns the unique source name (e.g., "falco", "trivy", "opagatekeeper")
	// This must match the source name used in filter configuration and metrics.
	Name() string

	// Run starts the adapter and sends normalized Events to the output channel.
	// The adapter should run until ctx is cancelled.
	// Errors should be logged but should not stop the adapter unless fatal.
	// The adapter is responsible for its own error handling and retries.
	Run(ctx context.Context, out chan<- *Event) error

	// Stop gracefully stops the adapter and cleans up resources.
	// This is called during shutdown.
	Stop()
	
	// Optimization methods (optional - adapters can implement if they support optimization)
	
	// GetOptimizationMetrics returns optimization metrics for this adapter
	// Returns nil if optimization is not supported
	GetOptimizationMetrics() interface{} // Returns *optimization.OptimizationMetrics
	
	// ApplyOptimization applies optimization configuration to this adapter
	// Returns error if optimization cannot be applied
	ApplyOptimization(config interface{}) error // config is *config.SourceConfig
	
	// ValidateOptimization validates optimization configuration for this adapter
	// Returns error if configuration is invalid
	ValidateOptimization(config interface{}) error // config is *config.SourceConfig
	
	// ResetMetrics resets optimization metrics for this adapter
	ResetMetrics()
}

// EventToObservation converts a normalized Event to an unstructured.Unstructured Observation CRD.
// This is a helper function for adapters to convert Events to the Observation format.
func EventToObservation(event *Event) *unstructured.Unstructured {
	if event == nil {
		return nil
	}

	namespace := event.Namespace
	if namespace == "" {
		namespace = "default"
	}

	detectedAt := event.DetectedAt
	if detectedAt == "" {
		detectedAt = time.Now().Format(time.RFC3339)
	}

	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "zen.kube-zen.io/v1",
			"kind":       "Observation",
			"metadata": map[string]interface{}{
				"generateName": event.Source + "-",
				"namespace":    namespace,
				"labels": map[string]interface{}{
					"source":   event.Source,
					"category": event.Category,
					"severity": event.Severity,
				},
			},
			"spec": map[string]interface{}{
				"source":     event.Source,
				"category":   event.Category,
				"severity":   event.Severity,
				"eventType":  event.EventType,
				"detectedAt": detectedAt,
			},
		},
	}

	// Add resource if provided
	if event.Resource != nil {
		resource := map[string]interface{}{
			"kind": event.Resource.Kind,
			"name": event.Resource.Name,
		}
		if event.Resource.APIVersion != "" {
			resource["apiVersion"] = event.Resource.APIVersion
		}
		if event.Resource.Namespace != "" {
			resource["namespace"] = event.Resource.Namespace
		}
		unstructured.SetNestedMap(obs.Object, resource, "spec", "resource")
	}

	// Add details if provided
	if len(event.Details) > 0 {
		unstructured.SetNestedMap(obs.Object, event.Details, "spec", "details")
	}

	return obs
}
