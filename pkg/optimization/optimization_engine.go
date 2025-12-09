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

package optimization

import (
	"context"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// OptimizationEngine orchestrates the optimization process for all sources
type OptimizationEngine struct {
	smartProcessor     *SmartProcessor
	stateManager       *OptimizationStateManager
	sourceConfigLoader interface {
		GetSourceConfig(source string) *config.SourceConfig
		GetAllSourceConfigs() map[string]*config.SourceConfig
	}

	// Optimization loop control
	optimizationInterval time.Duration
	running              bool
	mu                   sync.RWMutex
	cancel               context.CancelFunc
	wg                   sync.WaitGroup // For graceful shutdown
}

// NewOptimizationEngine creates a new optimization engine
func NewOptimizationEngine(
	smartProcessor *SmartProcessor,
	stateManager *OptimizationStateManager,
	sourceConfigLoader interface {
		GetSourceConfig(source string) *config.SourceConfig
		GetAllSourceConfigs() map[string]*config.SourceConfig
	},
) *OptimizationEngine {
	config := DefaultOptimizationConfig()
	return &OptimizationEngine{
		smartProcessor:       smartProcessor,
		stateManager:         stateManager,
		sourceConfigLoader:   sourceConfigLoader,
		optimizationInterval: config.OptimizationInterval,
	}
}

// Start starts the optimization engine background loop
func (oe *OptimizationEngine) Start(ctx context.Context) error {
	oe.mu.Lock()
	defer oe.mu.Unlock()

	if oe.running {
		return nil // Already running
	}

	optimizationCtx, cancel := context.WithCancel(ctx)
	oe.cancel = cancel
	oe.running = true

	go oe.optimizationLoop(optimizationCtx)

	logger.Info("Optimization engine started",
		logger.Fields{
			Component: "optimization",
			Operation: "engine_start",
			Additional: map[string]interface{}{
				"interval": oe.optimizationInterval.String(),
			},
		})

	return nil
}

// Stop stops the optimization engine and waits for goroutines to finish
func (oe *OptimizationEngine) Stop() {
	oe.mu.Lock()
	if !oe.running {
		oe.mu.Unlock()
		return
	}

	if oe.cancel != nil {
		oe.cancel()
	}

	oe.running = false
	oe.mu.Unlock()

	// Wait for goroutines to finish
	oe.wg.Wait()

	logger.Info("Optimization engine stopped",
		logger.Fields{
			Component: "optimization",
			Operation: "engine_stop",
		})
}

// optimizationLoop runs the continuous optimization loop
func (oe *OptimizationEngine) optimizationLoop(ctx context.Context) {
	oe.wg.Add(1)
	defer oe.wg.Done()

	ticker := time.NewTicker(oe.optimizationInterval)
	defer ticker.Stop()

	// Run immediately on start
	oe.runOptimizationCycle(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Optimization loop received shutdown signal",
				logger.Fields{
					Component: "optimization",
					Operation: "loop_shutdown",
				})
			return
		case <-ticker.C:
			oe.runOptimizationCycle(ctx)
		}
	}
}

// runOptimizationCycle runs a single optimization cycle for all sources
func (oe *OptimizationEngine) runOptimizationCycle(ctx context.Context) {
	if oe.sourceConfigLoader == nil {
		return
	}

	allConfigs := oe.sourceConfigLoader.GetAllSourceConfigs()

	for source, sourceConfig := range allConfigs {
		if sourceConfig == nil || !sourceConfig.Processing.AutoOptimize {
			continue // Skip sources without auto-optimization enabled
		}

		// Get current metrics for this source
		metrics := oe.smartProcessor.GetSourceMetrics(source)
		if metrics == nil {
			continue // No metrics available yet
		}

		// Check if optimization should be triggered
		strategyDecider := NewStrategyDecider()
		if !strategyDecider.ShouldOptimize(metrics, sourceConfig) {
			continue // Thresholds not exceeded
		}

		// Determine optimal strategy
		currentStrategy := strategyDecider.DetermineStrategy(metrics, sourceConfig)
		state := oe.stateManager.GetState(source)

		// Check if strategy change is needed
		if state.CurrentStrategy != currentStrategy.String() {
			oe.applyOptimization(source, sourceConfig, currentStrategy, metrics)
		}
	}
}

// applyOptimization applies an optimization decision
func (oe *OptimizationEngine) applyOptimization(
	source string,
	sourceConfig *config.SourceConfig,
	newStrategy ProcessingStrategy,
	metrics *OptimizationMetrics,
) {
	startTime := time.Now()
	state := oe.stateManager.GetState(source)
	oldStrategy := state.CurrentStrategy

	// Check cooldown period (hysteresis) to prevent oscillation
	if !oe.canMakeDecision(source, state) {
		logger.Debug("Optimization skipped due to cooldown period",
			logger.Fields{
				Component: "optimization",
				Operation: "optimization_cooldown",
				Source:    source,
				Additional: map[string]interface{}{
					"last_decision": state.LastDecision,
				},
			})
		RecordDecision("strategy_change", "skipped_cooldown", source, time.Since(startTime), 0.0)
		return
	}

	// Calculate confidence based on metrics
	confidence := oe.calculateConfidence(metrics, sourceConfig)

	// Only apply if confidence exceeds threshold
	if confidence < sourceConfig.Processing.ConfidenceThreshold {
		logger.Debug("Optimization skipped due to low confidence",
			logger.Fields{
				Component: "optimization",
				Operation: "optimization_skipped",
				Source:    source,
				Additional: map[string]interface{}{
					"confidence":        confidence,
					"threshold":         sourceConfig.Processing.ConfidenceThreshold,
					"proposed_strategy": newStrategy.String(),
				},
			})
		RecordDecision("strategy_change", "skipped_low_confidence", source, time.Since(startTime), confidence)
		return
	}

	// Record decision
	decision := OptimizationDecision{
		Type:          "strategy_change",
		PreviousValue: oldStrategy,
		NewValue:      newStrategy.String(),
		Confidence:    confidence,
		Reason:        oe.generateReason(metrics, newStrategy),
		ImpactMetrics: oe.getImpactMetrics(metrics),
	}

	err := oe.stateManager.RecordDecision(source, decision)
	if err != nil {
		logger.Debug("Failed to record optimization decision",
			logger.Fields{
				Component: "optimization",
				Operation: "record_decision_error",
				Source:    source,
				Error:     err,
			})
		RecordDecision("strategy_change", "error", source, time.Since(startTime), confidence)
		return
	}

	// Record successful decision metrics
	RecordDecision("strategy_change", "success", source, time.Since(startTime), confidence)

	logger.Info("Optimization applied",
		logger.Fields{
			Component: "optimization",
			Operation: "optimization_applied",
			Source:    source,
			Additional: map[string]interface{}{
				"old_strategy":   oldStrategy,
				"new_strategy":   newStrategy.String(),
				"confidence":     confidence,
				"reason":         decision.Reason,
				"impact_metrics": decision.ImpactMetrics,
			},
		})
}

// canMakeDecision checks if enough time has passed since the last decision (cooldown/hysteresis)
func (oe *OptimizationEngine) canMakeDecision(source string, state *OptimizationState) bool {
	config := DefaultOptimizationConfig()
	if state.LastDecision.IsZero() {
		return true // No previous decision, allow
	}

	cooldownPeriod := config.CooldownPeriod
	return time.Since(state.LastDecision) >= cooldownPeriod
}

// calculateConfidence calculates the confidence level for an optimization decision
func (oe *OptimizationEngine) calculateConfidence(
	metrics *OptimizationMetrics,
	sourceConfig *config.SourceConfig,
) float64 {
	config := DefaultOptimizationConfig()
	confidence := 0.5 // Base confidence

	// Increase confidence based on metrics consistency
	if metrics.EventsProcessed > config.MinEventsForConfidence {
		confidence += 0.2 // More data = higher confidence
	}

	// Increase confidence if dedup effectiveness is clearly high/low
	if metrics.DeduplicationRate > config.HighDedupRate {
		confidence += 0.1 // Very high dedup rate
	} else if metrics.DeduplicationRate < config.LowDedupRate {
		confidence += 0.1 // Very low dedup rate
	}

	// Increase confidence if filter effectiveness is clearly high/low
	if metrics.FilterEffectiveness > config.HighFilterRate {
		confidence += 0.1 // Very high filter rate
	} else if metrics.FilterEffectiveness < config.LowFilterRate {
		confidence += 0.1 // Very low filter rate
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// generateReason generates a human-readable reason for the optimization decision
func (oe *OptimizationEngine) generateReason(
	metrics *OptimizationMetrics,
	strategy ProcessingStrategy,
) string {
	switch strategy {
	case ProcessingStrategyFilterFirst:
		if metrics.LowSeverityPercent > 0.7 {
			return "High low-severity percentage (>70%), filtering first to reduce noise early"
		}
		return "Filter-first strategy selected based on current metrics"

	case ProcessingStrategyDedupFirst:
		if metrics.DeduplicationRate > 0.5 {
			return "High deduplication effectiveness (>50%), deduplicating first to remove duplicates early"
		}
		return "Dedup-first strategy selected based on current metrics"

	case ProcessingStrategyHybrid:
		return "Hybrid strategy selected for variable workload patterns"

	case ProcessingStrategyAdaptive:
		return "Adaptive strategy selected for high-volume, complex workload"

	default:
		return "Strategy change based on optimization analysis"
	}
}

// getImpactMetrics extracts key metrics that indicate expected impact
func (oe *OptimizationEngine) getImpactMetrics(metrics *OptimizationMetrics) map[string]float64 {
	return map[string]float64{
		"events_processed":      float64(metrics.EventsProcessed),
		"filter_effectiveness":  metrics.FilterEffectiveness,
		"dedup_effectiveness":   metrics.DeduplicationRate,
		"low_severity_percent":  metrics.LowSeverityPercent,
		"observations_per_min":  metrics.ObservationsPerMinute,
		"processing_latency_ms": float64(metrics.ProcessingLatency),
	}
}

// GetOptimizationStatus returns the current optimization status for a source
func (oe *OptimizationEngine) GetOptimizationStatus(source string) *OptimizationStatus {
	state := oe.stateManager.GetState(source)
	metrics := oe.smartProcessor.GetSourceMetrics(source)

	return &OptimizationStatus{
		Source:          source,
		CurrentStrategy: state.CurrentStrategy,
		LastDecision:    state.LastDecision,
		Metrics:         metrics,
		DecisionCount:   len(state.DecisionHistory),
	}
}

// OptimizationStatus represents the current optimization status for a source
type OptimizationStatus struct {
	Source          string
	CurrentStrategy string
	LastDecision    time.Time
	Metrics         *OptimizationMetrics
	DecisionCount   int
}
