package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `Generate shell completion scripts for wtree.

To load completions:

Bash:
  $ source <(wtree completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ wtree completion bash > /etc/bash_completion.d/wtree
  # macOS:
  $ wtree completion bash > /usr/local/etc/bash_completion.d/wtree

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  
  # To load completions for each session, execute once:
  $ wtree completion zsh > "${fpath[1]}/_wtree"
  
  # You will need to start a new shell for this setup to take effect.

Fish:
  $ wtree completion fish | source
  # To load completions for each session, execute once:
  $ wtree completion fish > ~/.config/fish/completions/wtree.fish

PowerShell:
  PS> wtree completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> wtree completion powershell > wtree.ps1
  # and source this file from your PowerShell profile.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletion(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}