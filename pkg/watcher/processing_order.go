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
	"github.com/kube-zen/zen-watcher/pkg/config"
)

// ProcessingOrder represents the processing order strategy
type ProcessingOrder string

const (
	ProcessingOrderAuto        ProcessingOrder = "auto"
	ProcessingOrderFilterFirst ProcessingOrder = "filter_first"
	ProcessingOrderDedupFirst  ProcessingOrder = "dedup_first"
)

// DetermineOptimalOrder determines the optimal processing order based on source config and metrics
func DetermineOptimalOrder(source string, sourceConfig *config.SourceConfig, metrics *SourceMetrics) ProcessingOrder {
	// If source config specifies a non-auto order, use it
	if sourceConfig != nil && sourceConfig.ProcessingOrder != "" && sourceConfig.ProcessingOrder != "auto" {
		return ProcessingOrder(sourceConfig.ProcessingOrder)
	}

	// Auto-optimization logic
	if metrics == nil {
		return GetDefaultOrder(source)
	}

	// Rule 1: If high LOW severity (>70%), filter_first
	if metrics.LowSeverityPercent > 0.7 {
		return ProcessingOrderFilterFirst
	}

	// Rule 2: If high dedup effectiveness (>50%), dedup_first
	if metrics.DedupEffectiveness > 0.5 {
		return ProcessingOrderDedupFirst
	}

	// Rule 3: Default based on source type
	return GetDefaultOrder(source)
}

// GetDefaultOrder returns the default processing order for a source
func GetDefaultOrder(source string) ProcessingOrder {
	// Default: filter first (configurable via Ingester CRD)
	// Source-specific defaults are configured via YAML, not hardcoded
	return ProcessingOrderFilterFirst
}

// SourceMetrics represents metrics for a source (used for order determination)
type SourceMetrics struct {
	Source                string
	ObservationsPerMinute float64
	LowSeverityPercent    float64
	DedupEffectiveness    float64
	FilterPassRate        float64
	TotalObservations     int64
	FilteredCount         int64
	DedupedCount          int64
	CreatedCount          int64
}
