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

package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
)

// ThresholdMonitor monitors optimization thresholds and triggers alerts
type ThresholdMonitor struct {
	sourceConfigLoader interface {
		GetSourceConfig(source string) *generic.SourceConfig
	}
	thresholdExceeded *prometheus.CounterVec
	checkInterval     time.Duration
	mu                sync.RWMutex
	lastChecks        map[string]time.Time
}

// NewThresholdMonitor creates a new threshold monitor
func NewThresholdMonitor(
	sourceConfigLoader interface {
		GetSourceConfig(source string) *generic.SourceConfig
	},
	thresholdExceeded *prometheus.CounterVec,
) *ThresholdMonitor {
	return &ThresholdMonitor{
		sourceConfigLoader: sourceConfigLoader,
		thresholdExceeded:  thresholdExceeded,
		checkInterval:      1 * time.Minute, // Check every minute
		lastChecks:         make(map[string]time.Time),
	}
}

// Start begins monitoring thresholds
func (tm *ThresholdMonitor) Start(ctx context.Context) error {
	logger := sdklog.NewLogger("zen-watcher-monitoring")
	logger.Info("Threshold monitor started",
		sdklog.Operation("threshold_monitor_start"),
		sdklog.Float64("check_interval_seconds", tm.checkInterval.Seconds()))

	ticker := time.NewTicker(tm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger := sdklog.NewLogger("zen-watcher-monitoring")
			logger.Info("Threshold monitor stopped",
				sdklog.Operation("threshold_monitor_stop"))
			return ctx.Err()
		case <-ticker.C:
			// Check thresholds periodically
			// This would query metrics and check against thresholds
			// For now, this is a placeholder - actual implementation would query Prometheus
		}
	}
}

// CheckThreshold checks if a threshold is exceeded for a source
// Returns true if event should be allowed (not rate limited), false if rate limited
func (tm *ThresholdMonitor) CheckThreshold(source string, metricName string, value float64, warningThreshold, criticalThreshold float64) bool {
	if tm.sourceConfigLoader == nil {
		return true // Allow if no config loader
	}

	sourceConfig := tm.sourceConfigLoader.GetSourceConfig(source)
	if sourceConfig == nil {
		return true // Allow if no config
	}

	// Check rate limiting first (if configured)
	if sourceConfig.RateLimit != nil && sourceConfig.RateLimit.ObservationsPerMinute > 0 {
		// Rate limiting would be checked here
		// For now, always allow (rate limiting would need per-source counters)
	}

	// Check thresholds and log warnings (but don't block)
	severity := ""
	if value >= criticalThreshold {
		severity = "critical"
		tm.recordThresholdExceeded(source, metricName, severity)
		logger := sdklog.NewLogger("zen-watcher-monitoring")
		logger.Warn("Critical threshold exceeded",
			sdklog.Operation("threshold_exceeded"),
			sdklog.String("source", source),
			sdklog.String("metric", metricName),
			sdklog.Float64("value", value),
			sdklog.Float64("critical_threshold", criticalThreshold),
			sdklog.String("severity", severity),
			sdklog.String("message", "Threshold exceeded - this is a warning only, event will still be processed"))
	} else if value >= warningThreshold {
		severity = "warning"
		tm.recordThresholdExceeded(source, metricName, severity)
		logger := sdklog.NewLogger("zen-watcher-monitoring")
		logger.Warn("Warning threshold exceeded",
			sdklog.Operation("threshold_exceeded"),
			sdklog.String("source", source),
			sdklog.String("metric", metricName),
			sdklog.Float64("value", value),
			sdklog.Float64("warning_threshold", warningThreshold),
			sdklog.String("severity", severity),
			sdklog.String("message", "Threshold exceeded - this is a warning only, event will still be processed"))
	}

	// Always allow (thresholds are warnings only, not blockers)
	return true
}

// recordThresholdExceeded records a threshold exceedance in metrics
func (tm *ThresholdMonitor) recordThresholdExceeded(source, threshold, severity string) {
	if tm.thresholdExceeded != nil {
		tm.thresholdExceeded.WithLabelValues(source, threshold, severity).Inc()
	}
}

// CheckObservationRate checks observation rate threshold
func (tm *ThresholdMonitor) CheckObservationRate(source string, observationsPerMinute float64) {
	// Default thresholds
	warningThreshold := 100.0
	criticalThreshold := 200.0

	if tm.sourceConfigLoader != nil {
		sourceConfig := tm.sourceConfigLoader.GetSourceConfig(source)
		if sourceConfig != nil {
			// Get thresholds from config (would need to add to SourceConfig)
			// For now, use defaults
		}
	}

	tm.CheckThreshold(source, "observations_per_minute", observationsPerMinute, warningThreshold, criticalThreshold)
}

// CheckLowSeverityPercent checks low severity percentage threshold
func (tm *ThresholdMonitor) CheckLowSeverityPercent(source string, lowSeverityPercent float64) {
	// Default thresholds
	warningThreshold := 0.7  // 70%
	criticalThreshold := 0.9 // 90%

	if tm.sourceConfigLoader != nil {
		sourceConfig := tm.sourceConfigLoader.GetSourceConfig(source)
		if sourceConfig != nil {
			// Get thresholds from config
		}
	}

	tm.CheckThreshold(source, "low_severity_percent", lowSeverityPercent, warningThreshold, criticalThreshold)
}

// CheckDedupEffectiveness checks dedup effectiveness threshold
func (tm *ThresholdMonitor) CheckDedupEffectiveness(source string, dedupEffectiveness float64) {
	// Default thresholds (lower is worse)
	warningThreshold := 0.3  // <30% effectiveness = warning
	criticalThreshold := 0.1 // <10% effectiveness = critical

	// Invert for comparison (we check if effectiveness is BELOW threshold)
	if dedupEffectiveness < criticalThreshold {
		tm.CheckThreshold(source, "dedup_effectiveness", dedupEffectiveness, criticalThreshold, criticalThreshold)
	} else if dedupEffectiveness < warningThreshold {
		tm.CheckThreshold(source, "dedup_effectiveness", dedupEffectiveness, warningThreshold, criticalThreshold)
	}
}
