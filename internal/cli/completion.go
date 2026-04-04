// Package cli provides the completion subcommand for shell script generation.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCmd creates the "completion" subcommand that generates shell
// completion scripts for bash, zsh, fish, and powershell.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate completion script",
		Long: `Generate a shell completion script for the specified shell.

To load completions:

Bash:
  $ source <(clawgo completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ clawgo completion bash > /etc/bash_completion.d/clawgo
  # macOS:
  $ clawgo completion bash > $(brew --prefix)/etc/bash_completion.d/clawgo

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ clawgo completion zsh > "${fpath[1]}/_clawgo"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ clawgo completion fish | source

  # To load completions for each session, execute once:
  $ clawgo completion fish > ~/.config/fish/completions/clawgo.fish

PowerShell:
  PS> clawgo completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, add the output to your profile:
  PS> clawgo completion powershell >> $PROFILE
`,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}

	return cmd
}
