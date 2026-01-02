package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func NewE2ECommand() *cobra.Command {
	var scriptPath string
	var artifactsDir string

	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "Run e2e tests and collect artifacts",
		Long: `Runs e2e test scripts and collects artifacts to a timestamped directory.

Looks for e2e scripts in common locations:
- scripts/e2e/*.sh
- test/e2e/*.sh

Artifacts are collected to a timestamped directory in the current working directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine script path
			if scriptPath == "" {
				// Look for scripts in common locations
				possiblePaths := []string{
					"scripts/e2e/run-e2e-matrix.sh",
					"scripts/e2e/bootstrap-kind-cluster.sh",
					"test/e2e/run.sh",
				}

				found := false
				for _, path := range possiblePaths {
					if _, err := os.Stat(path); err == nil {
						scriptPath = path
						found = true
						break
					}
				}

				if !found {
					return fmt.Errorf("no e2e script found in common locations. Use --script to specify path")
				}
			}

			// Create artifacts directory
			if artifactsDir == "" {
				timestamp := time.Now().Format("20060102-150405")
				artifactsDir = fmt.Sprintf("e2e-artifacts-%s", timestamp)
			}

			if err := os.MkdirAll(artifactsDir, 0755); err != nil {
				return fmt.Errorf("failed to create artifacts directory: %w", err)
			}

			cmd.Printf("Running e2e script: %s\n", scriptPath)
			cmd.Printf("Artifacts directory: %s\n", artifactsDir)
			cmd.Println("")

			// Run the script
			scriptCmd := exec.Command("bash", scriptPath)
			scriptCmd.Stdout = os.Stdout
			scriptCmd.Stderr = os.Stderr
			scriptCmd.Dir = filepath.Dir(scriptPath)

			// Set environment variable for artifacts directory
			scriptCmd.Env = append(os.Environ(), fmt.Sprintf("E2E_ARTIFACTS_DIR=%s", artifactsDir))

			if err := scriptCmd.Run(); err != nil {
				return fmt.Errorf("e2e script failed: %w", err)
			}

			cmd.Printf("\n✅ E2E tests completed. Artifacts collected in: %s\n", artifactsDir)

			return nil
		},
	}

	cmd.Flags().StringVar(&scriptPath, "script", "", "Path to e2e test script (default: auto-detect)")
	cmd.Flags().StringVar(&artifactsDir, "artifacts-dir", "", "Artifacts directory (default: e2e-artifacts-<timestamp>)")

	return cmd
}

