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

package advisor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// MetricsAnalyzer analyzes Prometheus metrics to find optimization opportunities
type MetricsAnalyzer struct {
	promClient v1.API
	sources    []string
	mu         sync.RWMutex
}

// NewMetricsAnalyzer creates a new metrics analyzer
func NewMetricsAnalyzer(promClient v1.API, sources []string) *MetricsAnalyzer {
	return &MetricsAnalyzer{
		promClient: promClient,
		sources:    sources,
	}
}

// Analyze analyzes metrics and returns optimization opportunities
func (ma *MetricsAnalyzer) Analyze(ctx context.Context) ([]Opportunity, error) {
	opportunities := make([]Opportunity, 0)

	// Analyze each source
	for _, source := range ma.sources {
		sourceOpps, err := ma.analyzeSource(ctx, source)
		if err != nil {
			logger.Debug("Failed to analyze source",
				logger.Fields{
					Component: "advisor",
					Operation: "analyze_source",
					Source:    source,
					Error:     err,
				})
			continue
		}
		opportunities = append(opportunities, sourceOpps...)
	}

	return opportunities, nil
}

// analyzeSource analyzes metrics for a specific source
func (ma *MetricsAnalyzer) analyzeSource(ctx context.Context, source string) ([]Opportunity, error) {
	opportunities := make([]Opportunity, 0)

	// Get source metrics
	metrics, err := ma.getSourceMetrics(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics for source %s: %w", source, err)
	}

	// Rule 1: High LOW severity ratio (>70%)
	if metrics.LowSeverityPercent > 0.7 {
		confidence := metrics.LowSeverityPercent
		opportunities = append(opportunities, Opportunity{
			Source:     source,
			Type:       "high_low_severity",
			Severity:   determineSeverity(metrics.LowSeverityPercent, 0.7, 0.9),
			Confidence: confidence,
			Metrics: map[string]interface{}{
				"low_severity_percent": metrics.LowSeverityPercent,
			},
			Description: fmt.Sprintf("%.0f%% of observations are LOW severity", metrics.LowSeverityPercent*100),
		})
	}

	// Rule 2: Low dedup effectiveness (<30%)
	if metrics.DedupEffectiveness < 0.3 && metrics.DedupedCount > 0 {
		confidence := 1.0 - metrics.DedupEffectiveness // Higher confidence if effectiveness is lower
		opportunities = append(opportunities, Opportunity{
			Source:     source,
			Type:       "low_dedup_effectiveness",
			Severity:   determineSeverity(1.0-metrics.DedupEffectiveness, 0.7, 0.9),
			Confidence: confidence,
			Metrics: map[string]interface{}{
				"dedup_effectiveness": metrics.DedupEffectiveness,
				"deduped_count":       metrics.DedupedCount,
			},
			Description: fmt.Sprintf("Deduplication effectiveness is only %.0f%%", metrics.DedupEffectiveness*100),
		})
	}

	// Rule 3: High observation rate (>100/min)
	if metrics.ObservationsPerMinute > 100 {
		severity := "medium"
		if metrics.ObservationsPerMinute > 200 {
			severity = "high"
		}
		confidence := 0.8 // High confidence for rate issues
		opportunities = append(opportunities, Opportunity{
			Source:     source,
			Type:       "high_observation_rate",
			Severity:   severity,
			Confidence: confidence,
			Metrics: map[string]interface{}{
				"observations_per_minute": metrics.ObservationsPerMinute,
			},
			Description: fmt.Sprintf("Creating %.0f observations/minute (threshold: 100/min)", metrics.ObservationsPerMinute),
		})
	}

	// Rule 4: Low filter pass rate (<20%) - filter is too aggressive
	if metrics.FilterPassRate < 0.2 && metrics.FilteredCount > 0 {
		confidence := 0.7
		opportunities = append(opportunities, Opportunity{
			Source:     source,
			Type:       "low_filter_pass_rate",
			Severity:   "medium",
			Confidence: confidence,
			Metrics: map[string]interface{}{
				"filter_pass_rate": metrics.FilterPassRate,
				"filtered_count":   metrics.FilteredCount,
			},
			Description: fmt.Sprintf("Filter pass rate is only %.0f%% (may be too aggressive)", metrics.FilterPassRate*100),
		})
	}

	return opportunities, nil
}

// getSourceMetrics retrieves metrics for a source from Prometheus
func (ma *MetricsAnalyzer) getSourceMetrics(ctx context.Context, source string) (*SourceMetrics, error) {
	metrics := &SourceMetrics{
		Source: source,
	}

	// Query Prometheus for metrics
	// Note: This is a simplified implementation - in production, use proper Prometheus queries

	// Example queries (simplified - actual implementation would use PromQL):
	// - zen_watcher_observations_per_minute{source="<source-name>"}
	// - zen_watcher_low_severity_percent{source="<source-name>"}
	// - zen_watcher_dedup_effectiveness{source="<source-name>"}
	// - zen_watcher_filter_pass_rate{source="<source-name>"}

	// For now, return empty metrics - actual implementation would query Prometheus
	// This is a placeholder that will be implemented when metrics are added
	_ = ma.promClient // Use promClient to query

	return metrics, nil
}

// determineSeverity determines severity based on value and thresholds
func determineSeverity(value, warningThreshold, criticalThreshold float64) string {
	if value >= criticalThreshold {
		return "high"
	}
	if value >= warningThreshold {
		return "medium"
	}
	return "low"
}

// SetPromClient sets the Prometheus client (for dependency injection)
func (ma *MetricsAnalyzer) SetPromClient(client v1.API) {
	ma.mu.Lock()
	defer ma.mu.Unlock()
	ma.promClient = client
}

// QueryPrometheus is a helper to query Prometheus (placeholder for actual implementation)
func (ma *MetricsAnalyzer) QueryPrometheus(ctx context.Context, query string) (model.Value, error) {
	if ma.promClient == nil {
		return nil, fmt.Errorf("prometheus client not initialized")
	}
	result, warnings, err := ma.promClient.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		logger.Debug("Prometheus query warnings",
			logger.Fields{
				Component: "advisor",
				Operation: "prometheus_query",
				Additional: map[string]interface{}{
					"warnings": warnings,
				},
			})
	}
	return result, nil
}
