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

package filter

import (
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	sdkfilter "github.com/kube-zen/zen-sdk/pkg/filter"
)

// MetricsAdapter adapts zen-watcher metrics to zen-sdk FilterMetrics interface
type MetricsAdapter struct {
	m *metrics.Metrics
}

// NewMetricsAdapter creates a new metrics adapter
func NewMetricsAdapter(m *metrics.Metrics) sdkfilter.FilterMetrics {
	if m == nil {
		return nil
	}
	return &MetricsAdapter{m: m}
}

// RecordFilterDecision records a filter decision
func (a *MetricsAdapter) RecordFilterDecision(source, decision, reason string) {
	if a == nil || a.m == nil || a.m.FilterDecisions == nil {
		return
	}
	a.m.FilterDecisions.WithLabelValues(source, decision, reason).Inc()
}

// RecordEvaluationDuration records the time taken to evaluate a filter rule
func (a *MetricsAdapter) RecordEvaluationDuration(source, ruleType string, durationSeconds float64) {
	if a == nil || a.m == nil || a.m.FilterRuleEvaluationDuration == nil {
		return
	}
	a.m.FilterRuleEvaluationDuration.WithLabelValues(source, ruleType).Observe(durationSeconds)
}

