package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <branch-or-path>",
	Short: "Delete a worktree",
	Long: `Delete a git worktree by branch name or path.

You can specify either the branch name or the worktree path. Use -b to also
delete the associated branch. Use --ignore-dirty to delete even if there
are uncommitted changes.

Examples:
  wtree delete feature-branch          # Delete worktree for branch
  wtree delete -b feature-branch       # Delete worktree and branch
  wtree delete --ignore-dirty old-work # Delete even if dirty`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeExistingWorktrees,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		identifier := args[0]

		// Get flag values
		deleteBranch, _ := cmd.Flags().GetBool("branch")
		ignoreDirty, _ := cmd.Flags().GetBool("ignore-dirty")

		options := worktree.DeleteOptions{
			DeleteBranch: deleteBranch,
			Force:        force,
			IgnoreDirty:  ignoreDirty,
			DryRun:       dryRun,
		}

		return manager.Delete(identifier, options)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolP("branch", "b", false, "also delete the branch")
	deleteCmd.Flags().Bool("ignore-dirty", false, "delete even if worktree has uncommitted changes")
}
