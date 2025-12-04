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
	"os"
	"strconv"
	"strings"

	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ObservationCreator handles creation of Observations with centralized filtering, normalization, deduplication, and metrics
// Flow: filter() → normalize() → dedup() → create Observation CRD + update metrics + log
type ObservationCreator struct {
	dynClient                dynamic.Interface
	eventGVR                 schema.GroupVersionResource
	eventsTotal              *prometheus.CounterVec
	observationsCreated      *prometheus.CounterVec
	observationsFiltered     *prometheus.CounterVec
	observationsDeduped      prometheus.Counter
	observationsCreateErrors *prometheus.CounterVec
	deduper                  *dedup.Deduper
	filter                   *filter.Filter
}

// NewObservationCreator creates a new observation creator with optional filter and metrics
func NewObservationCreator(
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	eventsTotal *prometheus.CounterVec,
	observationsCreated *prometheus.CounterVec,
	observationsFiltered *prometheus.CounterVec,
	observationsDeduped prometheus.Counter,
	observationsCreateErrors *prometheus.CounterVec,
	filter *filter.Filter,
) *ObservationCreator {
	// Get dedup window from env, default 60 seconds
	windowSeconds := 60
	if windowStr := os.Getenv("DEDUP_WINDOW_SECONDS"); windowStr != "" {
		if w, err := strconv.Atoi(windowStr); err == nil && w > 0 {
			windowSeconds = w
		}
	}
	// Get max cache size from env, default 10000
	maxSize := 10000
	if sizeStr := os.Getenv("DEDUP_MAX_SIZE"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			maxSize = s
		}
	}
	return &ObservationCreator{
		dynClient:                dynClient,
		eventGVR:                 eventGVR,
		eventsTotal:              eventsTotal,
		observationsCreated:      observationsCreated,
		observationsFiltered:     observationsFiltered,
		observationsDeduped:      observationsDeduped,
		observationsCreateErrors: observationsCreateErrors,
		deduper:                  dedup.NewDeduper(windowSeconds, maxSize),
		filter:                   filter,
	}
}

// CreateObservation creates an Observation CRD and increments metrics
// This is the centralized place where all Observations are created
// Flow: filter() → normalize() → dedup() → create Observation CRD + update metrics + log
// First event always creates an observation; duplicates within window are skipped
func (oc *ObservationCreator) CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error {
	// Extract source early for metrics
	sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	}
	if source == "" {
		source = "unknown"
	}

	// STEP 1: FILTER - Apply source-level filtering BEFORE normalization and deduplication
	if oc.filter != nil {
		allowed, reason := oc.filter.AllowWithReason(observation)
		if !allowed {
			// Filtered out - increment filtered metric and return early
			if oc.observationsFiltered != nil {
				oc.observationsFiltered.WithLabelValues(source, reason).Inc()
			}
			return nil
		}
	}

	// STEP 2: NORMALIZE - Normalize severity to uppercase for consistency
	// (Normalization happens inline during extraction below)

	// STEP 3: DEDUP - Check if we should create (first event always creates, duplicates within window are skipped)
	// Uses enhanced dedup with time buckets, fingerprinting, rate limiting, and aggregation
	dedupKey := oc.extractDedupKey(observation)
	if !oc.deduper.ShouldCreateWithContent(dedupKey, observation.Object) {
		logger.Debug("Skipping duplicate observation within window",
			logger.Fields{
				Component: "watcher",
				Operation: "observation_dedup",
				Source:    source,
				Reason:    "duplicate_within_window",
				Additional: map[string]interface{}{
					"dedup_key": dedupKey.String(),
				},
			})
		// Increment deduped metric
		if oc.observationsDeduped != nil {
			oc.observationsDeduped.Inc()
		}
		return nil // Skip duplicate, but don't error
	}

	// Extract namespace from observation
	namespace, found, _ := unstructured.NestedString(observation.Object, "metadata", "namespace")
	if !found || namespace == "" {
		namespace = "default"
	}

	// Ensure metadata exists (for labels, annotations, etc. - not TTL specific)
	metadata, _, _ := unstructured.NestedMap(observation.Object, "metadata")
	if metadata == nil {
		metadata = make(map[string]interface{})
		unstructured.SetNestedMap(observation.Object, metadata, "metadata")
	}

	// Extract category and severity from spec for metrics BEFORE creation
	// Use NestedFieldCopy to handle interface{} types, then convert to string
	categoryVal, categoryFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "category")
	severityVal, severityFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "severity")
	category := ""
	if categoryVal != nil {
		category = fmt.Sprintf("%v", categoryVal)
	} else if !categoryFound {
		logger.Debug("Category not found in spec",
			logger.Fields{
				Component: "watcher",
				Operation: "observation_create",
				Source:    source,
			})
	}
	severity := ""
	if severityVal != nil {
		severity = fmt.Sprintf("%v", severityVal)
	} else if !severityFound {
		logger.Debug("Severity not found in spec",
			logger.Fields{
				Component: "watcher",
				Operation: "observation_create",
				Source:    source,
			})
	}

	// Normalize severity to uppercase for consistency
	if severity != "" {
		severity = normalizeSeverity(severity)
	}

	// Set TTL in spec if not already set (Kubernetes native style)
	oc.setTTLIfNotSet(observation)

	// STEP 1: CREATE OBSERVATION CRD FIRST (before metrics)
	// Create the Observation (first event always creates)
	createdObservation, err := oc.dynClient.Resource(oc.eventGVR).Namespace(namespace).Create(ctx, observation, metav1.CreateOptions{})
	if err != nil {
		// Track creation errors
		if oc.observationsCreateErrors != nil {
			errorType := "create_failed"
			errMsg := strings.ToLower(err.Error())
			// Extract error type from error message
			if strings.Contains(errMsg, "already exists") {
				errorType = "already_exists"
			} else if strings.Contains(errMsg, "forbidden") {
				errorType = "forbidden"
			} else if strings.Contains(errMsg, "not found") {
				errorType = "not_found"
			}
			oc.observationsCreateErrors.WithLabelValues(source, errorType).Inc()
		}
		return fmt.Errorf("failed to create Observation: %w", err)
	}

	// STEP 2: UPDATE METRICS ONLY AFTER SUCCESSFUL CREATION
	// Increment observations created metric
	if oc.observationsCreated != nil {
		oc.observationsCreated.WithLabelValues(source).Inc()
	}

	// Increment eventsTotal metric - this tracks events by source/category/severity
	// Only increment AFTER observation CRD is successfully created
	if oc.eventsTotal != nil {
		if category == "" {
			category = "unknown"
		}
		if severity == "" {
			severity = "UNKNOWN"
		}
		oc.eventsTotal.WithLabelValues(source, category, severity).Inc()
		logger.Debug("Metric incremented after observation creation",
			logger.Fields{
				Component: "watcher",
				Operation: "observation_create",
				Source:    source,
				Additional: map[string]interface{}{
					"category":              category,
					"severity":              severity,
					"observation_name":      createdObservation.GetName(),
					"observation_namespace": namespace,
				},
			})
	} else {
		logger.Error("eventsTotal metric is nil, metrics will not be incremented",
			logger.Fields{
				Component: "watcher",
				Operation: "observation_create",
				Source:    source,
			})
	}

	// STEP 3: LOG SUCCESS
	logger.Debug("Observation created successfully",
		logger.Fields{
			Component: "watcher",
			Operation: "observation_create",
			Source:    source,
			Namespace: namespace,
			Additional: map[string]interface{}{
				"observation_name": createdObservation.GetName(),
				"category":         category,
				"severity":         severity,
			},
		})

	return nil
}

// extractDedupKey extracts minimal metadata for deduplication from observation
func (oc *ObservationCreator) extractDedupKey(observation *unstructured.Unstructured) dedup.DedupKey {
	// Extract source
	sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	}

	// Extract resource info
	resourceVal, _, _ := unstructured.NestedMap(observation.Object, "spec", "resource")
	namespace := ""
	kind := ""
	name := ""
	if resourceVal != nil {
		if ns, ok := resourceVal["namespace"].(string); ok {
			namespace = ns
		} else if ns, ok := resourceVal["namespace"].(interface{}); ok {
			namespace = fmt.Sprintf("%v", ns)
		}
		if k, ok := resourceVal["kind"].(string); ok {
			kind = k
		} else if k, ok := resourceVal["kind"].(interface{}); ok {
			kind = fmt.Sprintf("%v", k)
		}
		if n, ok := resourceVal["name"].(string); ok {
			name = n
		} else if n, ok := resourceVal["name"].(interface{}); ok {
			name = fmt.Sprintf("%v", n)
		}
	}
	// Fallback to metadata namespace if resource namespace is empty
	if namespace == "" {
		namespace, _, _ = unstructured.NestedString(observation.Object, "metadata", "namespace")
		if namespace == "" {
			namespace = "default"
		}
	}

	// Extract reason from details or eventType
	reason := ""
	detailsVal, _, _ := unstructured.NestedMap(observation.Object, "spec", "details")
	if detailsVal != nil {
		// Try common reason fields
		if r, ok := detailsVal["reason"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["reason"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["rule"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["rule"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["testNumber"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["testNumber"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["checkId"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["checkId"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["vulnerabilityID"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["vulnerabilityID"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["auditID"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["auditID"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		}
	}
	// Fallback to eventType if no reason found
	if reason == "" {
		eventTypeVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "eventType")
		if eventTypeVal != nil {
			reason = fmt.Sprintf("%v", eventTypeVal)
		}
	}

	// Extract message for hashing
	message := ""
	if detailsVal != nil {
		if msg, ok := detailsVal["message"].(string); ok {
			message = msg
		} else if msg, ok := detailsVal["message"].(interface{}); ok {
			message = fmt.Sprintf("%v", msg)
		} else if msg, ok := detailsVal["output"].(string); ok {
			message = msg
		} else if msg, ok := detailsVal["output"].(interface{}); ok {
			message = fmt.Sprintf("%v", msg)
		}
	}

	// Hash message
	messageHash := dedup.HashMessage(message)

	return dedup.DedupKey{
		Source:      source,
		Namespace:   namespace,
		Kind:        kind,
		Name:        name,
		Reason:      reason,
		MessageHash: messageHash,
	}
}

// setTTLIfNotSet sets spec.ttlSecondsAfterCreation if not already set
// Priority: 1) Already set in spec, 2) Environment variable, 3) Default (7 days)
func (oc *ObservationCreator) setTTLIfNotSet(observation *unstructured.Unstructured) {
	// Check if TTL is already set in spec
	if ttlVal, found, _ := unstructured.NestedInt64(observation.Object, "spec", "ttlSecondsAfterCreation"); found && ttlVal > 0 {
		// Already set, don't override
		return
	}

	// Get default TTL from environment variable (in seconds)
	// Convert from days (OBSERVATION_TTL_DAYS) or use OBSERVATION_TTL_SECONDS if set
	var defaultTTLSeconds int64 = 7 * 24 * 60 * 60 // Default: 7 days in seconds

	// TTL validation bounds
	const (
		MinTTLSeconds = 60                 // 1 minute minimum (prevents immediate deletion)
		MaxTTLSeconds = 365 * 24 * 60 * 60 // 1 year maximum (prevents indefinite retention)
	)

	// Check for seconds first (more precise)
	if ttlSecondsStr := os.Getenv("OBSERVATION_TTL_SECONDS"); ttlSecondsStr != "" {
		if ttlSeconds, err := strconv.ParseInt(ttlSecondsStr, 10, 64); err == nil && ttlSeconds > 0 {
			defaultTTLSeconds = ttlSeconds
		}
	} else if ttlDaysStr := os.Getenv("OBSERVATION_TTL_DAYS"); ttlDaysStr != "" {
		// Fallback to days (for backward compatibility)
		if ttlDays, err := strconv.Atoi(ttlDaysStr); err == nil && ttlDays > 0 {
			defaultTTLSeconds = int64(ttlDays) * 24 * 60 * 60
		}
	}

	// Validate TTL bounds
	if defaultTTLSeconds < MinTTLSeconds {
		logger.Warn("TTL value too small, using minimum",
			logger.Fields{
				Component: "watcher",
				Operation: "ttl_validation",
				Additional: map[string]interface{}{
					"requested_ttl": defaultTTLSeconds,
					"minimum_ttl":   MinTTLSeconds,
					"applied_ttl":   MinTTLSeconds,
				},
			})
		defaultTTLSeconds = MinTTLSeconds
	} else if defaultTTLSeconds > MaxTTLSeconds {
		logger.Warn("TTL value too large, using maximum",
			logger.Fields{
				Component: "watcher",
				Operation: "ttl_validation",
				Additional: map[string]interface{}{
					"requested_ttl": defaultTTLSeconds,
					"maximum_ttl":   MaxTTLSeconds,
					"applied_ttl":   MaxTTLSeconds,
				},
			})
		defaultTTLSeconds = MaxTTLSeconds
	}

	// Ensure spec exists
	spec, _, _ := unstructured.NestedMap(observation.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
		unstructured.SetNestedMap(observation.Object, spec, "spec")
	}

	// Set TTL in spec (only if not already set)
	spec["ttlSecondsAfterCreation"] = defaultTTLSeconds
	unstructured.SetNestedMap(observation.Object, spec, "spec")
}
