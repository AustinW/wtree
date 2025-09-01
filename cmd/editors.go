package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var editorsCmd = &cobra.Command{
	Use:   "editors [branch-or-path]",
	Short: "Open worktree in multiple editors",
	Long: `Open a worktree in multiple editors simultaneously.

This command allows you to open the same worktree in multiple editors 
at once, which is useful for different workflows (e.g., code + terminal).

Examples:
  wtree editors feature-branch          # Open in all configured editors
  wtree editors --editors code,vim .    # Open in specific editors
  wtree editors --terminal feature-branch  # Also open terminal`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeExistingWorktrees,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		var identifier string
		if len(args) > 0 {
			identifier = args[0]
		} else {
			identifier = "." // Current directory
		}

		// Get flag values
		editorsFlag, _ := cmd.Flags().GetString("editors")
		openTerminal, _ := cmd.Flags().GetBool("terminal")

		options := worktree.EditorsOptions{
			Editors:      editorsFlag,
			OpenTerminal: openTerminal,
		}

		return manager.OpenInEditors(identifier, options)
	},
}

func init() {
	rootCmd.AddCommand(editorsCmd)

	editorsCmd.Flags().String("editors", "", "comma-separated list of editors to open (e.g., 'code,vim')")
	editorsCmd.Flags().BoolP("terminal", "t", false, "also open a terminal in the worktree")
}
