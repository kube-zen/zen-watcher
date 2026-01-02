// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may Obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package processor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/hooks"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"github.com/kube-zen/zen-watcher/pkg/monitoring"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Processor processes RawEvents through the pipeline
type Processor struct {
	genericThresholdMonitor *monitoring.GenericThresholdMonitor
	filter                  *filter.Filter
	deduper                 *dedup.Deduper
	observationCreator      *watcher.ObservationCreator
	metrics                 *metrics.Metrics // Optional metrics
}

// NewProcessor creates a new event processor
func NewProcessor(
	filter *filter.Filter,
	deduper *dedup.Deduper,
	observationCreator *watcher.ObservationCreator,
) *Processor {
	return NewProcessorWithMetrics(filter, deduper, observationCreator, nil)
}

// NewProcessorWithMetrics creates a new event processor with metrics
func NewProcessorWithMetrics(
	filter *filter.Filter,
	deduper *dedup.Deduper,
	observationCreator *watcher.ObservationCreator,
	m *metrics.Metrics,
) *Processor {
	return &Processor{
		genericThresholdMonitor: monitoring.NewGenericThresholdMonitor(),
		filter:                  filter,
		deduper:                 deduper,
		observationCreator:      observationCreator,
		metrics:                 m,
	}
}

// ProcessEvent processes a RawEvent through the canonical pipeline:
// source → (filter | dedup, ordered dynamically by optimization) → normalize → destinations[]
//
// The canonical order is:
// 1. Threshold check (rate limits, warnings)
// 2. Determine processing order (filter_first or dedup_first) - BEFORE any processing
// 3. Filter and Dedup (order chosen by optimization engine: filter_first or dedup_first)
// 4. Normalization (after filter/dedup, prepares data for destinations)
// 5. Create Observation (write to destinations)
func (p *Processor) ProcessEvent(ctx context.Context, raw *generic.RawEvent, config *generic.SourceConfig) error {
	// Step 1: Check thresholds (rate limits, warnings)
	// Note: Thresholds are warnings only - they log but don't block events
	// Rate limiting is the only thing that blocks events
	if p.genericThresholdMonitor != nil {
		allowed := p.genericThresholdMonitor.CheckEvent(raw, config)
		if !allowed {
			// Rate limited - drop event
			return nil
		}
	}

	// Step 2: Determine processing order (filter_first or dedup_first) - BEFORE any processing
	// This decision must happen before any filter/dedup/normalization steps
	order := p.determineProcessingOrder(raw, config)

	// Step 3: Create minimal Observation structure for filter/dedup (no normalization yet)
	// We need a basic structure to run filter/dedup, but full normalization happens after
	observation := p.createMinimalObservation(raw, config)
	if observation == nil {
		return nil // Could not create observation
	}

	// Step 4: Apply filter and dedup in the order determined by optimization
	// Both filter and dedup are always applied; optimization chooses which runs first.
	// IMPORTANT: Normalization has NOT happened yet - we're working with minimal observation structure
	var filtered bool
	var deduped bool

	if order == "dedup_first" {
		// Dedup first, then filter
		dedupKey := p.extractDedupKey(observation, raw)
		shouldCreate := p.shouldCreateWithStrategy(dedupKey, observation.Object, config)
		if !shouldCreate {
			logger := sdklog.NewLogger("zen-watcher-processor")
			logger.Debug("Event deduplicated",
				sdklog.Operation("dedup"),
				sdklog.String("source", raw.Source))
			deduped = true
		}

		// Then filter (even if deduped, we still check filter for metrics)
		if !deduped && p.filter != nil {
			allowed, reason := p.filter.AllowWithReason(observation)
			if !allowed {
				logger := sdklog.NewLogger("zen-watcher-processor")
				logger.Debug("Event filtered",
					sdklog.Operation("filter"),
					sdklog.String("source", raw.Source),
					sdklog.String("reason", reason))
				filtered = true
			}
		}
	} else {
		// Filter first (default), then dedup
		if p.filter != nil {
			allowed, reason := p.filter.AllowWithReason(observation)
			if !allowed {
				logger := sdklog.NewLogger("zen-watcher-processor")
				logger.Debug("Event filtered",
					sdklog.Operation("filter"),
					sdklog.String("source", raw.Source),
					sdklog.String("reason", reason))
				filtered = true
			}
		}

		// Then dedup (even if filtered, we still check dedup for metrics)
		if !filtered {
			dedupKey := p.extractDedupKey(observation, raw)
			shouldCreate := p.shouldCreateWithStrategy(dedupKey, observation.Object, config)
			if !shouldCreate {
				logger := sdklog.NewLogger("zen-watcher-processor")
				logger.Debug("Event deduplicated",
					sdklog.Operation("dedup"),
					sdklog.String("source", raw.Source))
				deduped = true
			}
		}
	}

	// If filtered or deduped, stop here (no normalization needed)
	if filtered || deduped {
		return nil
	}

	// Step 5: Normalization (AFTER filter/dedup, before destinations)
	// This is the first and only place where full normalization happens
	observation = p.normalizeObservation(observation, raw, config)

	// Step 5.5: Execute hooks (post-normalization, pre-CRD write)
	if err := hooks.Processor(ctx, observation); err != nil {
		// Hook errors prevent Observation from being written
		return err
	}

	// Step 6: Create Observation (write to destinations)
	// ObservationCreator should NOT decide order - it receives already-processed events
	return p.observationCreator.CreateObservation(ctx, observation)
}

// determineProcessingOrder determines the processing order based on config and optimization
// Returns "filter_first" or "dedup_first" based on:
// 1. Explicit config order (if set)
// 2. Default to "filter_first"
// Note: Auto-optimization has been removed. Only manual order selection is supported.
func (p *Processor) determineProcessingOrder(raw *generic.RawEvent, config *generic.SourceConfig) string {
	// Check if order is explicitly set in config
	if config != nil && config.Processing != nil && config.Processing.Order != "" {
		order := config.Processing.Order
		// Support filter_first and dedup_first only
		if order == "filter_first" || order == "dedup_first" {
			return order
		}
	}

	// Default to filter_first
	return "filter_first"
}

// createMinimalObservation creates a minimal Observation structure for filter/dedup
// This does NOT perform normalization - that happens after filter/dedup
// However, we need to extract basic fields (like severity) from raw data so filters can work
func (p *Processor) createMinimalObservation(raw *generic.RawEvent, config *generic.SourceConfig) *unstructured.Unstructured {
	// Extract severity from raw data if available (for filtering)
	severity := "MEDIUM" // Default
	if raw.RawData != nil {
		if sev, ok := raw.RawData["severity"].(string); ok && sev != "" {
			severity = strings.ToUpper(sev)
		} else if sev, ok := raw.RawData["priority"].(string); ok && sev != "" {
			// Some sources use "priority" instead of "severity"
			severity = strings.ToUpper(sev)
		}
	}

	// Create a minimal event structure with just source and raw data
	// This is sufficient for filter/dedup operations
	event := &watcher.Event{
		Source:   raw.Source,
		Category: "security",  // Default, will be normalized later
		Severity: severity,    // Extract from raw data for filtering
		Details:  raw.RawData, // Preserve all raw data
	}

	// Convert to Observation (minimal structure)
	return watcher.EventToObservation(event)
}

// normalizeObservation performs full normalization after filter/dedup, before destinations
// This is the ONLY place where normalization happens
func (p *Processor) normalizeObservation(observation *unstructured.Unstructured, raw *generic.RawEvent, config *generic.SourceConfig) *unstructured.Unstructured {
	normalizeStartTime := time.Now()
	source := raw.Source

	// Convert observation back to Event for normalization
	event := p.observationToEvent(observation, raw, config)

	// Apply normalization config
	if config != nil && config.Normalization != nil {
		event.Category = config.Normalization.Domain
		event.EventType = config.Normalization.Type

		// Extract priority and map to severity
		priorityStart := time.Now()
		priority := p.extractPriority(raw, config)
		event.Severity = p.priorityToSeverity(priority)
		if p.metrics != nil {
			// Track priority mapping (simplified - track as field mapping)
			p.metrics.FilterDecisions.WithLabelValues(source, "normalization", "priority_mapping").Inc()
		}
		_ = priorityStart // Track if needed

		// Apply field mappings
		if config.Normalization.FieldMapping != nil {
			for _, mapping := range config.Normalization.FieldMapping {
				mappingStart := time.Now()
				value := p.extractField(raw.RawData, mapping.From)
				if value != nil {
					if event.Details == nil {
						event.Details = make(map[string]interface{})
					}
					event.Details[mapping.To] = value
					if p.metrics != nil {
						// Track successful field mapping
						// Note: We'd need a new metric for field mapping transformations
						// For now, use FilterDecisions as a placeholder
						p.metrics.FilterDecisions.WithLabelValues(source, "field_mapping", "success").Inc()
					}
				} else {
					if p.metrics != nil {
						// Track failed field extraction
						p.metrics.FilterDecisions.WithLabelValues(source, "field_mapping", "extraction_failed").Inc()
					}
				}
				_ = mappingStart // Track duration if needed
			}
		}
	} else {
		// Default normalization
		priority := p.extractPriority(raw, config)
		event.Severity = p.priorityToSeverity(priority)
		event.Category = "security"
		event.EventType = "custom-event"
		if p.metrics != nil {
			p.metrics.FilterDecisions.WithLabelValues(source, "normalization", "default").Inc()
		}
	}

	// Track total normalization duration
	if p.metrics != nil {
		// Note: We'd need a new metric for normalization duration
		// For now, normalization is fast enough that we don't need separate tracking
		_ = normalizeStartTime
	}

	// Convert back to Observation
	return watcher.EventToObservation(event)
}

// observationToEvent converts an Observation back to Event for normalization
func (p *Processor) observationToEvent(observation *unstructured.Unstructured, raw *generic.RawEvent, config *generic.SourceConfig) *watcher.Event {
	// Extract fields from observation
	event := &watcher.Event{
		Source:  raw.Source,
		Details: raw.RawData, // Always preserve raw data
	}

	// Try to extract from observation if available
	if category, ok, _ := unstructured.NestedString(observation.Object, "spec", "category"); ok {
		event.Category = category
	}
	if severity, ok, _ := unstructured.NestedString(observation.Object, "spec", "severity"); ok {
		event.Severity = severity
	}
	if eventType, ok, _ := unstructured.NestedString(observation.Object, "spec", "eventType"); ok {
		event.EventType = eventType
	}

	return event
}

// normalize converts RawEvent to Event using normalization config
func (p *Processor) normalize(raw *generic.RawEvent, config *generic.SourceConfig) *watcher.Event {
	if config.Normalization == nil {
		// Default normalization
		return &watcher.Event{
			Source:    raw.Source,
			Category:  "security", // Default
			Severity:  "MEDIUM",   // Default
			EventType: "custom-event",
			Details:   raw.RawData, // Preserve all raw data
		}
	}

	event := &watcher.Event{
		Source:    raw.Source,
		Category:  config.Normalization.Domain,
		EventType: config.Normalization.Type,
		Details:   raw.RawData, // Preserve ALL raw data
	}

	// Extract priority from raw data and map it
	priority := p.extractPriority(raw, config)

	// Map priority to severity
	event.Severity = p.priorityToSeverity(priority)

	// Apply field mappings
	if config.Normalization.FieldMapping != nil {
		for _, mapping := range config.Normalization.FieldMapping {
			value := p.extractField(raw.RawData, mapping.From)
			if value != nil {
				// Add to details or labels
				if event.Details == nil {
					event.Details = make(map[string]interface{})
				}
				event.Details[mapping.To] = value
			}
		}
	}

	return event
}

// extractPriority extracts priority from raw data
func (p *Processor) extractPriority(raw *generic.RawEvent, config *generic.SourceConfig) float64 {
	if config == nil || config.Normalization == nil || len(config.Normalization.Priority) == 0 {
		return 0.5 // Default
	}

	// Try to find priority in raw data
	// This is simplified - would use JSONPath in production
	for key, value := range raw.RawData {
		if mapped, exists := config.Normalization.Priority[fmt.Sprintf("%v", value)]; exists {
			return mapped
		}
		if mapped, exists := config.Normalization.Priority[key]; exists {
			return mapped
		}
	}

	return 0.5 // Default
}

// priorityToSeverity converts priority (0.0-1.0) to severity string
func (p *Processor) priorityToSeverity(priority float64) string {
	if priority >= 0.9 {
		return "CRITICAL"
	} else if priority >= 0.7 {
		return "HIGH"
	} else if priority >= 0.4 {
		return "MEDIUM"
	} else if priority >= 0.2 {
		return "LOW"
	}
	return "INFO"
}

// extractField extracts a field from raw data using JSONPath (simplified)
func (p *Processor) extractField(data map[string]interface{}, path string) interface{} {
	// Simplified JSONPath extraction
	// In production, would use a proper JSONPath library
	parts := splitPath(path)
	current := data
	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	return nil
}

// splitPath splits a JSONPath-like path
func splitPath(path string) []string {
	// Simplified - just split by "."
	return strings.Split(path, ".")
}

// extractDedupKey extracts deduplication key from observation and raw event
func (p *Processor) extractDedupKey(observation *unstructured.Unstructured, raw *generic.RawEvent) dedup.DedupKey {
	// Generate fingerprint from raw data
	rawEvent := watcher.RawEvent{
		Source: raw.Source,
		Type:   "", // Would extract from metadata
		Data:   raw.RawData,
	}
	fingerprint := watcher.GenerateFingerprint(rawEvent)

	// Hash the fingerprint for dedup key
	hash := sha256.Sum256([]byte(fingerprint))
	return dedup.DedupKey{
		Source:      raw.Source,
		MessageHash: fmt.Sprintf("%x", hash[:16]),
	}
}

// shouldCreateWithStrategy determines if an observation should be created using the configured dedup strategy
func (p *Processor) shouldCreateWithStrategy(key dedup.DedupKey, content map[string]interface{}, config *generic.SourceConfig) bool {
	// Get strategy from config, default to "fingerprint" if not set
	strategyName := "fingerprint"
	if config != nil && config.Dedup != nil {
		if config.Dedup.Strategy != "" {
			strategyName = config.Dedup.Strategy
		}
	}

	// Build strategy config from generic config
	strategyConfig := dedup.StrategyConfig{
		Strategy:           strategyName,
		Window:             "",
		Fields:             nil,
		MaxEventsPerWindow: 0,
	}
	if config != nil && config.Dedup != nil {
		strategyConfig.Window = config.Dedup.Window
		strategyConfig.Fields = config.Dedup.Fields
		strategyConfig.MaxEventsPerWindow = config.Dedup.MaxEventsPerWindow
	}

	// Get strategy from registry
	strategy := dedup.GetStrategy(strategyConfig)

	// Use strategy to determine if we should create
	return strategy.ShouldCreate(p.deduper, key, content)
}
