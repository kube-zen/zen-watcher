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

package monitoring

import (
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"golang.org/x/time/rate"
)

// GenericThresholdMonitor monitors thresholds for generic adapters
// Thresholds are warnings only - they log but don't block events
type GenericThresholdMonitor struct {
	mu               sync.RWMutex
	rateLimiters     map[string]*rate.Limiter // source -> rate limiter
	observationRates map[string]*rateCounter  // source -> observation rate counter
}

type rateCounter struct {
	count     int
	window    time.Duration
	lastReset time.Time
	mu        sync.Mutex
}

// NewGenericThresholdMonitor creates a new generic threshold monitor
func NewGenericThresholdMonitor() *GenericThresholdMonitor {
	return &GenericThresholdMonitor{
		rateLimiters:     make(map[string]*rate.Limiter),
		observationRates: make(map[string]*rateCounter),
	}
}

// CheckEvent checks thresholds for a raw event and logs warnings if exceeded
// Returns true if event should be processed, false if rate limited
func (gtm *GenericThresholdMonitor) CheckEvent(raw *generic.RawEvent, config *generic.SourceConfig) bool {
	if config == nil {
		return true
	}

	// Check rate limiting
	if config.RateLimit != nil && config.RateLimit.ObservationsPerMinute > 0 {
		if !gtm.allowRateLimit(raw.Source, config.RateLimit.ObservationsPerMinute, config.RateLimit.Burst) {
			logger := sdklog.NewLogger("zen-watcher-monitoring")
			logger.Warn("Event rate limited",
				sdklog.Operation("rate_limit"),
				sdklog.String("source", raw.Source),
				sdklog.String("message", "Event dropped due to rate limiting"))
			return false // Rate limited - drop event
		}
	}

	// Check observation rate threshold (warning only)
	if config.Thresholds != nil && config.Thresholds.ObservationsPerMinute != nil {
		rate := gtm.getObservationRate(raw.Source)
		if rate > float64(config.Thresholds.ObservationsPerMinute.Critical) {
			logger := sdklog.NewLogger("zen-watcher-monitoring")
			logger.Warn("Critical observation rate threshold exceeded",
				sdklog.Operation("threshold_warning"),
				sdklog.String("source", raw.Source),
				sdklog.Float64("rate", rate),
				sdklog.Int("critical_threshold", config.Thresholds.ObservationsPerMinute.Critical),
				sdklog.String("message", "High observation rate detected - consider adjusting filters or dedup window"))
		} else if rate > float64(config.Thresholds.ObservationsPerMinute.Warning) {
			logger := sdklog.NewLogger("zen-watcher-monitoring")
			logger.Warn("Warning observation rate threshold exceeded",
				sdklog.Operation("threshold_warning"),
				sdklog.String("source", raw.Source),
				sdklog.Float64("rate", rate),
				sdklog.Int("warning_threshold", config.Thresholds.ObservationsPerMinute.Warning),
				sdklog.String("message", "Observation rate is high - monitor for potential issues"))
		}
	}

	// Check custom thresholds
	if config.Thresholds != nil && len(config.Thresholds.Custom) > 0 {
		gtm.checkCustomThresholds(raw, config)
	}

	// Record observation for rate calculation
	gtm.recordObservation(raw.Source)

	return true // Always allow (thresholds are warnings only)
}

// allowRateLimit checks if event is within rate limit
func (gtm *GenericThresholdMonitor) allowRateLimit(source string, maxPerMinute, burst int) bool {
	gtm.mu.Lock()
	defer gtm.mu.Unlock()

	limiter, exists := gtm.rateLimiters[source]
	if !exists {
		// Create new rate limiter
		limiter = rate.NewLimiter(rate.Limit(maxPerMinute)/60.0, burst)
		gtm.rateLimiters[source] = limiter
	}

	return limiter.Allow()
}

// getObservationRate gets the current observation rate per minute for a source
func (gtm *GenericThresholdMonitor) getObservationRate(source string) float64 {
	gtm.mu.RLock()
	defer gtm.mu.RUnlock()

	counter, exists := gtm.observationRates[source]
	if !exists {
		return 0.0
	}

	counter.mu.Lock()
	defer counter.mu.Unlock()

	// Calculate rate over the window
	elapsed := time.Since(counter.lastReset)
	if elapsed < time.Minute {
		// Scale to per minute
		return float64(counter.count) * (60.0 / elapsed.Seconds())
	}

	// Reset if window expired
	counter.count = 0
	counter.lastReset = time.Now()
	return 0.0
}

// recordObservation records an observation for rate calculation
func (gtm *GenericThresholdMonitor) recordObservation(source string) {
	gtm.mu.Lock()
	defer gtm.mu.Unlock()

	counter, exists := gtm.observationRates[source]
	if !exists {
		counter = &rateCounter{
			window:    1 * time.Minute,
			lastReset: time.Now(),
		}
		gtm.observationRates[source] = counter
	}

	counter.mu.Lock()
	defer counter.mu.Unlock()

	// Reset if window expired
	if time.Since(counter.lastReset) >= counter.window {
		counter.count = 0
		counter.lastReset = time.Now()
	}

	counter.count++
}

// checkCustomThresholds checks custom thresholds against raw data
func (gtm *GenericThresholdMonitor) checkCustomThresholds(raw *generic.RawEvent, config *generic.SourceConfig) {
	for _, threshold := range config.Thresholds.Custom {
		value := gtm.extractField(raw.RawData, threshold.Field)
		if gtm.evaluateThreshold(value, threshold.Operator, threshold.Value) {
			logger := sdklog.NewLogger("zen-watcher-monitoring")
			logger.Warn("Custom threshold exceeded",
				sdklog.Operation("custom_threshold_warning"),
				sdklog.String("source", raw.Source),
				sdklog.String("threshold_name", threshold.Name),
				sdklog.String("field", threshold.Field),
				sdklog.String("value", fmt.Sprintf("%v", value)),
				sdklog.String("message", threshold.Message))
		}
	}
}

// extractField extracts a field from raw data using JSONPath (simplified)
func (gtm *GenericThresholdMonitor) extractField(data map[string]interface{}, path string) interface{} {
	// Simplified JSONPath - just split by "."
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
	result := make([]string, 0)
	current := ""
	for _, char := range path {
		if char == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// evaluateThreshold evaluates a threshold condition
func (gtm *GenericThresholdMonitor) evaluateThreshold(value interface{}, operator string, expected interface{}) bool {
	switch operator {
	case ">":
		return compareNumbers(value, expected, func(a, b float64) bool { return a > b })
	case "<":
		return compareNumbers(value, expected, func(a, b float64) bool { return a < b })
	case "==":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", expected)
	case "!=":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", expected)
	case "contains":
		valueStr := fmt.Sprintf("%v", value)
		expectedStr := fmt.Sprintf("%v", expected)
		return contains(valueStr, expectedStr)
	default:
		return false
	}
}

// compareNumbers compares two values as numbers
func compareNumbers(a, b interface{}, cmp func(float64, float64) bool) bool {
	aFloat := toFloat(a)
	bFloat := toFloat(b)
	return cmp(aFloat, bFloat)
}

// toFloat converts a value to float64
func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0.0
	}
}

// contains checks if a string contains another string
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

// findSubstring finds a substring in a string (simplified)
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
