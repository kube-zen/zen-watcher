package main

import (
	"os"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/commands"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "zenctl",
		Short: "Operator-grade CLI for Zen Kubernetes resources",
		Long: `zenctl provides a fast, operator-grade CLI for inspecting and managing
Zen Kubernetes resources including DeliveryFlows, Destinations, and Ingesters.`,
	}

	// Add global flags
	var kubeconfig string
	var context string
	var namespace string
	var allNamespaces bool

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	rootCmd.PersistentFlags().StringVar(&context, "context", "", "Kubernetes context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	rootCmd.PersistentFlags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List resources across all namespaces")

	// Store global options in command context
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.SetContext(commands.WithOptions(cmd.Context(), commands.Options{
			Kubeconfig:   kubeconfig,
			Context:      context,
			Namespace:    namespace,
			AllNamespaces: allNamespaces,
		}))
	}

	// Add subcommands
	rootCmd.AddCommand(commands.NewStatusCommand())
	rootCmd.AddCommand(commands.NewFlowsCommand())
	rootCmd.AddCommand(commands.NewExplainCommand())
	rootCmd.AddCommand(commands.NewDoctorCommand())
	rootCmd.AddCommand(commands.NewVersionCommand())
	rootCmd.AddCommand(commands.NewAdaptersCommand())
	rootCmd.AddCommand(commands.NewE2ECommand())
	// TODO S802-S804: Add export, validate, diff, support-bundle commands

	// Add completion command
	rootCmd.AddCommand(commands.NewCompletionCommand(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

