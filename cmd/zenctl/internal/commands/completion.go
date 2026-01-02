package commands

import (
	"github.com/spf13/cobra"
)

func NewCompletionCommand(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for zenctl.

To load completions:

Bash:
  $ source <(zenctl completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ zenctl completion bash > /etc/bash_completion.d/zenctl
  # macOS:
  $ zenctl completion bash > $(brew --prefix)/etc/bash_completion.d/zenctl

Zsh:
  $ source <(zenctl completion zsh)
  # To load completions for each session, execute once:
  $ zenctl completion zsh > "${fpath[1]}/_zenctl"

Fish:
  $ zenctl completion fish | source
  # To load completions for each session, execute once:
  $ zenctl completion fish > ~/.config/fish/completions/zenctl.fish
`,
		ValidArgs: []string{"bash", "zsh", "fish"},
		Args:      cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			switch shell {
			case "bash":
				return rootCmd.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return rootCmd.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return cmd.Help()
			}
		},
	}
	return cmd
}

