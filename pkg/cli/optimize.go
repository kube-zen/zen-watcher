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

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kube-zen/zen-watcher/pkg/advisor"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/client-go/dynamic"
)

// OptimizeCLI provides CLI commands for optimization
type OptimizeCLI struct {
	dynClient          dynamic.Interface
	sourceConfigLoader *config.SourceConfigLoader
	advisor            *advisor.Advisor
}

// NewOptimizeCLI creates a new optimization CLI
func NewOptimizeCLI(dynClient dynamic.Interface, sourceConfigLoader *config.SourceConfigLoader, advisor *advisor.Advisor) *OptimizeCLI {
	return &OptimizeCLI{
		dynClient:          dynClient,
		sourceConfigLoader: sourceConfigLoader,
		advisor:            advisor,
	}
}

// Analyze analyzes a source and shows optimization suggestions
func (cli *OptimizeCLI) Analyze(ctx context.Context, source string) error {
	logger.Info("Analyzing optimization opportunities",
		logger.Fields{
			Component: "cli",
			Operation: "analyze",
			Source:    source,
		})

	// Get source config
	var sourceConfig *config.SourceConfig
	if cli.sourceConfigLoader != nil {
		sourceConfig = cli.sourceConfigLoader.GetSourceConfig(source)
	}

	// Get current metrics (simplified - would query Prometheus in production)
	// For now, show config-based analysis

	fmt.Printf("\n=== Optimization Analysis for Source: %s ===\n\n", source)

	if sourceConfig == nil {
		fmt.Printf("No configuration found for source '%s'. Using defaults.\n\n", source)
	} else {
		fmt.Printf("Current Configuration:\n")
		fmt.Printf("  Processing Order: %s\n", sourceConfig.ProcessingOrder)
		fmt.Printf("  Auto Optimize: %v\n", sourceConfig.AutoOptimize)
		fmt.Printf("  Filter Min Priority: %.2f\n", sourceConfig.FilterMinPriority)
		fmt.Printf("  Dedup Window: %v\n", sourceConfig.DedupWindow)
		fmt.Printf("\n")
	}

	// Show suggestions (would come from advisor in production)
	fmt.Printf("Optimization Suggestions:\n")
	fmt.Printf("  (Run with --apply to see actionable suggestions)\n")
	fmt.Printf("\n")

	// Show impact if available
	if cli.advisor != nil {
		impact := cli.advisor.GetImpact(source)
		if impact.OptimizationsApplied > 0 {
			fmt.Printf("Past Optimizations:\n")
			fmt.Printf("  Applied: %d\n", impact.OptimizationsApplied)
			fmt.Printf("  Observations Reduced: %d\n", impact.ObservationsReduced)
			fmt.Printf("  Reduction: %.1f%%\n", impact.ReductionPercent*100)
			fmt.Printf("  Most Effective: %s\n", impact.MostEffective)
			fmt.Printf("\n")
		}
	}

	return nil
}

// Apply applies a suggestion by index
func (cli *OptimizeCLI) Apply(ctx context.Context, source string, suggestionIndex int) error {
	logger.Info("Applying optimization suggestion",
		logger.Fields{
			Component: "cli",
			Operation: "apply",
			Source:    source,
			Additional: map[string]interface{}{
				"suggestion_index": suggestionIndex,
			},
		})

	// In production, this would:
	// 1. Get suggestions from advisor
	// 2. Select suggestion by index
	// 3. Execute the kubectl command or patch CRD directly

	fmt.Printf("Applying suggestion #%d for source '%s'...\n", suggestionIndex, source)
	fmt.Printf("(This would execute the kubectl command from the suggestion)\n")

	return nil
}

// Auto enables auto-optimization for all sources
func (cli *OptimizeCLI) Auto(ctx context.Context, enable bool) error {
	action := "disable"
	if enable {
		action = "enable"
	}

	logger.Info("Auto-optimization toggle",
		logger.Fields{
			Component: "cli",
			Operation: "auto",
			Additional: map[string]interface{}{
				"enable": enable,
			},
		})

	fmt.Printf("Auto-optimization: %s\n", action)
	fmt.Printf("(This would update ObservationSourceConfig CRDs to set autoOptimize=%v)\n", enable)

	return nil
}

// History shows optimization history for a source
func (cli *OptimizeCLI) History(ctx context.Context, source string) error {
	logger.Info("Showing optimization history",
		logger.Fields{
			Component: "cli",
			Operation: "history",
			Source:    source,
		})

	fmt.Printf("\n=== Optimization History for Source: %s ===\n\n", source)

	if cli.advisor == nil {
		fmt.Printf("Advisor not available.\n")
		return nil
	}

	impact := cli.advisor.GetImpact(source)

	if impact.OptimizationsApplied == 0 {
		fmt.Printf("No optimizations applied yet.\n")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "Metric\tValue\n")
	fmt.Fprintf(w, "---\t---\n")
	fmt.Fprintf(w, "Optimizations Applied\t%d\n", impact.OptimizationsApplied)
	fmt.Fprintf(w, "Observations Reduced\t%d\n", impact.ObservationsReduced)
	fmt.Fprintf(w, "Reduction Percentage\t%.1f%%\n", impact.ReductionPercent*100)
	fmt.Fprintf(w, "CPU Savings (minutes)\t%.1f\n", impact.CPUSavingsMinutes)
	fmt.Fprintf(w, "Most Effective\t%s\n", impact.MostEffective)
	if !impact.LastOptimizedAt.IsZero() {
		fmt.Fprintf(w, "Last Optimized\t%s\n", impact.LastOptimizedAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()

	return nil
}

// ListSources lists all sources with their optimization status
func (cli *OptimizeCLI) ListSources(ctx context.Context) error {
	fmt.Printf("\n=== Available Sources ===\n\n")

	if cli.sourceConfigLoader == nil {
		fmt.Printf("Source config loader not available.\n")
		return nil
	}

	allConfigs := cli.sourceConfigLoader.GetAllSourceConfigs()

	if len(allConfigs) == 0 {
		fmt.Printf("No source configurations found.\n")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "Source\tProcessing Order\tAuto Optimize\tMin Priority\tDedup Window\n")
	fmt.Fprintf(w, "---\t---\t---\t---\t---\n")

	for source, config := range allConfigs {
		autoOptimize := "false"
		if config.AutoOptimize {
			autoOptimize = "true"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%v\n",
			source,
			config.ProcessingOrder,
			autoOptimize,
			config.FilterMinPriority,
			config.DedupWindow,
		)
	}
	w.Flush()

	return nil
}
