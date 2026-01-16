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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	sdkdedup "github.com/kube-zen/zen-sdk/pkg/dedup"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/optimization"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Package-level logger to avoid repeated allocations
var (
	observationLogger = sdklog.NewLogger("zen-watcher-observation-creator")
)

// ObservationCreator handles creation of resources (any GVR) with centralized filtering, normalization, deduplication, and metrics
// Flow: filter() → normalize() → dedup() → create resource (any GVR) + update metrics + log
// Note: Named "ObservationCreator" for backward compatibility, but it creates any resource type based on destination GVR
// H037: GVR writes are restricted by allowlist
type ObservationCreator struct {
	dynClient    dynamic.Interface
	eventGVR     schema.GroupVersionResource                     // Default GVR (for backward compatibility)
	gvrResolver  func(source string) schema.GroupVersionResource // Optional: Resolve GVR from source
	gvrAllowlist *GVRAllowlist                                   // H037: GVR allowlist for write restrictions

	// Metrics tracking
	metrics *MetricsTracker

	// Processing components
	deduper *sdkdedup.Deduper
	filter  *filter.Filter

	// Optimization components (optional)
	optimizationMetrics *OptimizationMetrics
	smartProcessor      *optimization.SmartProcessor
	systemMetrics       interface {
		RecordEvent()
		SetQueueDepth(int)
	}

	// Configuration
	sourceConfigLoader interface {
		GetSourceConfig(source string) *generic.SourceConfig
	}
	currentOrder map[string]ProcessingOrder
	orderMu      sync.RWMutex

	// Utilities
	fieldExtractor     *FieldExtractor
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
func (oc *ObservationCreator) GetDeduper() *sdkdedup.Deduper {
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
	// Get max cache size from env, default from config constants
	maxSize := config.DefaultDedupMaxSize
	if sizeStr := os.Getenv("DEDUP_MAX_SIZE"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			maxSize = s
		}
	}
	return &ObservationCreator{
		dynClient:           dynClient,
		eventGVR:            eventGVR,
		metrics:             NewMetricsTracker(eventsTotal, observationsCreated, observationsFiltered, observationsDeduped, observationsCreateErrors),
		deduper:             sdkdedup.NewDeduper(windowSeconds, maxSize),
		filter:              filter,
		optimizationMetrics: optimizationMetrics,
		currentOrder:        make(map[string]ProcessingOrder),
		smartProcessor:      optimization.NewSmartProcessor(), // Created here, can be shared with optimizer
		fieldExtractor:      NewFieldExtractor(),
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

// SetGVRResolver sets a function to resolve GVR from source name.
// This allows dynamic GVR selection based on destination configuration.
func (oc *ObservationCreator) SetGVRResolver(resolver func(source string) schema.GroupVersionResource) {
	oc.gvrResolver = resolver
}

// SetGVRAllowlist sets the GVR allowlist for write restrictions
// H037: Enforces namespace + allowlist restrictions
func (oc *ObservationCreator) SetGVRAllowlist(allowlist *GVRAllowlist) {
	oc.gvrAllowlist = allowlist
}

// CreateObservation creates an Observation CRD and increments metrics
// IMPORTANT: This method assumes the event has already been processed (filtered and deduplicated)
// by the Processor. It does NOT re-run filter or dedup logic.
// The canonical pipeline order is enforced in Processor.ProcessEvent:
// source → (filter | dedup, order chosen by optimization) → normalize → CreateObservation
func (oc *ObservationCreator) CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error {
	// Extract source early for metrics (optimized with field extractor and type assertion)
	sourceVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		// Optimize: use type assertion first, fallback to formatting only when needed
		if str, ok := sourceVal.(string); ok {
			source = str
		} else {
			source = fmt.Sprintf("%v", sourceVal)
		}
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

//nolint:unused // May be used for future processing order determination
func (oc *ObservationCreator) determineProcessingOrder(source string) ProcessingOrder {
	return ProcessingOrderFilterFirst
}

// getSourceMetrics gets current metrics for a source from optimization metrics
//
//nolint:unused // May be used for future metrics analysis
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
		FilteredCount:         0, // Filtered events are tracked via observationsFiltered metric
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
		if err := unstructured.SetNestedMap(observation.Object, metadata, "metadata"); err != nil {
			observationLogger.Warn("Failed to set metadata",
				sdklog.Operation("ensure_metadata"),
				sdklog.Error(err))
		}
	}

	// Extract category, severity, and eventType from spec for metrics BEFORE creation (optimized)
	category, severity, eventType := oc.extractMetricsFields(observation, observationLogger, source)

	// CRITICAL: Normalize severity and eventType in the observation spec before creation
	// This ensures validation passes even if the observation was created/modified elsewhere
	if severity != "" {
		normalizedSeverity := normalizeSeverity(severity)
		if err := unstructured.SetNestedField(observation.Object, normalizedSeverity, "spec", "severity"); err != nil {
			observationLogger.Warn("Failed to normalize severity in observation spec",
				sdklog.Operation("normalize_severity"),
				sdklog.String("source", source),
				sdklog.Error(err))
		} else {
			severity = normalizedSeverity // Update for metrics
		}
	}
	if eventType != "" {
		normalizedEventType := normalizeEventType(eventType)
		if err := unstructured.SetNestedField(observation.Object, normalizedEventType, "spec", "eventType"); err != nil {
			observationLogger.Warn("Failed to normalize eventType in observation spec",
				sdklog.Operation("normalize_event_type"),
				sdklog.String("source", source),
				sdklog.Error(err))
		} else {
			eventType = normalizedEventType // Update for metrics
		}
	}

	// Set TTL in spec if not already set (Kubernetes native style)
	oc.setTTLIfNotSet(observation)

	// STEP 1: CREATE RESOURCE (any GVR) FIRST (before metrics)
	// Resolve GVR (use resolver if available, otherwise use default)
	gvr := oc.eventGVR
	if oc.gvrResolver != nil {
		gvr = oc.gvrResolver(source)
	}

	// H037: Pre-validate GVR and namespace before creating writer (defense in depth)
	// This prevents malicious/buggy gvrResolver from attempting writes to unsafe GVRs
	if oc.gvrAllowlist != nil {
		if err := oc.gvrAllowlist.IsAllowed(gvr, namespace); err != nil {
			// Blocked at routing gate - don't attempt Kubernetes write
			oc.handleCreationError(err, source, gvr.Resource, 0)
			observationLogger.Warn("GVR write blocked at routing gate",
				sdklog.Operation("observation_create_blocked"),
				sdklog.String("source", source),
				sdklog.String("gvr", gvr.String()),
				sdklog.String("namespace", namespace),
				sdklog.Error(err))
			return fmt.Errorf("GVR write blocked at routing gate: %w", err)
		}
	}

	// Use CRDCreator for generic GVR support (works with any resource type)
	// H037: Pass allowlist to enforce GVR write restrictions (second layer of defense)
	crdCreator := NewCRDCreator(oc.dynClient, gvr, oc.gvrAllowlist)
	deliveryStartTime := time.Now()
	err := crdCreator.CreateCRD(ctx, observation)
	deliveryDuration := time.Since(deliveryStartTime)

	// For metrics, use observation (name will be generated by Kubernetes)
	createdObservation := observation

	if err != nil {
		oc.handleCreationError(err, source, gvr.Resource, deliveryDuration)
		return fmt.Errorf("failed to create resource %s: %w", gvr.Resource, err)
	}

	// Track successful destination delivery
	if oc.destinationMetrics != nil {
		oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "success").Inc()
		oc.destinationMetrics.DestinationDeliveryLatency.WithLabelValues(source, "crd").Observe(deliveryDuration.Seconds())
	}

	// STEP 2: UPDATE METRICS ONLY AFTER SUCCESSFUL CREATION
	oc.updateMetricsAfterCreation(observation, source, category, severity, eventType, namespace, createdObservation, observationLogger)

	// STEP 3: LOG SUCCESS
	observationLogger.Debug("Observation created successfully",
		sdklog.Operation("observation_create"),
		sdklog.String("source", source),
		sdklog.String("namespace", namespace),
		sdklog.String("observation_name", createdObservation.GetName()),
		sdklog.String("category", category),
		sdklog.String("severity", severity))

	return nil
}

// extractMetricsFields extracts category, severity, and eventType from observation
func (oc *ObservationCreator) extractMetricsFields(observation *unstructured.Unstructured, logger *sdklog.Logger, source string) (string, string, string) {
	categoryVal, categoryFound := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "category")
	severityVal, severityFound := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "severity")
	eventTypeVal, eventTypeFound := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "eventType")

	category := ""
	if categoryVal != nil {
		// Optimize: use type assertion first, fallback to formatting only when needed
		if str, ok := categoryVal.(string); ok {
			category = str
		} else {
			category = fmt.Sprintf("%v", categoryVal)
		}
	} else if !categoryFound {
		observationLogger.Debug("Category not found in spec",
			sdklog.Operation("observation_create"),
			sdklog.String("source", source))
	}

	severity := ""
	if severityVal != nil {
		// Optimize: use type assertion first, fallback to formatting only when needed
		if str, ok := severityVal.(string); ok {
			severity = str
		} else {
			severity = fmt.Sprintf("%v", severityVal)
		}
	} else if !severityFound {
		observationLogger.Debug("Severity not found in spec",
			sdklog.Operation("observation_create"),
			sdklog.String("source", source))
	}

	eventType := ""
	if eventTypeVal != nil {
		// Optimize: use type assertion first, fallback to formatting only when needed
		if str, ok := eventTypeVal.(string); ok {
			eventType = str
		} else {
			eventType = fmt.Sprintf("%v", eventTypeVal)
		}
	} else if !eventTypeFound {
		observationLogger.Debug("EventType not found in spec",
			sdklog.Operation("observation_create"),
			sdklog.String("source", source))
	}

	// Normalize severity to uppercase for consistency
	if severity != "" {
		severity = normalizeSeverity(severity)
	}

	return category, severity, eventType
}

// handleCreationError handles observation creation errors
// Security: Tracks security policy violations separately
func (oc *ObservationCreator) handleCreationError(err error, source, resource string, deliveryDuration time.Duration) {
	// Check if this is a security policy violation (allowlist denial)
	isSecurityViolation := errors.Is(err, ErrGVRNotAllowed) ||
		errors.Is(err, ErrGVRDenied) ||
		errors.Is(err, ErrNamespaceNotAllowed) ||
		errors.Is(err, ErrClusterScopedNotAllowed)

	// Track creation errors
	if oc.metrics != nil && oc.metrics.ObservationsCreateErrors != nil {
		errorType := classifyError(err)
		oc.metrics.ObservationsCreateErrors.WithLabelValues(source, errorType).Inc()
	}

	// Track destination delivery failure
	if oc.destinationMetrics != nil {
		if isSecurityViolation {
			// Security policy violation - track as "not_allowed"
			oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "not_allowed").Inc()
		} else {
			// Regular failure
			oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "failure").Inc()
			if deliveryDuration > 0 {
				oc.destinationMetrics.DestinationDeliveryLatency.WithLabelValues(source, "crd").Observe(deliveryDuration.Seconds())
			}
		}
	}
}

// updateMetricsAfterCreation updates all metrics after successful observation creation
func (oc *ObservationCreator) updateMetricsAfterCreation(observation *unstructured.Unstructured, source, category, severity, eventType, namespace string, createdObservation *unstructured.Unstructured, logger *sdklog.Logger) {
	// Increment observations created metric
	if oc.metrics != nil && oc.metrics.ObservationsCreated != nil {
		oc.metrics.ObservationsCreated.WithLabelValues(source).Inc()
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

	// Increment eventsTotal metric
	oc.updateEventsTotalMetric(observation, source, category, severity, eventType, namespace, createdObservation, logger)
}

// updateEventsTotalMetric updates the eventsTotal metric
func (oc *ObservationCreator) updateEventsTotalMetric(observation *unstructured.Unstructured, source, category, severity, eventType, namespace string, createdObservation *unstructured.Unstructured, logger *sdklog.Logger) {
	if oc.metrics == nil || oc.metrics.EventsTotal == nil {
		observationLogger.Error(fmt.Errorf("eventsTotal metric is nil"), "eventsTotal metric is nil, metrics will not be incremented",
			sdklog.Operation("observation_create"),
			sdklog.String("source", source))
		return
	}

	// Normalize values
	if category == "" {
		category = "unknown"
	}
	if severity == "" {
		severity = "info"
	}
	if eventType == "" {
		eventType = "unknown"
	}

	// Extract namespace and kind from resource
	resourceNamespace, resourceKind := oc.extractResourceInfo(observation, namespace)

	// Get processing strategy for this source
	strategy := oc.getProcessingStrategy(source)

	oc.metrics.EventsTotal.WithLabelValues(source, category, severity, eventType, resourceNamespace, resourceKind, strategy).Inc()
	logger.Debug("Metric incremented after observation creation",
		sdklog.Operation("observation_create"),
		sdklog.String("source", source),
		sdklog.String("category", category),
		sdklog.String("severity", severity),
		sdklog.String("eventType", eventType),
		sdklog.String("observation_name", createdObservation.GetName()),
		sdklog.String("observation_namespace", namespace),
		sdklog.String("resource_namespace", resourceNamespace),
		sdklog.String("resource_kind", resourceKind))
}

// extractResourceInfo extracts namespace and kind from resource
func (oc *ObservationCreator) extractResourceInfo(observation *unstructured.Unstructured, defaultNamespace string) (string, string) {
	resourceNamespace := defaultNamespace
	resourceKind := ""
	resourceVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec", "resource")
	if resourceVal != nil {
		if ns, ok := resourceVal["namespace"].(string); ok && ns != "" {
			resourceNamespace = ns
		} else if ns := resourceVal["namespace"]; ns != nil {
			resourceNamespace = fmt.Sprintf("%v", ns)
		}
		if k, ok := resourceVal["kind"].(string); ok && k != "" {
			resourceKind = k
		} else if k := resourceVal["kind"]; k != nil {
			resourceKind = fmt.Sprintf("%v", k)
		}
	}
	if resourceNamespace == "" {
		resourceNamespace = "default"
	}
	if resourceKind == "" {
		resourceKind = "Unknown"
	}
	return resourceNamespace, resourceKind
}

// getProcessingStrategy gets the processing strategy for a source
func (oc *ObservationCreator) getProcessingStrategy(source string) string {
	strategy := "filter_first"
	oc.orderMu.RLock()
	if order, exists := oc.currentOrder[source]; exists {
		strategy = string(order)
	}
	oc.orderMu.RUnlock()
	return strategy
}

// extractDedupKey extracts minimal metadata for deduplication from observation
//
//nolint:unused // May be used for future deduplication logic
func (oc *ObservationCreator) extractDedupKey(observation *unstructured.Unstructured) sdkdedup.DedupKey {
	// Extract source (optimized with type assertion)
	sourceVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		// Optimize: use type assertion first, fallback to formatting only when needed
		if str, ok := sourceVal.(string); ok {
			source = str
		} else {
			source = fmt.Sprintf("%v", sourceVal)
		}
	}

	// Extract resource info (optimized with helper function)
	resourceVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec", "resource")
	namespace := extractStringField(resourceVal, "namespace")
	kind := extractStringField(resourceVal, "kind")
	name := extractStringField(resourceVal, "name")
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
		reason = oc.extractReasonFromDetails(detailsVal)
	}
	// Fallback to eventType if no reason found (optimized)
	if reason == "" {
		eventTypeVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "eventType")
		if eventTypeVal != nil {
			// Optimize: use type assertion first, fallback to formatting only when needed
			if str, ok := eventTypeVal.(string); ok {
				reason = str
			} else {
				reason = fmt.Sprintf("%v", eventTypeVal)
			}
		}
	}

	// Extract message for hashing
	message := oc.extractMessage(detailsVal)

	// Hash message
	messageHash := sdkdedup.HashMessage(message)

	return sdkdedup.DedupKey{
		Source:      source,
		Namespace:   namespace,
		Kind:        kind,
		Name:        name,
		Reason:      reason,
		MessageHash: messageHash,
	}
}

// extractStringField extracts a string field from a map with type assertion optimization
// Note: Used in extractDedupKey (which may be unused but kept for future use)
//
//nolint:unused // Used in extractDedupKey which may be used in future
func extractStringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	val, ok := m[key]
	if !ok {
		return ""
	}
	// Optimize: use type assertion first, fallback to formatting only when needed
	if str, ok := val.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", val)
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

	// TTL validation bounds (use config constants)
	MinTTLSeconds := config.DefaultTTLMinSeconds
	MaxTTLSeconds := config.DefaultTTLMaxSeconds

	// Check for seconds first (more precise)
	if ttlSecondsStr := os.Getenv("OBSERVATION_TTL_SECONDS"); ttlSecondsStr != "" {
		if ttlSeconds, err := strconv.ParseInt(ttlSecondsStr, 10, 64); err == nil && ttlSeconds > 0 {
			defaultTTLSeconds = ttlSeconds
		} else {
			observationLogger.Warn("Failed to parse OBSERVATION_TTL_SECONDS, using default",
				sdklog.Operation("ttl_parsing"),
				sdklog.String("value", ttlSecondsStr),
				sdklog.Error(err))
		}
	} else if ttlDaysStr := os.Getenv("OBSERVATION_TTL_DAYS"); ttlDaysStr != "" {
		// Fallback to days (for backward compatibility)
		if ttlDays, err := strconv.Atoi(ttlDaysStr); err == nil && ttlDays > 0 {
			defaultTTLSeconds = int64(ttlDays) * 24 * 60 * 60
		} else {
			observationLogger.Warn("Failed to parse OBSERVATION_TTL_DAYS, using default",
				sdklog.Operation("ttl_parsing"),
				sdklog.String("value", ttlDaysStr),
				sdklog.Error(err))
		}
	}

	// Validate TTL bounds
	if defaultTTLSeconds < MinTTLSeconds {
		observationLogger.Warn("TTL value too small, using minimum",
			sdklog.Operation("ttl_validation"),
			sdklog.Int64("requested_ttl", defaultTTLSeconds),
			sdklog.Int64("minimum_ttl", MinTTLSeconds),
			sdklog.Int64("applied_ttl", MinTTLSeconds))
		defaultTTLSeconds = MinTTLSeconds
	} else if defaultTTLSeconds > MaxTTLSeconds {
		observationLogger.Warn("TTL value too large, using maximum",
			sdklog.Operation("ttl_validation"),
			sdklog.Int64("requested_ttl", defaultTTLSeconds),
			sdklog.Int64("maximum_ttl", MaxTTLSeconds),
			sdklog.Int64("applied_ttl", MaxTTLSeconds))
		defaultTTLSeconds = MaxTTLSeconds
	}

	// Ensure spec exists (optimized)
	spec, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
		if err := unstructured.SetNestedMap(observation.Object, spec, "spec"); err != nil {
			observationLogger.Warn("Failed to set spec",
				sdklog.Operation("set_ttl"),
				sdklog.Error(err))
			return
		}
	}

	// Set TTL in spec (only if not already set)
	spec["ttlSecondsAfterCreation"] = defaultTTLSeconds
	if err := unstructured.SetNestedMap(observation.Object, spec, "spec"); err != nil {
		observationLogger.Warn("Failed to set spec with TTL",
			sdklog.Operation("set_ttl"),
			sdklog.Error(err))
	}
}

// getOrCreateSourceCounters gets or creates source counters, returns nil if metrics disabled
func (oc *ObservationCreator) getOrCreateSourceCounters(source string) *sourceCounters {
	if oc.optimizationMetrics == nil {
		return nil
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
	return oc.optimizationMetrics.sourceCounters[source]
}

// recordEventAttempted records that an event was attempted
func (oc *ObservationCreator) recordEventAttempted(source string) {
	counters := oc.getOrCreateSourceCounters(source)
	if counters != nil {
		counters.attempted++
	}
}

// recordEventFiltered records that an event was filtered
//
//nolint:unused // May be used for future metrics tracking
func (oc *ObservationCreator) recordEventFiltered(source, reason string) {
	_ = oc.getOrCreateSourceCounters(source)
	// Filtered events are tracked via observationsFiltered metric
}

// recordEventDeduped records that an event was deduplicated
//
//nolint:unused // May be used for future metrics tracking
func (oc *ObservationCreator) recordEventDeduped(source string) {
	counters := oc.getOrCreateSourceCounters(source)
	if counters != nil {
		counters.deduped++
	}
}

// recordEventCreated records that an event was created
func (oc *ObservationCreator) recordEventCreated(source, severity string) {
	counters := oc.getOrCreateSourceCounters(source)
	if counters != nil {
		counters.created++
		counters.totalCount++

		// Track low severity
		severityLower := strings.ToLower(severity)
		if severityLower == "low" || severityLower == "info" {
			counters.lowSeverity++
		}
	}

	// Update severity distribution (separate from counters)
	if oc.optimizationMetrics != nil && oc.optimizationMetrics.SeverityDistribution != nil {
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

// extractReasonFromDetails extracts reason from details map
// nolint:unused // Kept for future use
func (oc *ObservationCreator) extractReasonFromDetails(detailsVal map[string]interface{}) string {
	reasonFields := []string{"reason", "rule", "testNumber", "checkId", "vulnerabilityID", "auditID"}
	for _, field := range reasonFields {
		if r, ok := detailsVal[field].(string); ok && r != "" {
			return r
		}
		if r := detailsVal[field]; r != nil {
			return fmt.Sprintf("%v", r)
		}
	}
	return ""
}

// extractMessage extracts message for hashing
// nolint:unused // Kept for future use
func (oc *ObservationCreator) extractMessage(detailsVal map[string]interface{}) string {
	if detailsVal == nil {
		return ""
	}
	messageFields := []string{"message", "output"}
	for _, field := range messageFields {
		if msg, ok := detailsVal[field].(string); ok && msg != "" {
			return msg
		}
		if msg := detailsVal[field]; msg != nil {
			return fmt.Sprintf("%v", msg)
		}
	}
	return ""
}
