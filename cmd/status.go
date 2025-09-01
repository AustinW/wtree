package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detailed status of worktrees",
	Long: `Show detailed status information for all worktrees including git status,
branch relationships, sync status, and ahead/behind information.

This provides a comprehensive overview of your worktree ecosystem, helping you
understand which branches need attention, which are behind their remotes,
and which have uncommitted changes.

Examples:
  wtree status                         # Show status for all worktrees
  wtree status --current               # Show only current worktree status
  wtree status --branch feature       # Show status for specific branch
  wtree status --verbose               # Show detailed git information`,
	Aliases: []string{"st"},
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		// Get flag values
		currentOnly, _ := cmd.Flags().GetBool("current")
		branchFilter, _ := cmd.Flags().GetString("branch")
		verbose, _ := cmd.Flags().GetBool("verbose")

		options := worktree.StatusOptions{
			CurrentOnly:  currentOnly,
			BranchFilter: branchFilter,
			Verbose:      verbose,
		}

		return manager.Status(options)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolP("current", "c", false, "show only current worktree status")
	statusCmd.Flags().StringP("branch", "b", "", "show status for specific branch")
	statusCmd.Flags().BoolP("verbose", "v", false, "show detailed git information")
}
