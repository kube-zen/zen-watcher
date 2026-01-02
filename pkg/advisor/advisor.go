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
	"sync"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Advisor coordinates optimization analysis, suggestions, and impact tracking
type Advisor struct {
	metricsAnalyzer   *MetricsAnalyzer
	suggestionEngine  *SuggestionEngine
	impactTracker     *ImpactTracker
	analysisInterval  time.Duration
	suggestionHandler func(Suggestion)
	mu                sync.RWMutex
	running           bool
}

// NewAdvisor creates a new optimization advisor
func NewAdvisor(metricsAnalyzer *MetricsAnalyzer, suggestionEngine *SuggestionEngine, impactTracker *ImpactTracker) *Advisor {
	return &Advisor{
		metricsAnalyzer:  metricsAnalyzer,
		suggestionEngine: suggestionEngine,
		impactTracker:    impactTracker,
		analysisInterval: 15 * time.Minute, // Default: analyze every 15 minutes
		running:          false,
	}
}

// SetAnalysisInterval sets the interval for optimization analysis
func (a *Advisor) SetAnalysisInterval(interval time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.analysisInterval = interval
}

// OnSuggestion registers a callback for when suggestions are generated
func (a *Advisor) OnSuggestion(handler func(Suggestion)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.suggestionHandler = handler
}

// Start begins the optimization advisor loop
func (a *Advisor) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = true
	a.mu.Unlock()

	logger := sdklog.NewLogger("zen-watcher-advisor")
	logger.Info("Optimization advisor started",
		sdklog.Operation("advisor_start"),
		sdklog.Float64("analysis_interval_minutes", a.analysisInterval.Minutes()))

	ticker := time.NewTicker(a.analysisInterval)
	defer ticker.Stop()

	// Run initial analysis
	a.runAnalysis(ctx)

	for {
		select {
		case <-ctx.Done():
			a.mu.Lock()
			a.running = false
			a.mu.Unlock()
			logger := sdklog.NewLogger("zen-watcher-advisor")
			logger.Info("Optimization advisor stopped",
				sdklog.Operation("advisor_stop"))
			return ctx.Err()
		case <-ticker.C:
			a.runAnalysis(ctx)
		}
	}
}

// runAnalysis performs a full optimization analysis cycle
func (a *Advisor) runAnalysis(ctx context.Context) {
	logger := sdklog.NewLogger("zen-watcher-advisor")
	logger.Debug("Starting optimization analysis",
		sdklog.Operation("analysis_start"))

	// Step 1: Analyze metrics to find opportunities
	opportunities, err := a.metricsAnalyzer.Analyze(ctx)
	if err != nil {
		logger.Error(err, "Failed to analyze metrics",
			sdklog.Operation("analysis_metrics"))
		return
	}

	if len(opportunities) == 0 {
		logger.Debug("No optimization opportunities found",
			sdklog.Operation("analysis_complete"))
		return
	}

	// Step 2: Generate suggestions from opportunities
	suggestions := a.suggestionEngine.GenerateSuggestions(opportunities)

	// Step 3: Process suggestions (log, emit, track)
	for _, suggestion := range suggestions {
		a.processSuggestion(suggestion)
	}

	// Step 4: Log summary
	a.logSummary(opportunities, suggestions)

	logger.Debug("Optimization analysis complete",
		sdklog.Operation("analysis_complete"),
		sdklog.Int("opportunities", len(opportunities)),
		sdklog.Int("suggestions", len(suggestions)))
}

// processSuggestion handles a generated suggestion
func (a *Advisor) processSuggestion(suggestion Suggestion) {
	// Track suggestion generation
	a.impactTracker.RecordSuggestion(suggestion)

	// Call registered handler if confidence is high enough
	a.mu.RLock()
	handler := a.suggestionHandler
	a.mu.RUnlock()

	if handler != nil && suggestion.Confidence >= 0.7 {
		handler(suggestion)
	}
}

// logSummary logs a periodic optimization summary
func (a *Advisor) logSummary(opportunities []Opportunity, suggestions []Suggestion) {
	if len(suggestions) == 0 {
		return
	}

	// Group by source
	sourceStats := make(map[string]map[string]interface{})
	for _, opp := range opportunities {
		if sourceStats[opp.Source] == nil {
			sourceStats[opp.Source] = make(map[string]interface{})
		}
		// Aggregate stats per source
	}

	logger := sdklog.NewLogger("zen-watcher-advisor")
	logger.Info("Optimization summary",
		sdklog.Operation("optimization_summary"),
		sdklog.Int("opportunities", len(opportunities)),
		sdklog.Int("suggestions", len(suggestions)))
}

// ApplySuggestion applies a suggestion
func (a *Advisor) ApplySuggestion(ctx context.Context, suggestion Suggestion) error {
	logger := sdklog.NewLogger("zen-watcher-advisor")
	logger.Info("Applying suggestion",
		sdklog.Operation("apply_suggestion"),
		sdklog.String("source", suggestion.Source),
		sdklog.String("type", suggestion.Type),
		sdklog.Float64("confidence", suggestion.Confidence),
		sdklog.String("command", suggestion.Command))

	// Track application
	a.impactTracker.RecordApplication(suggestion)

	// Apply the suggestion (this would typically patch the CRD)
	// For now, we just log - actual implementation would use Kubernetes client
	return nil
}

// GetImpact returns optimization impact metrics
func (a *Advisor) GetImpact(source string) *ImpactMetrics {
	return a.impactTracker.GetImpact(source)
}
