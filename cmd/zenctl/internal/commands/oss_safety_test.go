package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestZenctlOSSRemainsSaaSFree ensures zenctl-oss does not register SaaS-only commands
func TestZenctlOSSRemainsSaaSFree(t *testing.T) {
	rootCmd := newRootCommandForTest()

	// Get list of all command names
	commands := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		commands[cmd.Name()] = true
	}

	// SaaS-only commands that must not exist in OSS
	forbiddenCommands := []string{
		"audit",       // SaaS API endpoint
		"entitlement", // SaaS entitlement management
		"tenant",      // SaaS tenant management
	}

	for _, forbidden := range forbiddenCommands {
		if commands[forbidden] {
			t.Errorf("OSS zenctl must not include SaaS-only command '%s'", forbidden)
		}
	}

	// Verify expected OSS commands exist
	expectedCommands := []string{
		"status",
		"flows",
		"explain",
		"doctor",
		"adapters",
		"e2e",
		"export",
		"validate",
		"diff",
		"version",
		"completion",
	}

	for _, expected := range expectedCommands {
		if !commands[expected] {
			t.Errorf("Expected OSS command '%s' not found", expected)
		}
	}
}

// TestZenctlHelpOutputContainsOnlyOSSCommands checks help output for SaaS commands
func TestZenctlHelpOutputContainsOnlyOSSCommands(t *testing.T) {
	rootCmd := newRootCommandForTest()

	// Generate help output
	var helpOutput strings.Builder
	rootCmd.SetOutput(&helpOutput)
	_ = rootCmd.Usage()

	helpText := helpOutput.String()

	// SaaS-only patterns that must not appear in help
	forbiddenPatterns := []string{
		"audit",
		"entitlement",
		"tenant",
		"/v1/audit",
		"ZEN_API_BASE_URL",
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(helpText, pattern) {
			t.Errorf("OSS zenctl help output must not contain SaaS pattern '%s'", pattern)
		}
	}
}

// newRootCommandForTest creates a root command for testing (without executing)
func newRootCommandForTest() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "zenctl",
		Short: "Operator-grade CLI for Zen Kubernetes resources",
	}

	// Register all commands (same as main.go)
	rootCmd.AddCommand(NewStatusCommand())
	rootCmd.AddCommand(NewFlowsCommand())
	rootCmd.AddCommand(NewExplainCommand())
	rootCmd.AddCommand(NewDoctorCommand())
	rootCmd.AddCommand(NewVersionCommand())
	rootCmd.AddCommand(NewAdaptersCommand())
	rootCmd.AddCommand(NewE2ECommand())
	rootCmd.AddCommand(NewExportCommand())
	rootCmd.AddCommand(NewValidateCommand())
	rootCmd.AddCommand(NewDiffCommand())
	rootCmd.AddCommand(NewCompletionCommand(rootCmd))

	return rootCmd
}
