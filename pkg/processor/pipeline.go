// Copyright 2024 The Zen Watcher Authors
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

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/kube-zen/zen-watcher/pkg/logger"
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
}

// NewProcessor creates a new event processor
func NewProcessor(
	filter *filter.Filter,
	deduper *dedup.Deduper,
	observationCreator *watcher.ObservationCreator,
) *Processor {
	return &Processor{
		genericThresholdMonitor: monitoring.NewGenericThresholdMonitor(),
		filter:                  filter,
		deduper:                 deduper,
		observationCreator:     observationCreator,
	}
}

// ProcessEvent processes a RawEvent through the pipeline:
// 1. Threshold check (rate limits, warnings)
// 2. Filtering
// 3. Normalization
// 4. Deduplication
// 5. Create Observation
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

	// Step 2: Normalize raw event to Event
	event := p.normalize(raw, config)
	if event == nil {
		return nil // Filtered out during normalization
	}

	// Step 3: Convert Event to Observation
	observation := watcher.EventToObservation(event)

	// Step 4: Apply filters
	if p.filter != nil {
		allowed, reason := p.filter.AllowWithReason(observation)
		if !allowed {
			logger.Debug("Event filtered",
				logger.Fields{
					Component: "processor",
					Operation: "filter",
					Source:    raw.Source,
					Reason:    reason,
				})
			return nil
		}
	}

	// Step 5: Check deduplication
	dedupKey := p.extractDedupKey(observation, raw)
	if !p.deduper.ShouldCreateWithContent(dedupKey, observation.Object) {
		logger.Debug("Event deduplicated",
			logger.Fields{
				Component: "processor",
				Operation: "dedup",
				Source:    raw.Source,
			})
		return nil
	}

	// Step 6: Create Observation
	return p.observationCreator.CreateObservation(ctx, observation)
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
	if config.Normalization == nil || len(config.Normalization.Priority) == 0 {
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
