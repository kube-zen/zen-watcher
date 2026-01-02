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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/internal/kubernetes"
	"github.com/kube-zen/zen-watcher/pkg/cli"
)

func main() {
	var (
		command    = flag.String("command", "", "DEPRECATED: Auto-optimization CLI removed. Use Ingester CRD to configure processing order.")
		source     = flag.String("source", "", "Source name (required for analyze, apply, history)")
		suggestion = flag.Int("suggestion", 0, "Suggestion index (required for apply)")
		enable     = flag.Bool("enable", false, "DEPRECATED: Auto-optimization has been removed")
		logLevel   = flag.String("log-level", "INFO", "Log level")
	)
	flag.Parse()

	// Set log level via environment variable (zen-sdk reads LOG_LEVEL)
	if *logLevel != "" {
		os.Setenv("LOG_LEVEL", *logLevel)
	}
	// zen-sdk logger initializes automatically, no explicit Init() needed
	_ = sdklog.NewLogger("zen-watcher-optimize")

	// Initialize Kubernetes clients
	clients, err := kubernetes.NewClients()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Kubernetes clients: %v\n", err)
		os.Exit(1)
	}

	// Create CLI (advisor would be created if Prometheus client available)
	// Note: SourceConfigLoader removed - optimization now works with IngesterConfig directly
	optimizeCLI := cli.NewOptimizeCLI(clients.Dynamic, nil)

	ctx := context.Background()

	// Execute command
	if err := executeCommand(ctx, optimizeCLI, *command, *source, *suggestion, *enable); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// executeCommand executes the specified command
func executeCommand(ctx context.Context, optimizeCLI *cli.OptimizeCLI, command, source string, suggestion int, enable bool) error {
	switch command {
	case "analyze":
		return executeAnalyze(ctx, optimizeCLI, source)
	case "apply":
		return executeApply(ctx, optimizeCLI, source, suggestion)
	case "auto":
		return optimizeCLI.Auto(ctx, enable)
	case "history":
		return executeHistory(ctx, optimizeCLI, source)
	case "list":
		return optimizeCLI.ListSources(ctx)
	default:
		printUsage()
		os.Exit(1)
		return nil
	}
}

// executeAnalyze executes the analyze command
func executeAnalyze(ctx context.Context, optimizeCLI *cli.OptimizeCLI, source string) error {
	if source == "" {
		return fmt.Errorf("--source is required for analyze command")
	}
	return optimizeCLI.Analyze(ctx, source)
}

// executeApply executes the apply command
func executeApply(ctx context.Context, optimizeCLI *cli.OptimizeCLI, source string, suggestion int) error {
	if source == "" {
		return fmt.Errorf("--source is required for apply command")
	}
	if suggestion == 0 {
		return fmt.Errorf("--suggestion is required for apply command")
	}
	return optimizeCLI.Apply(ctx, source, suggestion)
}

// executeHistory executes the history command
func executeHistory(ctx context.Context, optimizeCLI *cli.OptimizeCLI, source string) error {
	if source == "" {
		return fmt.Errorf("--source is required for history command")
	}
	return optimizeCLI.History(ctx, source)
}

// printUsage prints usage information
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: zen-watcher-optimize --command=<command> [options]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  analyze    - Analyze optimization opportunities for a source\n")
	fmt.Fprintf(os.Stderr, "  apply      - Apply a suggestion by index\n")
	fmt.Fprintf(os.Stderr, "  auto       - DEPRECATED: Auto-optimization removed. Configure processing order in Ingester CRD.\n")
	fmt.Fprintf(os.Stderr, "  history    - Show optimization history for a source\n")
	fmt.Fprintf(os.Stderr, "  list       - List all sources and their optimization status\n\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=analyze --source=<source-name>\n")
	fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=apply --source=<source-name> --suggestion=1\n")
	fmt.Fprintf(os.Stderr, "  DEPRECATED: Use Ingester CRD spec.processing.order instead\n")
	fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=history --source=<source-name>\n")
	fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=list\n")
}
