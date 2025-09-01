package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:   "switch <branch-or-path>",
	Short: "Switch to a worktree",
	Long: `Switch to a different worktree by branch name or path.

This command helps you navigate between worktrees. You can specify either
the branch name or the worktree path. Use -o to automatically open in
your configured editor.

Examples:
  wtree switch main                    # Switch to main worktree
  wtree switch feature-branch          # Switch to feature branch worktree
  wtree switch -o bugfix               # Switch and open in editor`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeExistingWorktrees,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		identifier := args[0]

		// Get flag values
		openEditor, _ := cmd.Flags().GetBool("open")

		options := worktree.SwitchOptions{
			OpenEditor: openEditor,
		}

		return manager.Switch(identifier, options)
	},
}

func init() {
	rootCmd.AddCommand(switchCmd)

	switchCmd.Flags().BoolP("open", "o", false, "open in editor after switching")
}
