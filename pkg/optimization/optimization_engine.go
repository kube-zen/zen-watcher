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

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// OptimizationEngine orchestrates the optimization process for all sources
type OptimizationEngine struct {
	smartProcessor     *SmartProcessor
	stateManager       *OptimizationStateManager
	sourceConfigLoader interface {
		GetSourceConfig(source string) *generic.SourceConfig
		GetAllSourceConfigs() map[string]*generic.SourceConfig
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
		GetSourceConfig(source string) *generic.SourceConfig
		GetAllSourceConfigs() map[string]*generic.SourceConfig
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

	logger := sdklog.NewLogger("zen-watcher-optimization")
	logger.Info("Optimization engine started",
		sdklog.Operation("engine_start"),
		sdklog.Duration("interval", oe.optimizationInterval))

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

	logger := sdklog.NewLogger("zen-watcher-optimization")
	logger.Info("Optimization engine stopped",
		sdklog.Operation("engine_stop"))
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
			logger := sdklog.NewLogger("zen-watcher-optimization")
			logger.Info("Optimization loop received shutdown signal",
				sdklog.Operation("loop_shutdown"))
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

	// Note: Auto-optimization has been removed.
	// This optimization loop is no longer active.
	// Processing order is now configured manually via config.Processing.Order
	// Skip all sources since auto-optimization is disabled
	for range allConfigs {
		continue
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
