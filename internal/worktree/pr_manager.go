package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/awhite/wtree/internal/github"
	"github.com/awhite/wtree/pkg/types"
)

// PRManager handles PR-specific worktree operations
type PRManager struct {
	*Manager
	github *github.Client
}

// PRWorktreeOptions defines options for PR worktree creation
type PRWorktreeOptions struct {
	Force      bool // Force creation even if path exists
	OpenEditor bool // Open in editor after creation
}

// PRCleanupOptions defines options for PR cleanup operations
type PRCleanupOptions struct {
	State   string // PR state filter (open, closed, merged, all)
	Force   bool   // Force cleanup without confirmation
	DryRun  bool   // Show what would be cleaned up
	Limit   int    // Maximum number of PRs to process
}

// PRWorktreeInfo represents a PR worktree with metadata
type PRWorktreeInfo struct {
	*types.WorktreeInfo
	PRNumber   int
	PRTitle    string
	PRAuthor   string
	PRState    string
	PRUrl      string
	PRIsDraft  bool
	LastUpdate time.Time
}

// NewPRManager creates a new PR worktree manager
func NewPRManager(manager *Manager, githubClient *github.Client) *PRManager {
	return &PRManager{
		Manager: manager,
		github:  githubClient,
	}
}

// CreatePRWorktree creates a worktree for a specific PR
func (pm *PRManager) CreatePRWorktree(prNumber int, options PRWorktreeOptions) error {
	pm.ui.Header("Creating worktree for PR #%d", prNumber)

	// Validate GitHub CLI availability
	if err := pm.github.IsAvailable(); err != nil {
		return err
	}

	// Fetch PR information
	pm.ui.Progress("Fetching PR information...")
	prInfo, err := pm.github.GetPR(prNumber)
	if err != nil {
		return err
	}

	// Validate PR state
	if err := pm.github.ValidatePRState(prInfo); err != nil {
		return err
	}

	// Warn about draft PRs
	if prInfo.IsDraft {
		pm.ui.Warning("PR #%d is a draft", prNumber)
	}

	pm.ui.Info("PR: %s by %s", prInfo.Title, prInfo.Author)
	pm.ui.Info("Branch: %s -> %s", prInfo.HeadRef, prInfo.BaseRef)

	// Clear any previous rollback operations
	pm.rollback.Clear()

	// Generate PR worktree path
	worktreePath, err := pm.generatePRWorktreePath(prNumber)
	if err != nil {
		return fmt.Errorf("failed to generate PR worktree path: %w", err)
	}

	// Check if path already exists
	if pathExists(worktreePath) {
		if !options.Force {
			return types.NewFileSystemError("create-pr-worktree", worktreePath,
				fmt.Sprintf("PR worktree path already exists: %s", worktreePath), nil)
		}
		pm.ui.Warning("Removing existing path: %s", worktreePath)
		if err := pm.removeExistingPath(worktreePath); err != nil {
			return err
		}
	}

	// Checkout PR branch using GitHub CLI
	pm.ui.Progress("Checking out PR branch...")
	branchName, err := pm.github.CheckoutPR(prNumber)
	if err != nil {
		return fmt.Errorf("failed to checkout PR: %w", err)
	}

	pm.ui.Info("Checked out branch: %s", branchName)

	// Execute pre-create hooks
	hookCtx := pm.buildPRHookContext(types.HookPreCreate, branchName, worktreePath, prInfo)
	if err := pm.executeHooks(types.HookPreCreate, hookCtx); err != nil {
		return fmt.Errorf("pre-create hook failed: %w", err)
	}

	// Create the worktree
	pm.ui.Info("Creating PR worktree at: %s", worktreePath)
	if err := pm.repo.CreateWorktree(worktreePath, branchName); err != nil {
		return fmt.Errorf("failed to create PR worktree: %w", err)
	}
	pm.rollback.AddWorktreeCleanup(worktreePath)

	// Copy/link files based on configuration
	if err := pm.handleFileOperations(worktreePath); err != nil {
		pm.ui.Warning("File operations failed: %v", err)
		pm.ui.Warning("Rolling back PR worktree creation")
		pm.rollback.Execute()
		return fmt.Errorf("file operations failed: %w", err)
	}

	// Store PR metadata
	if err := pm.storePRMetadata(worktreePath, prInfo); err != nil {
		pm.ui.Warning("Failed to store PR metadata: %v", err)
	}

	// Execute post-create hooks
	hookCtx.Event = types.HookPostCreate
	if err := pm.executeHooks(types.HookPostCreate, hookCtx); err != nil {
		pm.ui.Warning("Post-create hook failed, but PR worktree was created: %v", err)
	}

	// Success - clear rollback operations
	pm.rollback.Clear()
	pm.ui.Success("PR worktree created successfully: %s", worktreePath)
	pm.ui.InfoIndented("PR #%d: %s", prNumber, prInfo.Title)
	pm.ui.InfoIndented("Author: %s", prInfo.Author)
	pm.ui.InfoIndented("URL: %s", prInfo.URL)

	// Open in editor if configured
	if options.OpenEditor || pm.shouldAutoOpenEditor() {
		if err := pm.openInEditor(worktreePath); err != nil {
			pm.ui.Warning("Failed to open in editor: %v", err)
		}
	}

	return nil
}

// ListPRWorktrees lists all PR-related worktrees
func (pm *PRManager) ListPRWorktrees() ([]*PRWorktreeInfo, error) {
	worktrees, err := pm.repo.ListWorktrees()
	if err != nil {
		return nil, err
	}

	var prWorktrees []*PRWorktreeInfo
	repoName := pm.repo.GetRepoName()

	for _, wt := range worktrees {
		if pm.isPRWorktree(wt.Path, repoName) {
			prNumber := pm.extractPRNumber(wt.Path, repoName)
			if prNumber > 0 {
				prWorktree := &PRWorktreeInfo{
					WorktreeInfo: wt,
					PRNumber:     prNumber,
				}

				// Try to load PR metadata
				if metadata, err := pm.loadPRMetadata(wt.Path); err == nil {
					prWorktree.PRTitle = metadata.Title
					prWorktree.PRAuthor = metadata.Author
					prWorktree.PRState = metadata.State
					prWorktree.PRUrl = metadata.URL
					prWorktree.PRIsDraft = metadata.IsDraft
					prWorktree.LastUpdate = metadata.UpdatedAt
				}

				prWorktrees = append(prWorktrees, prWorktree)
			}
		}
	}

	return prWorktrees, nil
}

// CleanupPRWorktrees removes PR worktrees based on criteria
func (pm *PRManager) CleanupPRWorktrees(options PRCleanupOptions) error {
	pm.ui.Header("Cleaning up PR worktrees")

	// Get all PR worktrees
	prWorktrees, err := pm.ListPRWorktrees()
	if err != nil {
		return err
	}

	if len(prWorktrees) == 0 {
		pm.ui.Info("No PR worktrees found")
		return nil
	}

	// Filter PRs by state if specified
	var toCleanup []*PRWorktreeInfo
	if options.State != "" && options.State != "all" {
		// Fetch current PR states from GitHub
		pm.ui.Progress("Checking PR states...")
		
		for _, prWt := range prWorktrees {
			if prInfo, err := pm.github.GetPR(prWt.PRNumber); err == nil {
				if options.State == prInfo.State || 
				   (options.State == "closed" && (prInfo.State == "closed" || prInfo.State == "merged")) {
					prWt.PRState = prInfo.State
					toCleanup = append(toCleanup, prWt)
				}
			} else {
				// If we can't fetch PR info, assume it might be deleted/closed
				if options.State == "closed" {
					toCleanup = append(toCleanup, prWt)
				}
			}
		}
	} else {
		toCleanup = prWorktrees
	}

	// Apply limit if specified
	if options.Limit > 0 && len(toCleanup) > options.Limit {
		toCleanup = toCleanup[:options.Limit]
	}

	if len(toCleanup) == 0 {
		pm.ui.Info("No PR worktrees match cleanup criteria")
		return nil
	}

	// Show what would be cleaned up
	pm.ui.Info("Found %d PR worktrees for cleanup:", len(toCleanup))
	table := pm.ui.NewTable()
	table.SetHeaders("PR", "Title", "Author", "State", "Path")

	for _, prWt := range toCleanup {
		title := prWt.PRTitle
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		table.AddRow(
			fmt.Sprintf("#%d", prWt.PRNumber),
			title,
			prWt.PRAuthor,
			prWt.PRState,
			prWt.Path,
		)
	}
	table.Render()

	if options.DryRun {
		pm.ui.Info("Dry run - no worktrees were actually removed")
		return nil
	}

	// Confirm cleanup unless forced
	if !options.Force {
		confirmMsg := fmt.Sprintf("Delete %d PR worktrees?", len(toCleanup))
		if err := pm.ui.Confirm(confirmMsg); err != nil {
			return err
		}
	}

	// Remove each PR worktree
	removed := 0
	for _, prWt := range toCleanup {
		pm.ui.Info("Removing PR #%d worktree: %s", prWt.PRNumber, prWt.Path)
		
		deleteOptions := DeleteOptions{
			DeleteBranch: false, // Don't delete PR branches automatically
			Force:        options.Force,
			IgnoreDirty:  true, // Allow cleanup of dirty PR worktrees
		}

		if err := pm.Delete(prWt.Branch, deleteOptions); err != nil {
			pm.ui.Warning("Failed to remove PR #%d worktree: %v", prWt.PRNumber, err)
		} else {
			removed++
		}
	}

	pm.ui.Success("Successfully removed %d out of %d PR worktrees", removed, len(toCleanup))
	return nil
}

// Helper methods

func (pm *PRManager) generatePRWorktreePath(prNumber int) (string, error) {
	repoRoot, err := pm.repo.GetRepoRoot()
	if err != nil {
		return "", err
	}

	parentDir := filepath.Dir(repoRoot)
	repoName := pm.repo.GetRepoName()
	
	// PR worktree pattern: {repo}-pr-{number}
	dirName := fmt.Sprintf("%s-pr-%d", repoName, prNumber)
	
	return filepath.Join(parentDir, dirName), nil
}

func (pm *PRManager) isPRWorktree(path, repoName string) bool {
	baseName := filepath.Base(path)
	expectedPrefix := repoName + "-pr-"
	return strings.HasPrefix(baseName, expectedPrefix)
}

func (pm *PRManager) extractPRNumber(path, repoName string) int {
	baseName := filepath.Base(path)
	expectedPrefix := repoName + "-pr-"
	
	if !strings.HasPrefix(baseName, expectedPrefix) {
		return 0
	}
	
	prNumberStr := strings.TrimPrefix(baseName, expectedPrefix)
	if prNumber, err := parsePositiveInt(prNumberStr); err == nil {
		return prNumber
	}
	
	return 0
}

func (pm *PRManager) buildPRHookContext(event types.HookEvent, branch, worktreePath string, prInfo *github.PRInfo) types.HookContext {
	repoRoot, _ := pm.repo.GetRepoRoot()
	
	ctx := types.HookContext{
		Event:        event,
		Branch:       branch,
		RepoPath:     repoRoot,
		WorktreePath: worktreePath,
		TargetBranch: prInfo.BaseRef,
		Environment:  make(map[string]string),
	}

	// Add PR-specific environment variables
	ctx.Environment["WTREE_PR_NUMBER"] = fmt.Sprintf("%d", prInfo.Number)
	ctx.Environment["WTREE_PR_TITLE"] = prInfo.Title
	ctx.Environment["WTREE_PR_AUTHOR"] = prInfo.Author
	ctx.Environment["WTREE_PR_URL"] = prInfo.URL
	ctx.Environment["WTREE_PR_STATE"] = prInfo.State
	ctx.Environment["WTREE_PR_HEAD_REF"] = prInfo.HeadRef
	ctx.Environment["WTREE_PR_BASE_REF"] = prInfo.BaseRef

	return ctx
}

func (pm *PRManager) storePRMetadata(worktreePath string, prInfo *github.PRInfo) error {
	metadataPath := filepath.Join(worktreePath, ".wtree-pr.json")
	
	metadataJson := fmt.Sprintf(`{
	"number": %d,
	"title": %q,
	"author": %q,
	"state": %q,
	"url": %q,
	"isDraft": %t,
	"headRef": %q,
	"baseRef": %q,
	"createdAt": %q,
	"updatedAt": %q
}`, prInfo.Number, prInfo.Title, prInfo.Author, prInfo.State, prInfo.URL,
		prInfo.IsDraft, prInfo.HeadRef, prInfo.BaseRef, 
		prInfo.CreatedAt.Format(time.RFC3339), prInfo.UpdatedAt.Format(time.RFC3339))

	return writeFile(metadataPath, []byte(metadataJson), 0644)
}

func (pm *PRManager) loadPRMetadata(worktreePath string) (*github.PRInfo, error) {
	metadataPath := filepath.Join(worktreePath, ".wtree-pr.json")
	
	data, err := readFile(metadataPath)
	if err != nil {
		return nil, err
	}
	
	var prInfo github.PRInfo
	if err := json.Unmarshal(data, &prInfo); err != nil {
		return nil, err
	}
	
	return &prInfo, nil
}

// Utility functions that would need to be implemented or imported
func parsePositiveInt(s string) (int, error) {
	if i, err := strconv.Atoi(s); err != nil {
		return 0, err
	} else if i <= 0 {
		return 0, fmt.Errorf("not a positive integer: %d", i)
	} else {
		return i, nil
	}
}

// Helper functions for file I/O
func writeFile(path string, data []byte, perm int) error {
	return os.WriteFile(path, data, os.FileMode(perm))
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (pm *PRManager) removeExistingPath(path string) error {
	return os.RemoveAll(path)
}