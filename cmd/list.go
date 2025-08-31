package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long: `List all git worktrees with their status and information.

Shows branch name, path, status, and type for each worktree. Use filters
to narrow down the results or show additional status information.

Examples:
  wtree list                           # List all worktrees
  wtree list --status                  # List with git status
  wtree list --filter feature         # Filter by branch name
  wtree list --dirty                   # Show only dirty worktrees`,
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}
		
		// Get flag values
		showStatus, _ := cmd.Flags().GetBool("status")
		branchFilter, _ := cmd.Flags().GetString("filter")
		onlyDirty, _ := cmd.Flags().GetBool("dirty")
		
		options := worktree.ListOptions{
			ShowStatus:   showStatus,
			BranchFilter: branchFilter,
			OnlyDirty:    onlyDirty,
		}

		return manager.List(options)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolP("status", "s", false, "show git status for each worktree")
	listCmd.Flags().StringP("filter", "", "", "filter by branch name (substring match)")
	listCmd.Flags().Bool("dirty", false, "show only worktrees with uncommitted changes")
}