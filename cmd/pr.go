package cmd

import (
	"fmt"
	"strconv"

	"github.com/awhite/wtree/internal/github"
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage GitHub PR worktrees",
	Long: `Manage GitHub Pull Request worktrees.

This command integrates with GitHub CLI to create and manage worktrees
for GitHub Pull Requests. It supports creating worktrees from PR numbers,
listing PR worktrees, and cleaning up closed/merged PRs.

Examples:
  wtree pr 123                     # Create worktree for PR #123
  wtree pr list                    # List all PR worktrees
  wtree pr clean                   # Clean up closed PR worktrees
  wtree pr clean --state merged    # Clean up only merged PRs`,
}

var prCreateCmd = &cobra.Command{
	Use:   "create <pr-number>",
	Short: "Create worktree for a GitHub PR",
	Long: `Create a git worktree for a specific GitHub Pull Request.

This command fetches the PR information from GitHub, checks out the
PR branch locally, and creates a worktree with the naming pattern
{repo}-pr-{number}. It also stores PR metadata for later reference.

Examples:
  wtree pr create 123              # Create worktree for PR #123
  wtree pr create 456 -o           # Create and open in editor
  wtree pr create 789 --force      # Force creation even if path exists`,
	Aliases: []string{"checkout", "co"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse PR number
		prNumber, err := strconv.Atoi(args[0])
		if err != nil || prNumber <= 0 {
			return fmt.Errorf("invalid PR number: %s", args[0])
		}

		manager, err := setupManager()
		if err != nil {
			return err
		}

		// Create GitHub client
		globalConfig := manager.GetGlobalConfig()
		githubClient := github.NewClient(
			globalConfig.GitHub.CLICommand,
			globalConfig.GitHub.CacheTimeout,
		)

		// Create PR manager
		prManager := worktree.NewPRManager(manager, githubClient)

		// Get flag values
		openEditor, _ := cmd.Flags().GetBool("open")

		options := worktree.PRWorktreeOptions{
			Force:      force,
			OpenEditor: openEditor,
		}

		return prManager.CreatePRWorktree(prNumber, options)
	},
}

// For convenience, also allow `wtree pr <number>` as a shortcut
var prNumberCmd = &cobra.Command{
	Use:   "<pr-number>",
	Short: "Create worktree for PR (shorthand)",
	Long:  `Create worktree for a GitHub PR. This is a shorthand for 'wtree pr create <pr-number>'.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// This is the same as pr create, but as a direct subcommand
		return prCreateCmd.RunE(cmd, args)
	},
	Hidden: true, // Hide from help to avoid confusion
}

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all PR worktrees",
	Long: `List all GitHub Pull Request worktrees with their status.

Shows PR number, title, author, state, and worktree path for each
PR worktree. Includes both active and inactive PR worktrees.

Examples:
  wtree pr list                    # List all PR worktrees
  wtree pr list --verbose          # List with detailed information`,
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		// Create GitHub client
		globalConfig := manager.GetGlobalConfig()
		githubClient := github.NewClient(
			globalConfig.GitHub.CLICommand,
			globalConfig.GitHub.CacheTimeout,
		)

		// Create PR manager
		prManager := worktree.NewPRManager(manager, githubClient)

		// Get all PR worktrees
		prWorktrees, err := prManager.ListPRWorktrees()
		if err != nil {
			return err
		}

		if len(prWorktrees) == 0 {
			manager.GetUI().Info("No PR worktrees found")
			return nil
		}

		// Display results
		ui := manager.GetUI()
		ui.Header("GitHub PR Worktrees")

		table := ui.NewTable()
		table.SetHeaders("PR", "Title", "Author", "State", "Path")

		for _, prWt := range prWorktrees {
			title := prWt.PRTitle
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			if title == "" {
				title = "<unknown>"
			}

			author := prWt.PRAuthor
			if author == "" {
				author = "<unknown>"
			}

			state := prWt.PRState
			if state == "" {
				state = "<unknown>"
			}

			table.AddRow(
				fmt.Sprintf("#%d", prWt.PRNumber),
				title,
				author,
				state,
				prWt.Path,
			)
		}

		table.Render()
		return nil
	},
}

var prCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up PR worktrees",
	Long: `Clean up GitHub Pull Request worktrees based on PR state.

By default, cleans up worktrees for closed and merged PRs. You can
specify different criteria using flags. Use --dry-run to preview
what would be cleaned up.

Examples:
  wtree pr clean                   # Clean up closed/merged PRs
  wtree pr clean --state closed    # Clean up only closed PRs
  wtree pr clean --state merged    # Clean up only merged PRs
  wtree pr clean --dry-run         # Preview cleanup without executing
  wtree pr clean --limit 10        # Clean up at most 10 worktrees`,
	Aliases: []string{"cleanup"},
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := setupManager()
		if err != nil {
			return err
		}

		// Create GitHub client
		globalConfig := manager.GetGlobalConfig()
		githubClient := github.NewClient(
			globalConfig.GitHub.CLICommand,
			globalConfig.GitHub.CacheTimeout,
		)

		// Create PR manager
		prManager := worktree.NewPRManager(manager, githubClient)

		// Get flag values
		state, _ := cmd.Flags().GetString("state")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		limit, _ := cmd.Flags().GetInt("limit")

		// Default state to "closed" if not specified
		if state == "" {
			state = "closed"
		}

		options := worktree.PRCleanupOptions{
			State:  state,
			Force:  force,
			DryRun: dryRun,
			Limit:  limit,
		}

		return prManager.CleanupPRWorktrees(options)
	},
}

func init() {
	rootCmd.AddCommand(prCmd)

	// Add subcommands
	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prListCmd)
	prCmd.AddCommand(prCleanCmd)

	// Add the hidden shorthand command
	prCmd.AddCommand(prNumberCmd)

	// Flags for pr create
	prCreateCmd.Flags().BoolP("open", "o", false, "open in editor after creation")

	// Flags for pr clean
	prCleanCmd.Flags().String("state", "", "PR state to clean up (open, closed, merged, all)")
	prCleanCmd.Flags().Bool("dry-run", false, "show what would be cleaned up without executing")
	prCleanCmd.Flags().Int("limit", 0, "maximum number of PRs to clean up (0 = no limit)")
}