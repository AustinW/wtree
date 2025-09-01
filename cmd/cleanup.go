package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Smart cleanup of merged and stale worktrees",
	Long: `Intelligently clean up worktrees that are no longer needed.

This command analyzes your worktrees and identifies candidates for cleanup:
- Branches that have been merged into the main branch
- Worktrees with no recent activity (stale)
- Broken or corrupted worktrees

You can preview what will be cleaned up with --dry-run, and use various
filters to be more selective about what gets cleaned up.

Examples:
  wtree cleanup                        # Interactive cleanup with prompts
  wtree cleanup --dry-run             # Preview what would be cleaned up
  wtree cleanup --merged-only         # Clean only merged branches
  wtree cleanup --auto                # Auto-cleanup without prompts
  wtree cleanup --older-than 30d      # Clean worktrees older than 30 days`,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		// Get flag values
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		mergedOnly, _ := cmd.Flags().GetBool("merged-only")
		auto, _ := cmd.Flags().GetBool("auto")
		olderThan, _ := cmd.Flags().GetString("older-than")
		verbose, _ := cmd.Flags().GetBool("verbose")

		options := worktree.CleanupOptions{
			DryRun:     dryRun,
			MergedOnly: mergedOnly,
			Auto:       auto,
			OlderThan:  olderThan,
			Verbose:    verbose,
		}

		return manager.Cleanup(options)
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().BoolP("dry-run", "n", false, "preview what would be cleaned up")
	cleanupCmd.Flags().Bool("merged-only", false, "clean only branches that have been merged")
	cleanupCmd.Flags().Bool("auto", false, "automatically clean up without prompts")
	cleanupCmd.Flags().String("older-than", "", "clean worktrees older than duration (e.g., 30d, 2w)")
	cleanupCmd.Flags().BoolP("verbose", "v", false, "show detailed information about cleanup candidates")
}
