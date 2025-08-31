package cmd

import (
	"github.com/spf13/cobra"
)

// completeBranchNames provides completion for branch names
func completeBranchNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	manager, err := setupManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	branches, err := manager.GetRepo().ListBranches()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return branches, cobra.ShellCompDirectiveNoFileComp
}

// completeExistingWorktrees provides completion for existing worktree branches
func completeExistingWorktrees(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	manager, err := setupManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	worktrees, err := manager.GetRepo().ListWorktrees()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var branches []string
	for _, wt := range worktrees {
		if !wt.IsMainRepo { // Don't include main repo in completion
			branches = append(branches, wt.Branch)
		}
	}

	return branches, cobra.ShellCompDirectiveNoFileComp
}

// completeAvailableBranches provides completion for branches that don't have worktrees
func completeAvailableBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	manager, err := setupManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Get all branches
	branches, err := manager.GetRepo().ListBranches()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Get existing worktrees
	worktrees, err := manager.GetRepo().ListWorktrees()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Create a set of existing branches with worktrees
	existingBranches := make(map[string]bool)
	for _, wt := range worktrees {
		existingBranches[wt.Branch] = true
	}

	// Filter out branches that already have worktrees
	var availableBranches []string
	for _, branch := range branches {
		if !existingBranches[branch] {
			availableBranches = append(availableBranches, branch)
		}
	}

	return availableBranches, cobra.ShellCompDirectiveNoFileComp
}