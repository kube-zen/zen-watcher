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
)

// Optimizer is the main optimization coordinator that ties everything together
type Optimizer struct {
	engine              *OptimizationEngine
	smartProcessor      *SmartProcessor
	stateManager        *OptimizationStateManager
	adaptiveProcessors  map[string]*AdaptiveProcessor
	sourceConfigLoader  interface {
		GetSourceConfig(source string) *config.SourceConfig
		GetAllSourceConfigs() map[string]*config.SourceConfig
	}
	
	mu sync.RWMutex
}

// NewOptimizer creates a new optimizer with all components
func NewOptimizer(
	sourceConfigLoader interface {
		GetSourceConfig(source string) *config.SourceConfig
		GetAllSourceConfigs() map[string]*config.SourceConfig
	},
) *Optimizer {
	smartProcessor := NewSmartProcessor()
	return NewOptimizerWithProcessor(smartProcessor, sourceConfigLoader)
}

// NewOptimizerWithProcessor creates a new optimizer with a shared SmartProcessor
func NewOptimizerWithProcessor(
	smartProcessor *SmartProcessor,
	sourceConfigLoader interface {
		GetSourceConfig(source string) *config.SourceConfig
		GetAllSourceConfigs() map[string]*config.SourceConfig
	},
) *Optimizer {
	stateManager := NewOptimizationStateManager()
	engine := NewOptimizationEngine(smartProcessor, stateManager, sourceConfigLoader)

	return &Optimizer{
		engine:             engine,
		smartProcessor:     smartProcessor,
		stateManager:       stateManager,
		adaptiveProcessors: make(map[string]*AdaptiveProcessor),
		sourceConfigLoader: sourceConfigLoader,
	}
}

// Start starts the optimizer
func (o *Optimizer) Start(ctx context.Context) error {
	return o.engine.Start(ctx)
}

// Stop stops the optimizer
func (o *Optimizer) Stop() {
	o.engine.Stop()
}

// GetSmartProcessor returns the smart processor
func (o *Optimizer) GetSmartProcessor() *SmartProcessor {
	return o.smartProcessor
}

// GetStateManager returns the state manager
func (o *Optimizer) GetStateManager() *OptimizationStateManager {
	return o.stateManager
}

// GetEngine returns the optimization engine
func (o *Optimizer) GetEngine() *OptimizationEngine {
	return o.engine
}

// GetOrCreateAdaptiveProcessor gets or creates an adaptive processor for a source
func (o *Optimizer) GetOrCreateAdaptiveProcessor(source string) *AdaptiveProcessor {
	o.mu.Lock()
	defer o.mu.Unlock()

	if processor, exists := o.adaptiveProcessors[source]; exists {
		return processor
	}

	var sourceConfig *config.SourceConfig
	if o.sourceConfigLoader != nil {
		sourceConfig = o.sourceConfigLoader.GetSourceConfig(source)
	}

	if sourceConfig == nil {
		return nil
	}

	metricsCollector := o.smartProcessor.GetOrCreateMetricsCollector(source)
	performanceTracker := o.smartProcessor.GetOrCreatePerformanceTracker(source)

	processor := NewAdaptiveProcessor(
		source,
		sourceConfig,
		metricsCollector,
		performanceTracker,
	)

	o.adaptiveProcessors[source] = processor
	return processor
}

// TriggerAdaptation triggers adaptation for a specific source
func (o *Optimizer) TriggerAdaptation(source string) error {
	processor := o.GetOrCreateAdaptiveProcessor(source)
	if processor == nil {
		return nil // Source not configured for optimization
	}

	if !processor.ShouldAdapt() {
		return nil // Not ready for adaptation
	}

	return processor.Adapt()
}

// GetOptimizationStatus returns optimization status for a source
func (o *Optimizer) GetOptimizationStatus(source string) *OptimizationStatus {
	return o.engine.GetOptimizationStatus(source)
}

