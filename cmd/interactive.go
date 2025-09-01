package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactive mode with fuzzy-finding for branch selection",
	Long: `Launch interactive mode to browse and select branches with fuzzy-finding.

This mode provides an intuitive interface for:
- Fuzzy searching through available branches
- Quick worktree creation and switching
- Batch operations on multiple branches
- Visual preview of operations

Examples:
  wtree interactive                 # Launch interactive browser
  wtree interactive --create        # Interactive branch creation
  wtree interactive --cleanup       # Interactive cleanup mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		// Get flag values
		createMode, _ := cmd.Flags().GetBool("create")
		cleanupMode, _ := cmd.Flags().GetBool("cleanup")
		switchMode, _ := cmd.Flags().GetBool("switch")

		options := worktree.InteractiveOptions{
			CreateMode:  createMode,
			CleanupMode: cleanupMode,
			SwitchMode:  switchMode,
			DryRun:      dryRun,
		}

		return manager.Interactive(options)
	},
}

func init() {
	rootCmd.AddCommand(interactiveCmd)

	interactiveCmd.Flags().Bool("create", false, "launch in branch creation mode")
	interactiveCmd.Flags().Bool("cleanup", false, "launch in cleanup mode")
	interactiveCmd.Flags().Bool("switch", false, "launch in switch mode")
}
