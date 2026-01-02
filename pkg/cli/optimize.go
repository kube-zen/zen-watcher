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

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/advisor"
	"k8s.io/client-go/dynamic"
)

// OptimizeCLI provides CLI commands for optimization
type OptimizeCLI struct {
	dynClient dynamic.Interface
	advisor   *advisor.Advisor
}

// NewOptimizeCLI creates a new optimization CLI
func NewOptimizeCLI(dynClient dynamic.Interface, advisor *advisor.Advisor) *OptimizeCLI {
	return &OptimizeCLI{
		dynClient: dynClient,
		advisor:   advisor,
	}
}

// Analyze analyzes a source and shows optimization suggestions
func (cli *OptimizeCLI) Analyze(ctx context.Context, source string) error {
	logger := sdklog.NewLogger("zen-watcher-cli")
	logger.Info("Analyzing optimization opportunities",
		sdklog.Operation("analyze"),
		sdklog.String("source", source))

	// Get current metrics (simplified - would query Prometheus in production)
	// For now, show config-based analysis

	fmt.Printf("\n=== Optimization Analysis for Source: %s ===\n\n", source)
	fmt.Printf("Configuration analysis would be loaded from Ingester CRDs.\n")
	fmt.Printf("(Source config loader removed - use IngesterConfig instead)\n\n")

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
	logger := sdklog.NewLogger("zen-watcher-cli")
	logger.Info("Applying optimization suggestion",
		sdklog.Operation("apply"),
		sdklog.String("source", source),
		sdklog.Int("suggestion_index", suggestionIndex))

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
	fmt.Printf("Auto-optimization has been removed.\n")
	fmt.Printf("Configure processing order manually in Ingester CRD using spec.processing.order:\n")
	fmt.Printf("  - filter_first: For high LOW severity (>70%%)\n")
	fmt.Printf("  - dedup_first: For high duplicate rate (>50%%)\n")
	return nil
}

// History shows optimization history for a source
func (cli *OptimizeCLI) History(ctx context.Context, source string) error {
	logger := sdklog.NewLogger("zen-watcher-cli")
	logger.Info("Showing optimization history",
		sdklog.Operation("history"),
		sdklog.String("source", source))

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
	fmt.Printf("Source listing would query Ingester CRDs.\n")
	fmt.Printf("(Source config loader removed - use IngesterConfig instead)\n")
	return nil
}
