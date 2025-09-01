package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge <source-branch>",
	Short: "Merge a branch into current worktree",
	Long: `Merge changes from the specified branch into the current worktree.

The working directory must be clean unless --force is used. This runs
pre-merge and post-merge hooks if configured in .wtreerc.

Examples:
  wtree merge feature-branch           # Merge feature into current
  wtree merge -m "Custom message" fix  # Merge with custom message
  wtree merge --force dirty-branch     # Force merge even if dirty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		sourceBranch := args[0]

		// Get flag values
		message, _ := cmd.Flags().GetString("message")

		options := worktree.MergeOptions{
			Message: message,
			Force:   force,
		}

		return manager.Merge(sourceBranch, options)
	},
}

func init() {
	rootCmd.AddCommand(mergeCmd)

	mergeCmd.Flags().StringP("message", "m", "", "custom merge commit message")
}
