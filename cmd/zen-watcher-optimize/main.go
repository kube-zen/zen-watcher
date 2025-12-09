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

	"github.com/kube-zen/zen-watcher/internal/kubernetes"
	"github.com/kube-zen/zen-watcher/pkg/cli"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
)

func main() {
	var (
		command    = flag.String("command", "", "Command to execute: analyze, apply, auto, history, list")
		source     = flag.String("source", "", "Source name (required for analyze, apply, history)")
		suggestion = flag.Int("suggestion", 0, "Suggestion index (required for apply)")
		enable     = flag.Bool("enable", false, "Enable auto-optimization (for auto command)")
		logLevel   = flag.String("log-level", "INFO", "Log level")
	)
	flag.Parse()

	// Initialize logger
	if err := logger.Init(*logLevel, false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Initialize Kubernetes clients
	clients, err := kubernetes.NewClients()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Kubernetes clients: %v\n", err)
		os.Exit(1)
	}

	// Create source config loader
	sourceConfigLoader := config.NewSourceConfigLoader(clients.Dynamic)

	// Create CLI (advisor would be created if Prometheus client available)
	optimizeCLI := cli.NewOptimizeCLI(clients.Dynamic, sourceConfigLoader, nil)

	ctx := context.Background()

	// Execute command
	switch *command {
	case "analyze":
		if *source == "" {
			fmt.Fprintf(os.Stderr, "Error: --source is required for analyze command\n")
			os.Exit(1)
		}
		if err := optimizeCLI.Analyze(ctx, *source); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "apply":
		if *source == "" {
			fmt.Fprintf(os.Stderr, "Error: --source is required for apply command\n")
			os.Exit(1)
		}
		if *suggestion == 0 {
			fmt.Fprintf(os.Stderr, "Error: --suggestion is required for apply command\n")
			os.Exit(1)
		}
		if err := optimizeCLI.Apply(ctx, *source, *suggestion); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "auto":
		if err := optimizeCLI.Auto(ctx, *enable); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "history":
		if *source == "" {
			fmt.Fprintf(os.Stderr, "Error: --source is required for history command\n")
			os.Exit(1)
		}
		if err := optimizeCLI.History(ctx, *source); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "list":
		if err := optimizeCLI.ListSources(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Usage: zen-watcher-optimize --command=<command> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  analyze    - Analyze optimization opportunities for a source\n")
		fmt.Fprintf(os.Stderr, "  apply      - Apply a suggestion by index\n")
		fmt.Fprintf(os.Stderr, "  auto       - Enable/disable auto-optimization\n")
		fmt.Fprintf(os.Stderr, "  history    - Show optimization history for a source\n")
		fmt.Fprintf(os.Stderr, "  list       - List all sources and their optimization status\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=analyze --source=<source-name>\n")
		fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=apply --source=<source-name> --suggestion=1\n")
		fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=auto --enable\n")
		fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=history --source=<source-name>\n")
		fmt.Fprintf(os.Stderr, "  zen-watcher-optimize --command=list\n")
		os.Exit(1)
	}
}
