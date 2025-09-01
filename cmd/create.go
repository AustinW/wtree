package cmd

import (
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <branch-name>",
	Short: "Create a new worktree",
	Long: `Create a new git worktree for the specified branch.

If the branch doesn't exist, use -b to create it. The worktree will be 
created in the parent directory using the configured naming pattern.

Examples:
  wtree create feature-branch           # Create worktree for existing branch
  wtree create -b new-feature main     # Create new branch from main
  wtree create -f existing-branch      # Force creation even if path exists`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: completeBranchNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		branchName := args[0]
		
		// Get flag values
		createBranch, _ := cmd.Flags().GetBool("branch")
		fromBranch, _ := cmd.Flags().GetString("from")
		openEditor, _ := cmd.Flags().GetBool("open")
		
		options := worktree.CreateOptions{
			CreateBranch: createBranch,
			FromBranch:   fromBranch,
			Force:        force,
			OpenEditor:   openEditor,
			DryRun:       dryRun,
		}

		return manager.Create(branchName, options)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().BoolP("branch", "b", false, "create new branch if it doesn't exist")
	createCmd.Flags().StringP("from", "", "HEAD", "base branch for new branch creation")
	createCmd.Flags().BoolP("open", "o", false, "open in editor after creation")
	
	// Register completion for the --from flag
	_ = createCmd.RegisterFlagCompletionFunc("from", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		manager, err := setupManager()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		branches, err := manager.GetRepo().ListBranches()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return branches, cobra.ShellCompDirectiveNoFileComp
	})
}