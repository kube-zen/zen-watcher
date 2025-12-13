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
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/optimization"
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
	// Optimization metrics (optional)
	optimizationMetrics *OptimizationMetrics
	// Source config loader (optional, for dynamic processing order)
	sourceConfigLoader interface {
		GetSourceConfig(source string) *generic.SourceConfig
	}
	// Current processing order per source (for logging changes)
	currentOrder map[string]ProcessingOrder
	orderMu      sync.RWMutex
	// SmartProcessor for per-source optimization (optional)
	smartProcessor *optimization.SmartProcessor
	// System metrics tracker for HA coordination (optional)
	systemMetrics interface {
		RecordEvent()
		SetQueueDepth(int)
	}
	// Field extractor for optimized field access (Phase 2 optimization)
	fieldExtractor *FieldExtractor
	// Metrics for destination delivery tracking
	destinationMetrics *metrics.Metrics
}

// OptimizationMetrics holds optimization-related metrics
type OptimizationMetrics struct {
	FilterPassRate        *prometheus.GaugeVec
	DedupEffectiveness    *prometheus.GaugeVec
	LowSeverityPercent    *prometheus.GaugeVec
	ObservationsPerMinute *prometheus.GaugeVec
	ObservationsPerHour   *prometheus.GaugeVec
	SeverityDistribution  *prometheus.CounterVec
	// Per-source counters for calculating rates
	sourceCounters map[string]*sourceCounters
	countersMu     sync.RWMutex
}

// sourceCounters tracks counters per source for rate calculations
type sourceCounters struct {
	attempted   int64 // Total events attempted
	filtered    int64 // Events filtered out
	deduped     int64 // Events deduplicated
	created     int64 // Events created
	lowSeverity int64 // LOW severity count
	totalCount  int64 // Total count for severity distribution
	lastUpdate  time.Time
}

// NewOptimizationMetrics creates optimization metrics from the main Metrics struct
func NewOptimizationMetrics(filterPassRate, dedupEffectiveness, lowSeverityPercent,
	observationsPerMinute, observationsPerHour *prometheus.GaugeVec,
	severityDistribution *prometheus.CounterVec) *OptimizationMetrics {
	return &OptimizationMetrics{
		FilterPassRate:        filterPassRate,
		DedupEffectiveness:    dedupEffectiveness,
		LowSeverityPercent:    lowSeverityPercent,
		ObservationsPerMinute: observationsPerMinute,
		ObservationsPerHour:   observationsPerHour,
		SeverityDistribution:  severityDistribution,
		sourceCounters:        make(map[string]*sourceCounters),
	}
}

// GetDeduper returns a reference to the deduper for dynamic configuration updates
func (oc *ObservationCreator) GetDeduper() *dedup.Deduper {
	return oc.deduper
}

// SetSystemMetrics sets the system metrics tracker for HA coordination
func (oc *ObservationCreator) SetSystemMetrics(sm interface {
	RecordEvent()
	SetQueueDepth(int)
}) {
	oc.systemMetrics = sm
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
	return NewObservationCreatorWithOptimization(
		dynClient, eventGVR, eventsTotal, observationsCreated,
		observationsFiltered, observationsDeduped, observationsCreateErrors,
		filter, nil,
	)
}

// NewObservationCreatorWithOptimization creates a new observation creator with optimization metrics
func NewObservationCreatorWithOptimization(
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	eventsTotal *prometheus.CounterVec,
	observationsCreated *prometheus.CounterVec,
	observationsFiltered *prometheus.CounterVec,
	observationsDeduped prometheus.Counter,
	observationsCreateErrors *prometheus.CounterVec,
	filter *filter.Filter,
	optimizationMetrics *OptimizationMetrics,
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
		optimizationMetrics:      optimizationMetrics,
		currentOrder:             make(map[string]ProcessingOrder),
		smartProcessor:           optimization.NewSmartProcessor(), // Created here, can be shared with optimizer
		fieldExtractor:           NewFieldExtractor(),
	}
}

// SetOptimizationMetrics sets optimization metrics for recording
func (oc *ObservationCreator) SetOptimizationMetrics(metrics *OptimizationMetrics) {
	oc.optimizationMetrics = metrics
}

// SetSourceConfigLoader sets the source config loader for dynamic processing order
func (oc *ObservationCreator) SetSourceConfigLoader(loader interface {
	GetSourceConfig(source string) *generic.SourceConfig
}) {
	oc.sourceConfigLoader = loader
}

// GetSmartProcessor returns the SmartProcessor instance (for integration with optimizer)
func (oc *ObservationCreator) GetSmartProcessor() *optimization.SmartProcessor {
	return oc.smartProcessor
}

// SetDestinationMetrics sets destination metrics for tracking delivery
func (oc *ObservationCreator) SetDestinationMetrics(m *metrics.Metrics) {
	oc.destinationMetrics = m
}

// CreateObservation creates an Observation CRD and increments metrics
// IMPORTANT: This method assumes the event has already been processed (filtered and deduplicated)
// by the Processor. It does NOT re-run filter or dedup logic.
// The canonical pipeline order is enforced in Processor.ProcessEvent:
// source → (filter | dedup, order chosen by optimization) → normalize → CreateObservation
func (oc *ObservationCreator) CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error {
	// Extract source early for metrics (optimized with field extractor)
	sourceVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	}
	if source == "" {
		source = "unknown"
	}

	// Record attempted event for optimization metrics
	if oc.optimizationMetrics != nil {
		oc.recordEventAttempted(source)
	}

	// Get SmartProcessor metrics collector if available
	startTime := time.Now()
	var collector *optimization.PerSourceMetricsCollector
	if oc.smartProcessor != nil {
		collector = oc.smartProcessor.GetOrCreateMetricsCollector(source)
	}

	// Create the observation (filter and dedup were already handled by Processor)
	err := oc.createObservation(ctx, observation, source)

	// Record processing time in SmartProcessor metrics if available
	if collector != nil {
		collector.RecordProcessing(time.Since(startTime), err)
	}

	return err
}

func (oc *ObservationCreator) determineProcessingOrder(source string) ProcessingOrder {
	return ProcessingOrderFilterFirst
}

// getSourceMetrics gets current metrics for a source from optimization metrics
func (oc *ObservationCreator) getSourceMetrics(source string) *SourceMetrics {
	if oc.optimizationMetrics == nil {
		return nil
	}

	oc.optimizationMetrics.countersMu.RLock()
	defer oc.optimizationMetrics.countersMu.RUnlock()

	counters, exists := oc.optimizationMetrics.sourceCounters[source]
	if !exists {
		return nil
	}

	// Calculate metrics
	lowSeverityPercent := 0.0
	if counters.totalCount > 0 {
		lowSeverityPercent = float64(counters.lowSeverity) / float64(counters.totalCount)
	}

	dedupEffectiveness := 0.0
	totalProcessed := counters.created + counters.deduped
	if totalProcessed > 0 {
		dedupEffectiveness = float64(counters.deduped) / float64(totalProcessed)
	}

	obsPerMinute := 0.0
	now := time.Now()
	timeWindow := now.Sub(counters.lastUpdate)
	if timeWindow > 0 && timeWindow < 5*time.Minute {
		obsPerMinute = float64(counters.created) / timeWindow.Minutes()
	}

	return &SourceMetrics{
		Source:                source,
		ObservationsPerMinute: obsPerMinute,
		LowSeverityPercent:    lowSeverityPercent,
		DedupEffectiveness:    dedupEffectiveness,
		TotalObservations:     counters.totalCount,
		FilteredCount:         counters.filtered,
		DedupedCount:          counters.deduped,
		CreatedCount:          counters.created,
	}
}

// createObservation creates the observation (extracted from original CreateObservation)
func (oc *ObservationCreator) createObservation(ctx context.Context, observation *unstructured.Unstructured, source string) error {

	// Extract namespace from observation (optimized)
	namespace, found := oc.fieldExtractor.ExtractString(observation.Object, "metadata", "namespace")
	if !found || namespace == "" {
		namespace = "default"
	}

	// Ensure metadata exists (for labels, annotations, etc. - not TTL specific)
	metadata, _ := oc.fieldExtractor.ExtractMap(observation.Object, "metadata")
	if metadata == nil {
		metadata = make(map[string]interface{})
		unstructured.SetNestedMap(observation.Object, metadata, "metadata")
	}

	// Extract category, severity, and eventType from spec for metrics BEFORE creation (optimized)
	categoryVal, categoryFound := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "category")
	severityVal, severityFound := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "severity")
	eventTypeVal, eventTypeFound := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "eventType")
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
	eventType := ""
	if eventTypeVal != nil {
		eventType = fmt.Sprintf("%v", eventTypeVal)
	} else if !eventTypeFound {
		logger.Debug("EventType not found in spec",
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
	deliveryStartTime := time.Now()
	createdObservation, err := oc.dynClient.Resource(oc.eventGVR).Namespace(namespace).Create(ctx, observation, metav1.CreateOptions{})
	deliveryDuration := time.Since(deliveryStartTime)

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
		// Track destination delivery failure
		if oc.destinationMetrics != nil {
			oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "failure").Inc()
			oc.destinationMetrics.DestinationDeliveryLatency.WithLabelValues(source, "crd").Observe(deliveryDuration.Seconds())
		}
		return fmt.Errorf("failed to create Observation: %w", err)
	}

	// Track successful destination delivery
	if oc.destinationMetrics != nil {
		oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "success").Inc()
		oc.destinationMetrics.DestinationDeliveryLatency.WithLabelValues(source, "crd").Observe(deliveryDuration.Seconds())
	}

	// STEP 2: UPDATE METRICS ONLY AFTER SUCCESSFUL CREATION
	// Increment observations created metric
	if oc.observationsCreated != nil {
		oc.observationsCreated.WithLabelValues(source).Inc()
	}

	// Track event for HA metrics
	if oc.systemMetrics != nil {
		oc.systemMetrics.RecordEvent()
	}

	// Record created event for optimization metrics
	if oc.optimizationMetrics != nil {
		oc.recordEventCreated(source, severity)
		oc.updateOptimizationMetrics(source)
	}

	// Increment eventsTotal metric - this tracks events by source/category/severity/eventType/namespace/kind
	// Only increment AFTER observation CRD is successfully created
	if oc.eventsTotal != nil {
		if category == "" {
			category = "unknown"
		}
		if severity == "" {
			severity = "info"
		}
		if eventType == "" {
			eventType = "unknown"
		}

		// Extract namespace and kind from resource (optimized)
		resourceNamespace := namespace // Use observation namespace as fallback
		resourceKind := ""
		resourceVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec", "resource")
		if resourceVal != nil {
			if ns, ok := resourceVal["namespace"].(string); ok && ns != "" {
				resourceNamespace = ns
			} else if ns, ok := resourceVal["namespace"].(interface{}); ok {
				resourceNamespace = fmt.Sprintf("%v", ns)
			}
			if k, ok := resourceVal["kind"].(string); ok && k != "" {
				resourceKind = k
			} else if k, ok := resourceVal["kind"].(interface{}); ok {
				resourceKind = fmt.Sprintf("%v", k)
			}
		}
		if resourceNamespace == "" {
			resourceNamespace = "default"
		}
		if resourceKind == "" {
			resourceKind = "Unknown"
		}

		// Get processing strategy for this source (default to filter_first)
		strategy := "filter_first"
		oc.orderMu.RLock()
		if order, exists := oc.currentOrder[source]; exists {
			strategy = string(order)
		}
		oc.orderMu.RUnlock()

		oc.eventsTotal.WithLabelValues(source, category, severity, eventType, resourceNamespace, resourceKind, strategy).Inc()
		logger.Debug("Metric incremented after observation creation",
			logger.Fields{
				Component: "watcher",
				Operation: "observation_create",
				Source:    source,
				Additional: map[string]interface{}{
					"category":              category,
					"severity":              severity,
					"eventType":             eventType,
					"observation_name":      createdObservation.GetName(),
					"observation_namespace": namespace,
					"resource_namespace":    resourceNamespace,
					"resource_kind":         resourceKind,
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
	// Extract source (optimized)
	sourceVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	}

	// Extract resource info (optimized)
	resourceVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec", "resource")
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
	// Fallback to metadata namespace if resource namespace is empty (optimized)
	if namespace == "" {
		namespace, _ = oc.fieldExtractor.ExtractString(observation.Object, "metadata", "namespace")
		if namespace == "" {
			namespace = "default"
		}
	}

	// Extract reason from details or eventType (optimized)
	reason := ""
	detailsVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec", "details")
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
	// Fallback to eventType if no reason found (optimized)
	if reason == "" {
		eventTypeVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "eventType")
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
	// Check if TTL is already set in spec (optimized)
	if ttlVal, found := oc.fieldExtractor.ExtractInt64(observation.Object, "spec", "ttlSecondsAfterCreation"); found && ttlVal > 0 {
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

	// Ensure spec exists (optimized)
	spec, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
		unstructured.SetNestedMap(observation.Object, spec, "spec")
	}

	// Set TTL in spec (only if not already set)
	spec["ttlSecondsAfterCreation"] = defaultTTLSeconds
	unstructured.SetNestedMap(observation.Object, spec, "spec")
}

// recordEventAttempted records that an event was attempted
func (oc *ObservationCreator) recordEventAttempted(source string) {
	if oc.optimizationMetrics == nil {
		return
	}
	oc.optimizationMetrics.countersMu.Lock()
	defer oc.optimizationMetrics.countersMu.Unlock()

	if oc.optimizationMetrics.sourceCounters == nil {
		oc.optimizationMetrics.sourceCounters = make(map[string]*sourceCounters)
	}

	if _, exists := oc.optimizationMetrics.sourceCounters[source]; !exists {
		oc.optimizationMetrics.sourceCounters[source] = &sourceCounters{
			lastUpdate: time.Now(),
		}
	}
	oc.optimizationMetrics.sourceCounters[source].attempted++
}

// recordEventFiltered records that an event was filtered
func (oc *ObservationCreator) recordEventFiltered(source, reason string) {
	if oc.optimizationMetrics == nil {
		return
	}
	oc.optimizationMetrics.countersMu.Lock()
	defer oc.optimizationMetrics.countersMu.Unlock()

	if oc.optimizationMetrics.sourceCounters == nil {
		oc.optimizationMetrics.sourceCounters = make(map[string]*sourceCounters)
	}

	if _, exists := oc.optimizationMetrics.sourceCounters[source]; !exists {
		oc.optimizationMetrics.sourceCounters[source] = &sourceCounters{
			lastUpdate: time.Now(),
		}
	}
	oc.optimizationMetrics.sourceCounters[source].filtered++
}

// recordEventDeduped records that an event was deduplicated
func (oc *ObservationCreator) recordEventDeduped(source string) {
	if oc.optimizationMetrics == nil {
		return
	}
	oc.optimizationMetrics.countersMu.Lock()
	defer oc.optimizationMetrics.countersMu.Unlock()

	if oc.optimizationMetrics.sourceCounters == nil {
		oc.optimizationMetrics.sourceCounters = make(map[string]*sourceCounters)
	}

	if _, exists := oc.optimizationMetrics.sourceCounters[source]; !exists {
		oc.optimizationMetrics.sourceCounters[source] = &sourceCounters{
			lastUpdate: time.Now(),
		}
	}
	oc.optimizationMetrics.sourceCounters[source].deduped++
}

// recordEventCreated records that an event was created
func (oc *ObservationCreator) recordEventCreated(source, severity string) {
	if oc.optimizationMetrics == nil {
		return
	}
	oc.optimizationMetrics.countersMu.Lock()
	defer oc.optimizationMetrics.countersMu.Unlock()

	if oc.optimizationMetrics.sourceCounters == nil {
		oc.optimizationMetrics.sourceCounters = make(map[string]*sourceCounters)
	}

	if _, exists := oc.optimizationMetrics.sourceCounters[source]; !exists {
		oc.optimizationMetrics.sourceCounters[source] = &sourceCounters{
			lastUpdate: time.Now(),
		}
	}
	oc.optimizationMetrics.sourceCounters[source].created++
	oc.optimizationMetrics.sourceCounters[source].totalCount++

	// Track low severity
	severityLower := strings.ToLower(severity)
	if severityLower == "low" || severityLower == "info" {
		oc.optimizationMetrics.sourceCounters[source].lowSeverity++
	}

	// Update severity distribution
	if oc.optimizationMetrics.SeverityDistribution != nil {
		oc.optimizationMetrics.SeverityDistribution.WithLabelValues(source, strings.ToUpper(severity)).Inc()
	}
}

// updateOptimizationMetrics calculates and updates optimization metrics
func (oc *ObservationCreator) updateOptimizationMetrics(source string) {
	if oc.optimizationMetrics == nil {
		return
	}

	oc.optimizationMetrics.countersMu.RLock()
	counters, exists := oc.optimizationMetrics.sourceCounters[source]
	oc.optimizationMetrics.countersMu.RUnlock()

	if !exists || counters.attempted == 0 {
		return
	}

	// Calculate filter pass rate: (created + deduped) / attempted
	filterPassRate := float64(counters.created+counters.deduped) / float64(counters.attempted)
	if oc.optimizationMetrics.FilterPassRate != nil {
		oc.optimizationMetrics.FilterPassRate.WithLabelValues(source).Set(filterPassRate)
	}

	// Calculate dedup effectiveness: deduped / (created + deduped)
	totalProcessed := counters.created + counters.deduped
	dedupEffectiveness := 0.0
	if totalProcessed > 0 {
		dedupEffectiveness = float64(counters.deduped) / float64(totalProcessed)
	}
	if oc.optimizationMetrics.DedupEffectiveness != nil {
		oc.optimizationMetrics.DedupEffectiveness.WithLabelValues(source).Set(dedupEffectiveness)
	}

	// Calculate low severity percent
	lowSeverityPercent := 0.0
	if counters.totalCount > 0 {
		lowSeverityPercent = float64(counters.lowSeverity) / float64(counters.totalCount)
	}
	if oc.optimizationMetrics.LowSeverityPercent != nil {
		oc.optimizationMetrics.LowSeverityPercent.WithLabelValues(source).Set(lowSeverityPercent)
	}

	// Calculate observations per minute (simplified - uses time window)
	now := time.Now()
	timeWindow := now.Sub(counters.lastUpdate)
	if timeWindow > 0 && timeWindow < 5*time.Minute {
		// Only update if within reasonable window
		obsPerMinute := float64(counters.created) / timeWindow.Minutes()
		if oc.optimizationMetrics.ObservationsPerMinute != nil {
			oc.optimizationMetrics.ObservationsPerMinute.WithLabelValues(source).Set(obsPerMinute)
		}
		obsPerHour := obsPerMinute * 60
		if oc.optimizationMetrics.ObservationsPerHour != nil {
			oc.optimizationMetrics.ObservationsPerHour.WithLabelValues(source).Set(obsPerHour)
		}
	}
}
