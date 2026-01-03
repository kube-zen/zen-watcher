package commands

import (
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/version"
	"github.com/spf13/cobra"
)

func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show zenctl version information",
		Long:  "Displays the version, git commit SHA, and build time of zenctl",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version.VersionCommand())
		},
	}
}
