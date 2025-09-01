package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/awhite/wtree/internal/config"
	"github.com/awhite/wtree/internal/git"
	"github.com/awhite/wtree/internal/ui"
	"github.com/awhite/wtree/pkg/types"
)

// Manager handles core worktree operations and orchestrates all components
type Manager struct {
	repo          git.Repository
	configMgr     *config.Manager
	ui            *ui.Manager
	fileManager   *FileManager
	rollback      *RollbackManager
	lockManager   *LockManager
	globalConfig  *types.WTreeConfig
	projectConfig *types.ProjectConfig
}

// NewManager creates a new worktree manager
func NewManager(repo git.Repository, configMgr *config.Manager, ui *ui.Manager) *Manager {
	lockManager, err := NewLockManager()
	if err != nil {
		// Log error but don't fail - fall back to no locking
		if ui != nil {
			ui.Warning("Failed to initialize lock manager, concurrency protection disabled: %v", err)
		}
		lockManager = nil
	}

	return &Manager{
		repo:        repo,
		configMgr:   configMgr,
		ui:          ui,
		fileManager: NewFileManager(ui != nil),
		rollback:    NewRollbackManager(repo),
		lockManager: lockManager,
	}
}

// GetRepository returns the git repository
func (m *Manager) GetRepository() git.Repository {
	return m.repo
}

// GetConfigManager returns the configuration manager
func (m *Manager) GetConfigManager() *config.Manager {
	return m.configMgr
}

// GetUIManager returns the UI manager
func (m *Manager) GetUIManager() *ui.Manager {
	return m.ui
}

// Initialize loads configurations and validates the setup
func (m *Manager) Initialize() error {
	var err error

	// Load global configuration
	m.globalConfig, err = m.configMgr.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	// Load project configuration
	repoRoot, err := m.repo.GetRepoRoot()
	if err != nil {
		return err
	}

	m.projectConfig, err = m.configMgr.LoadProjectConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Update file manager verbosity
	if m.ui != nil {
		m.fileManager = NewFileManager(m.globalConfig.UI.Verbose)
	}

	return nil
}

// GetGlobalConfig returns the global configuration
func (m *Manager) GetGlobalConfig() *types.WTreeConfig {
	return m.globalConfig
}

// GetProjectConfig returns the project configuration
func (m *Manager) GetProjectConfig() *types.ProjectConfig {
	return m.projectConfig
}

// GetUI returns the UI manager
func (m *Manager) GetUI() *ui.Manager {
	return m.ui
}

// Create creates a new worktree with the specified branch
func (m *Manager) Create(branchName string, options CreateOptions) error {
	if err := m.validateCreateOptions(branchName, options); err != nil {
		return err
	}

	m.ui.Header("Creating worktree for branch '%s'", branchName)

	// Create multi-step progress for worktree creation
	steps := []string{
		"Validating branch and options",
		"Creating git worktree",
		"Running project setup",
		"Opening in editor (if requested)",
	}
	progress := m.ui.NewMultiStepProgress(steps)

	// Step 1: Validation
	progress.StartStep(0)

	// Generate worktree path
	worktreePath, err := m.generateWorktreePath(branchName)
	if err != nil {
		progress.FailStep(0)
		return fmt.Errorf("failed to generate worktree path: %w", err)
	}
	progress.CompleteStep(0)

	// Acquire operation lock to prevent concurrent creation
	var operationLock *OperationLock
	if m.lockManager != nil {
		timeout := m.getOperationTimeout()
		operationLock, err = m.lockManager.AcquireLock(LockTypeCreate, worktreePath, timeout)
		if err != nil {
			return fmt.Errorf("failed to acquire operation lock: %w", err)
		}
		defer func() {
			if releaseErr := m.lockManager.ReleaseLock(operationLock); releaseErr != nil {
				m.ui.Warning("Failed to release operation lock: %v", releaseErr)
			}
		}()
	}

	// Clear any previous rollback operations
	m.rollback.Clear()

	// Atomically check and prepare the worktree path
	if err := m.atomicPathPreparation(worktreePath, options.Force); err != nil {
		return err
	}

	branchCreated := false
	// Create branch if needed
	if !m.repo.BranchExists(branchName) {
		if !options.CreateBranch {
			return types.NewGitError("create-worktree",
				fmt.Sprintf("branch '%s' does not exist", branchName), nil)
		}

		m.ui.Info("Creating branch '%s' from '%s'", branchName, options.FromBranch)
		if err := m.repo.CreateBranch(branchName, options.FromBranch); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
		branchCreated = true
		m.rollback.AddBranchCleanup(branchName)
	}

	// Execute pre-create hooks
	hookCtx := m.buildHookContext(types.HookPreCreate, branchName, worktreePath)
	if err := m.executeHooks(types.HookPreCreate, hookCtx); err != nil {
		if branchCreated {
			m.ui.Warning("Rolling back branch creation due to pre-create hook failure")
			_ = m.rollback.Execute()
		}
		return fmt.Errorf("pre-create hook failed: %w", err)
	}

	// Step 2: Create the worktree
	progress.StartStep(1)
	m.ui.Info("Creating worktree at: %s", worktreePath)
	if err := m.repo.CreateWorktree(worktreePath, branchName); err != nil {
		progress.FailStep(1)
		if branchCreated {
			m.ui.Warning("Rolling back branch creation due to worktree creation failure")
			_ = m.rollback.Execute()
		}
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	m.rollback.AddWorktreeCleanup(worktreePath)
	progress.CompleteStep(1)

	// Step 3: Project setup
	progress.StartStep(2)

	// Copy/link files based on configuration
	if err := m.handleFileOperations(worktreePath); err != nil {
		progress.FailStep(2)
		m.ui.Warning("File operations failed: %v", err)
		m.ui.Warning("Rolling back worktree creation")
		_ = m.rollback.Execute()
		return fmt.Errorf("file operations failed: %w", err)
	}

	// Execute post-create hooks
	hookCtx.Event = types.HookPostCreate
	if err := m.executeHooks(types.HookPostCreate, hookCtx); err != nil {
		m.ui.Warning("Post-create hook failed, but worktree was created: %v", err)
	}
	progress.CompleteStep(2)

	// Success - clear rollback operations
	m.rollback.Clear()
	m.ui.Success("Worktree created successfully: %s", worktreePath)

	// Step 4: Open in editor if configured
	if options.OpenEditor || m.shouldAutoOpenEditor() {
		progress.StartStep(3)
		if err := m.openInEditor(worktreePath); err != nil {
			progress.FailStep(3)
			m.ui.Warning("Failed to open in editor: %v", err)
		} else {
			progress.CompleteStep(3)
		}
	} else {
		progress.CompleteStep(3) // Skip this step
	}

	return nil
}

// Delete removes a worktree and optionally its branch
func (m *Manager) Delete(identifier string, options DeleteOptions) error {
	if err := m.validateDeleteOptions(identifier, options); err != nil {
		return err
	}

	// Resolve identifier to worktree info
	worktree, err := m.resolveWorktree(identifier)
	if err != nil {
		return err
	}

	if worktree.IsMainRepo {
		return types.NewValidationError("delete-worktree",
			"cannot delete main repository worktree", nil)
	}

	// Acquire operation lock to prevent concurrent operations on this worktree
	var operationLock *OperationLock
	if m.lockManager != nil {
		timeout := m.getOperationTimeout()
		operationLock, err = m.lockManager.AcquireLock(LockTypeDelete, worktree.Path, timeout)
		if err != nil {
			return fmt.Errorf("failed to acquire operation lock: %w", err)
		}
		defer func() {
			if releaseErr := m.lockManager.ReleaseLock(operationLock); releaseErr != nil {
				m.ui.Warning("Failed to release operation lock: %v", releaseErr)
			}
		}()
	}

	m.ui.Header("Deleting worktree: %s", worktree.Branch)

	// Check for uncommitted changes
	if !options.Force {
		status, err := m.repo.GetWorktreeStatus(worktree.Path)
		if err == nil && !status.IsClean {
			if !options.IgnoreDirty {
				return types.NewValidationError("delete-worktree",
					fmt.Sprintf("worktree has uncommitted changes: %s", worktree.Path), nil)
			}
			m.ui.Warning("Worktree has uncommitted changes but ignoring due to --ignore-dirty")
		}
	}

	// Confirm deletion unless forced
	if !options.Force {
		msg := fmt.Sprintf("Delete worktree '%s' at %s?", worktree.Branch, worktree.Path)
		if err := m.ui.Confirm(msg); err != nil {
			return err
		}
	}

	// If dry run, show what would be done and exit
	if options.DryRun {
		m.ui.Info("[DRY RUN] Would remove worktree: %s", worktree.Path)
		if options.DeleteBranch {
			m.ui.Info("[DRY RUN] Would delete branch: %s", worktree.Branch)
		}
		m.ui.Success("[DRY RUN] Deletion preview completed")
		return nil
	}

	// Execute pre-delete hooks
	hookCtx := m.buildHookContext(types.HookPreDelete, worktree.Branch, worktree.Path)
	if err := m.executeHooks(types.HookPreDelete, hookCtx); err != nil {
		return fmt.Errorf("pre-delete hook failed: %w", err)
	}

	// Remove the worktree
	m.ui.Info("Removing worktree: %s", worktree.Path)
	if err := m.repo.RemoveWorktree(worktree.Path, options.Force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Delete branch if requested
	if options.DeleteBranch {
		m.ui.Info("Deleting branch: %s", worktree.Branch)
		if err := m.repo.DeleteBranch(worktree.Branch, options.Force); err != nil {
			m.ui.Warning("Failed to delete branch: %v", err)
		}
	}

	// Execute post-delete hooks
	hookCtx.Event = types.HookPostDelete
	if err := m.executeHooks(types.HookPostDelete, hookCtx); err != nil {
		m.ui.Warning("Post-delete hook failed: %v", err)
	}

	m.ui.Success("Worktree deleted successfully: %s", worktree.Branch)
	return nil
}

// List displays all worktrees with their status
func (m *Manager) List(options ListOptions) error {
	m.ui.Header("Git Worktrees")

	worktrees, err := m.repo.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		m.ui.Info("No worktrees found")
		return nil
	}

	// Create table
	table := m.ui.NewTable()
	table.SetHeaders("Branch", "Path", "Status", "Type")

	for _, wt := range worktrees {
		status := "clean"
		wtType := "worktree"

		if wt.IsMainRepo {
			wtType = "main"
		}

		// Get status if requested
		if options.ShowStatus && !wt.IsMainRepo {
			if wtStatus, err := m.repo.GetWorktreeStatus(wt.Path); err == nil {
				if !wtStatus.IsClean {
					status = fmt.Sprintf("dirty (%d files)", wtStatus.ChangedFiles)
				}
			}
		}

		// Apply filters
		if options.BranchFilter != "" && !strings.Contains(wt.Branch, options.BranchFilter) {
			continue
		}
		if options.OnlyDirty && status == "clean" {
			continue
		}

		table.AddRow(wt.Branch, wt.Path, status, wtType)
	}

	table.Render()
	return nil
}

// Merge merges changes from one branch into current worktree
func (m *Manager) Merge(sourceBranch string, options MergeOptions) error {
	if err := m.validateMergeOptions(sourceBranch, options); err != nil {
		return err
	}

	currentBranch, err := m.repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	m.ui.Header("Merging '%s' into '%s'", sourceBranch, currentBranch)

	// Check working directory is clean
	if !options.Force {
		isClean, err := m.repo.IsClean()
		if err != nil {
			return fmt.Errorf("failed to check repository status: %w", err)
		}
		if !isClean {
			return types.NewValidationError("merge",
				"working directory must be clean before merge", nil)
		}
	}

	// Execute pre-merge hooks
	repoRoot, _ := m.repo.GetRepoRoot()
	hookCtx := m.buildHookContext(types.HookPreMerge, currentBranch, repoRoot)
	hookCtx.TargetBranch = sourceBranch
	if err := m.executeHooks(types.HookPreMerge, hookCtx); err != nil {
		return fmt.Errorf("pre-merge hook failed: %w", err)
	}

	// Perform the merge
	m.ui.Info("Merging branch: %s", sourceBranch)
	if err := m.repo.Merge(sourceBranch, options.Message); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	// Execute post-merge hooks
	hookCtx.Event = types.HookPostMerge
	if err := m.executeHooks(types.HookPostMerge, hookCtx); err != nil {
		m.ui.Warning("Post-merge hook failed: %v", err)
	}

	m.ui.Success("Merge completed successfully")
	return nil
}

// Switch changes to a different worktree/branch
func (m *Manager) Switch(identifier string, options SwitchOptions) error {
	worktree, err := m.resolveWorktree(identifier)
	if err != nil {
		return err
	}

	if !pathExists(worktree.Path) {
		return types.NewFileSystemError("switch", worktree.Path,
			fmt.Sprintf("worktree path does not exist: %s", worktree.Path), nil)
	}

	m.ui.Success("Switching to worktree: %s (%s)", worktree.Branch, worktree.Path)

	// Output shell command to change directory
	// This allows the user to run: eval "$(wtree switch branch-name)"
	fmt.Printf("cd %s\n", shellescape(worktree.Path))

	if options.OpenEditor || m.shouldAutoOpenEditor() {
		if err := m.openInEditor(worktree.Path); err != nil {
			m.ui.Warning("Failed to open in editor: %v", err)
		}
	}

	return nil
}

// shellescape escapes a path for safe use in shell commands
func shellescape(path string) string {
	// Simple shell escaping - wrap in single quotes and escape any single quotes
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
}

// Status shows detailed status information for worktrees
func (m *Manager) Status(options StatusOptions) error {
	m.ui.Header("Worktree Status")

	worktrees, err := m.repo.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		m.ui.Info("No worktrees found")
		return nil
	}

	// Get current working directory to identify current worktree
	currentDir, _ := os.Getwd()

	// Create detailed status display
	for _, wt := range worktrees {
		// Apply branch filter
		if options.BranchFilter != "" && !strings.Contains(wt.Branch, options.BranchFilter) {
			continue
		}

		// Check if this is current worktree
		isCurrent := strings.HasPrefix(currentDir, wt.Path)
		if options.CurrentOnly && !isCurrent {
			continue
		}

		// Display worktree header
		header := wt.Branch
		if isCurrent {
			header += " (current)"
		}
		if wt.IsMainRepo {
			header += " [main repository]"
		}

		m.ui.Header("%s", header)
		m.ui.Info("Path: %s", wt.Path)

		// Get detailed status if not main repo
		if !wt.IsMainRepo {
			if status, err := m.repo.GetWorktreeStatus(wt.Path); err == nil {
				if status.IsClean {
					m.ui.Success("Status: Clean")
				} else {
					m.ui.Warning("Status: Dirty (%d changed files)", status.ChangedFiles)
					if options.Verbose && status.ChangedFiles < 10 {
						// Show changed files if not too many
						// Note: This would need the git status to include file names
						m.ui.Info("Changed files: %d", status.ChangedFiles)
					}
				}

				// Show branch relationship info in verbose mode
				if options.Verbose {
					if status.Ahead > 0 {
						m.ui.Info("Ahead of remote by %d commits", status.Ahead)
					}
					if status.Behind > 0 {
						m.ui.Warning("Behind remote by %d commits", status.Behind)
					}
					if status.Ahead == 0 && status.Behind == 0 {
						m.ui.Success("Up to date with remote")
					}
				}
			} else {
				m.ui.Error("Failed to get status: %v", err)
			}
		}

		m.ui.Info("") // Add spacing between worktrees
	}

	return nil
}

// Cleanup performs intelligent cleanup of worktrees
func (m *Manager) Cleanup(options CleanupOptions) error {
	m.ui.Header("Smart Worktree Cleanup")

	worktrees, err := m.repo.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		m.ui.Info("No worktrees found")
		return nil
	}

	// Find cleanup candidates with spinner
	spinner := m.ui.NewSpinner("Analyzing worktrees for cleanup candidates...")
	spinner.Start()
	candidates, err := m.findCleanupCandidates(worktrees, options)
	if err != nil {
		spinner.ErrorStop("Failed to analyze worktrees")
		return fmt.Errorf("failed to find cleanup candidates: %w", err)
	}
	spinner.SuccessStop(fmt.Sprintf("Found %d cleanup candidates", len(candidates)))

	if len(candidates) == 0 {
		m.ui.Success("No worktrees found that need cleanup")
		return nil
	}

	// Display candidates
	if options.DryRun || options.Verbose {
		m.ui.Header("Cleanup Candidates")
		table := m.ui.NewTable()
		table.SetHeaders("Branch", "Path", "Reason", "Last Activity")

		for _, candidate := range candidates {
			table.AddRow(
				candidate.Branch,
				candidate.Path,
				candidate.Reason,
				candidate.LastActivity,
			)
		}
		table.Render()
	}

	if options.DryRun {
		m.ui.Info("Dry run: %d worktrees would be cleaned up", len(candidates))
		return nil
	}

	// Confirm cleanup unless auto mode
	if !options.Auto {
		if err := m.ui.Confirm(fmt.Sprintf("Clean up %d worktrees?", len(candidates))); err != nil {
			m.ui.Info("Cleanup cancelled")
			return nil
		}
	}

	// Perform cleanup
	cleaned := 0
	for _, candidate := range candidates {
		m.ui.Info("Cleaning up %s...", candidate.Branch)

		deleteOptions := DeleteOptions{
			DeleteBranch: candidate.ShouldDeleteBranch,
			Force:        true,
			IgnoreDirty:  true,
		}

		if err := m.Delete(candidate.Branch, deleteOptions); err != nil {
			m.ui.Warning("Failed to clean up %s: %v", candidate.Branch, err)
		} else {
			cleaned++
		}
	}

	m.ui.Success("Cleaned up %d/%d worktrees", cleaned, len(candidates))
	return nil
}

// CleanupCandidate represents a worktree that could be cleaned up
type CleanupCandidate struct {
	Branch             string
	Path               string
	Reason             string
	LastActivity       string
	ShouldDeleteBranch bool
}

// findCleanupCandidates analyzes worktrees to find cleanup candidates
func (m *Manager) findCleanupCandidates(worktrees []*types.WorktreeInfo, options CleanupOptions) ([]CleanupCandidate, error) {
	var candidates []CleanupCandidate
	currentDir, _ := os.Getwd()

	for _, wt := range worktrees {
		// Skip main repository
		if wt.IsMainRepo {
			continue
		}

		// Skip current worktree for safety
		if strings.HasPrefix(currentDir, wt.Path) {
			continue
		}

		// Check if path still exists
		if !pathExists(wt.Path) {
			candidates = append(candidates, CleanupCandidate{
				Branch:             wt.Branch,
				Path:               wt.Path,
				Reason:             "Path no longer exists",
				LastActivity:       "N/A",
				ShouldDeleteBranch: false, // Don't delete branch if path is missing
			})
			continue
		}

		// Check if branch is merged (this would need git operations)
		if options.MergedOnly || !options.MergedOnly {
			// For now, we'll implement a basic check
			// In a full implementation, this would check git log to see if branch is merged
			isMerged, err := m.isBranchMerged(wt.Branch)
			if err == nil && isMerged {
				candidates = append(candidates, CleanupCandidate{
					Branch:             wt.Branch,
					Path:               wt.Path,
					Reason:             "Branch has been merged",
					LastActivity:       "N/A", // Would need to check git log
					ShouldDeleteBranch: true,
				})
				continue
			}
		}

		// Check age if specified
		if options.OlderThan != "" {
			// Parse duration and check file modification time
			// This is a simplified implementation
			if isOlderThan, _ := m.isWorktreeOlderThan(wt.Path, options.OlderThan); isOlderThan {
				candidates = append(candidates, CleanupCandidate{
					Branch:             wt.Branch,
					Path:               wt.Path,
					Reason:             fmt.Sprintf("Inactive for more than %s", options.OlderThan),
					LastActivity:       "N/A", // Would show actual date
					ShouldDeleteBranch: false,
				})
			}
		}
	}

	return candidates, nil
}

// isBranchMerged checks if a branch has been merged into main/master
func (m *Manager) isBranchMerged(branch string) (bool, error) {
	// This is a placeholder implementation
	// In reality, this would use git commands to check if the branch is merged
	// For now, return false to be safe
	return false, nil
}

// isWorktreeOlderThan checks if a worktree is older than the specified duration
func (m *Manager) isWorktreeOlderThan(path, duration string) (bool, error) {
	// This is a placeholder implementation
	// In reality, this would parse the duration and check file/git timestamps
	return false, nil
}

// helper methods

func (m *Manager) generateWorktreePath(branchName string) (string, error) {
	repoRoot, err := m.repo.GetRepoRoot()
	if err != nil {
		return "", err
	}

	parentDir := filepath.Dir(repoRoot)
	repoName := m.repo.GetRepoName()

	// Apply worktree pattern from project config
	pattern := m.projectConfig.WorktreePattern
	if pattern == "" {
		pattern = "{repo}-{branch}"
	}

	dirName := strings.ReplaceAll(pattern, "{repo}", repoName)
	dirName = strings.ReplaceAll(dirName, "{branch}", branchName)

	return filepath.Join(parentDir, dirName), nil
}

func (m *Manager) resolveWorktree(identifier string) (*types.WorktreeInfo, error) {
	worktrees, err := m.repo.ListWorktrees()
	if err != nil {
		return nil, err
	}

	// Try exact branch match first
	for _, wt := range worktrees {
		if wt.Branch == identifier {
			return wt, nil
		}
	}

	// Try path match
	for _, wt := range worktrees {
		if wt.Path == identifier || filepath.Base(wt.Path) == identifier {
			return wt, nil
		}
	}

	return nil, types.NewValidationError("resolve-worktree",
		fmt.Sprintf("worktree not found: %s", identifier), nil)
}

func (m *Manager) buildHookContext(event types.HookEvent, branch, worktreePath string) types.HookContext {
	repoRoot, _ := m.repo.GetRepoRoot()

	return types.HookContext{
		Event:        event,
		Branch:       branch,
		RepoPath:     repoRoot,
		WorktreePath: worktreePath,
		Environment:  make(map[string]string),
	}
}

func (m *Manager) executeHooks(event types.HookEvent, ctx types.HookContext) error {
	if m.projectConfig == nil || len(m.projectConfig.Hooks[event]) == 0 {
		return nil
	}

	timeout := m.configMgr.ResolveTimeout(m.globalConfig, m.projectConfig)
	allowFailure := m.configMgr.ResolveAllowFailure(m.globalConfig, m.projectConfig)

	runner := NewHookRunner(m.projectConfig, timeout, m.globalConfig.UI.Verbose, allowFailure)
	return runner.RunHooks(event, ctx)
}

func (m *Manager) handleFileOperations(worktreePath string) error {
	repoRoot, err := m.repo.GetRepoRoot()
	if err != nil {
		return err
	}

	// Copy files
	if len(m.projectConfig.CopyFiles) > 0 {
		m.ui.Progress("Copying files...")
		if err := m.fileManager.CopyFiles(m.projectConfig.CopyFiles, repoRoot, worktreePath, m.projectConfig.IgnoreFiles); err != nil {
			return fmt.Errorf("copy files failed: %w", err)
		}
	}

	// Link files
	if len(m.projectConfig.LinkFiles) > 0 {
		m.ui.Progress("Creating file links...")
		if err := m.fileManager.LinkFiles(m.projectConfig.LinkFiles, repoRoot, worktreePath, m.projectConfig.IgnoreFiles); err != nil {
			return fmt.Errorf("link files failed: %w", err)
		}
	}

	return nil
}

func (m *Manager) shouldAutoOpenEditor() bool {
	return false // TODO: Add AutoOpen field to config if needed
}

func (m *Manager) openInEditor(path string) error {
	editor := m.configMgr.ResolveEditor(m.globalConfig, m.projectConfig)
	return m.openInSpecificEditor(path, editor)
}

// executeEditorCommand executes the editor command
func (m *Manager) executeEditorCommand(cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no editor command provided")
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	// For some editors, we want to run in background (detached)
	// For others (like vim/nano), we want to wait
	terminalEditors := map[string]bool{
		"vim":   true,
		"nvim":  true,
		"nano":  true,
		"emacs": true,
	}

	if terminalEditors[cmdArgs[0]] {
		// For terminal editors, run in foreground
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		// For GUI editors, run in background
		return cmd.Start()
	}
}

func (m *Manager) validateCreateOptions(branchName string, options CreateOptions) error {
	if branchName == "" {
		return types.NewValidationError("create-options", "branch name is required", nil)
	}

	if strings.ContainsAny(branchName, "/\\:*?\"<>|") {
		return types.NewValidationError("create-options", "branch name contains invalid characters", nil)
	}

	return nil
}

func (m *Manager) validateDeleteOptions(identifier string, options DeleteOptions) error {
	if identifier == "" {
		return types.NewValidationError("delete-options", "worktree identifier is required", nil)
	}
	return nil
}

func (m *Manager) validateMergeOptions(sourceBranch string, options MergeOptions) error {
	if sourceBranch == "" {
		return types.NewValidationError("merge-options", "source branch is required", nil)
	}
	return nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// atomicPathPreparation atomically checks and prepares the worktree path
// This fixes the TOCTOU race condition by performing check and creation atomically
func (m *Manager) atomicPathPreparation(worktreePath string, force bool) error {
	// Try to create the parent directory first
	parentDir := filepath.Dir(worktreePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Use os.Mkdir to atomically check if directory doesn't exist and create it
	err := os.Mkdir(worktreePath, 0755)
	if err == nil {
		// Successfully created directory, add cleanup to rollback
		m.rollback.AddFileCleanup(worktreePath)
		return nil
	}

	if !os.IsExist(err) {
		// Some other error occurred
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Directory already exists
	if !force {
		return types.NewFileSystemError("create-worktree", worktreePath,
			fmt.Sprintf("worktree path already exists: %s", worktreePath), nil)
	}

	// Force flag is set, remove existing path and try again
	m.ui.Warning("Removing existing path: %s", worktreePath)
	if err := os.RemoveAll(worktreePath); err != nil {
		return fmt.Errorf("failed to remove existing path: %w", err)
	}

	// Try creating again after removal
	if err := os.Mkdir(worktreePath, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory after cleanup: %w", err)
	}

	m.rollback.AddFileCleanup(worktreePath)
	return nil
}

// Interactive launches an interactive mode with fuzzy-finding for branch selection
func (m *Manager) Interactive(options InteractiveOptions) error {
	m.ui.Header("Interactive Mode")

	// Get all available branches
	branches, err := m.repo.ListBranches()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	// Get existing worktrees to filter out branches that already have worktrees
	worktrees, err := m.repo.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Create a set of existing branches with worktrees
	existingBranches := make(map[string]bool)
	for _, wt := range worktrees {
		existingBranches[wt.Branch] = true
	}

	var availableBranches []string
	var worktreeBranches []string

	for _, branch := range branches {
		if existingBranches[branch] {
			worktreeBranches = append(worktreeBranches, branch)
		} else {
			availableBranches = append(availableBranches, branch)
		}
	}

	// Determine mode
	var mode string
	var targetBranches []string

	switch {
	case options.CreateMode:
		mode = "CREATE"
		targetBranches = availableBranches
		m.ui.Info("Create mode: Select branches to create worktrees for")
	case options.CleanupMode:
		mode = "CLEANUP"
		targetBranches = worktreeBranches
		m.ui.Info("Cleanup mode: Select worktrees to clean up")
	case options.SwitchMode:
		mode = "SWITCH"
		targetBranches = worktreeBranches
		m.ui.Info("Switch mode: Select worktree to switch to")
	default:
		mode = "BROWSE"
		targetBranches = append(worktreeBranches, availableBranches...)
		m.ui.Info("Browse mode: View all branches and worktrees")
	}

	if len(targetBranches) == 0 {
		m.ui.Warning("No branches available for %s mode", mode)
		return nil
	}

	// Simple interactive selection (placeholder for fuzzy-finding)
	m.ui.Info("\nAvailable branches:")
	for i, branch := range targetBranches {
		status := ""
		if existingBranches[branch] {
			status = " [has worktree]"
		}
		m.ui.Info("  %d. %s%s", i+1, branch, status)
	}

	// For now, implement basic selection - in a real implementation,
	// we would use a library like github.com/manifoldco/promptui or
	// github.com/AlecAivazis/survey for fuzzy finding
	m.ui.Info("\nEnter the number of the branch you want to select (or press Enter to cancel):")

	var selection int
	if _, err := fmt.Scanln(&selection); err != nil {
		m.ui.Info("Selection cancelled")
		return nil
	}

	if selection < 1 || selection > len(targetBranches) {
		return fmt.Errorf("invalid selection: %d", selection)
	}

	selectedBranch := targetBranches[selection-1]
	m.ui.Success("Selected: %s", selectedBranch)

	// Execute the appropriate action based on mode
	switch mode {
	case "CREATE":
		if options.DryRun {
			m.ui.Info("[DRY RUN] Would create worktree for branch: %s", selectedBranch)
			return nil
		}
		createOpts := CreateOptions{
			CreateBranch: false, // Branch already exists
			DryRun:       options.DryRun,
		}
		return m.Create(selectedBranch, createOpts)

	case "CLEANUP":
		if options.DryRun {
			m.ui.Info("[DRY RUN] Would cleanup worktree for branch: %s", selectedBranch)
			return nil
		}
		deleteOpts := DeleteOptions{
			DeleteBranch: false, // Don't delete branch by default in cleanup
			DryRun:       options.DryRun,
		}
		return m.Delete(selectedBranch, deleteOpts)

	case "SWITCH":
		switchOpts := SwitchOptions{}
		return m.Switch(selectedBranch, switchOpts)

	case "BROWSE":
		// Show detailed information about the selected branch
		m.ui.Info("\nBranch Details: %s", selectedBranch)
		if existingBranches[selectedBranch] {
			// Find the worktree for this branch
			for _, wt := range worktrees {
				if wt.Branch == selectedBranch {
					m.ui.Info("  Path: %s", wt.Path)
					m.ui.Info("  Type: %s", func() string {
						if wt.IsMainRepo {
							return "Main Repository"
						}
						return "Worktree"
					}())
					break
				}
			}
		} else {
			m.ui.Info("  Status: No worktree (available for creation)")
		}
		return nil
	}

	return nil
}

// OpenInEditors opens a worktree in multiple editors simultaneously
func (m *Manager) OpenInEditors(identifier string, options EditorsOptions) error {
	// Resolve the worktree
	var worktreePath string

	if identifier == "." {
		// Current directory - resolve to worktree path
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		worktreePath = currentDir
	} else {
		// Resolve identifier to worktree info
		worktree, err := m.resolveWorktree(identifier)
		if err != nil {
			return err
		}
		worktreePath = worktree.Path
	}

	m.ui.Header("Opening worktree in editors: %s", worktreePath)

	// Parse editors list
	var editorsToOpen []string
	if options.Editors != "" {
		editorsToOpen = strings.Split(options.Editors, ",")
		for i, editor := range editorsToOpen {
			editorsToOpen[i] = strings.TrimSpace(editor)
		}
	} else {
		// Use default editor from config
		defaultEditor := m.configMgr.ResolveEditor(m.globalConfig, m.projectConfig)
		editorsToOpen = []string{defaultEditor}
	}

	// Open each editor
	for _, editor := range editorsToOpen {
		if err := m.openInSpecificEditor(worktreePath, editor); err != nil {
			m.ui.Warning("Failed to open in %s: %v", editor, err)
		}
	}

	// Open terminal if requested
	if options.OpenTerminal {
		if err := m.openTerminal(worktreePath); err != nil {
			m.ui.Warning("Failed to open terminal: %v", err)
		}
	}

	m.ui.Success("Opened worktree in %d editor(s)", len(editorsToOpen))
	return nil
}

// openInSpecificEditor opens a path in a specific editor
func (m *Manager) openInSpecificEditor(path, editor string) error {
	m.ui.Info("Opening in %s: %s", editor, path)

	// Map of common editors and their command patterns
	editorCommands := map[string][]string{
		"code":     {"code", path},
		"cursor":   {"cursor", path},
		"vim":      {"vim", path},
		"nvim":     {"nvim", path},
		"nano":     {"nano", path},
		"emacs":    {"emacs", path},
		"subl":     {"subl", path}, // Sublime Text
		"atom":     {"atom", path},
		"webstorm": {"webstorm", path},
		"idea":     {"idea", path},
		"pycharm":  {"pycharm", path},
		"goland":   {"goland", path},
		"fleet":    {"fleet", path},
		"zed":      {"zed", path},
	}

	// Check if we have a predefined command for this editor
	if cmdArgs, exists := editorCommands[editor]; exists {
		return m.executeEditorCommand(cmdArgs)
	}

	// For custom editors, assume the editor name is the command
	// and pass the path as an argument
	return m.executeEditorCommand([]string{editor, path})
}

// openTerminal opens a terminal in the specified path
func (m *Manager) openTerminal(path string) error {
	m.ui.Info("Opening terminal: %s", path)

	// Map of common terminal applications by OS
	terminalCommands := map[string][]string{
		// macOS
		"Terminal.app": {"open", "-a", "Terminal", path},
		"iTerm.app":    {"open", "-a", "iTerm", path},
		"Alacritty":    {"alacritty", "--working-directory", path},
		"Kitty":        {"kitty", "--directory", path},

		// Linux/Windows (simplified)
		"gnome-terminal": {"gnome-terminal", "--working-directory=" + path},
		"xterm":          {"xterm", "-e", "cd " + path + " && bash"},
		"wt":             {"wt", "-d", path}, // Windows Terminal
	}

	// Try common terminals in order of preference
	preferredTerminals := []string{"iTerm.app", "Terminal.app", "Alacritty", "Kitty", "gnome-terminal", "wt", "xterm"}

	for _, terminal := range preferredTerminals {
		if cmdArgs, exists := terminalCommands[terminal]; exists {
			if err := m.executeEditorCommand(cmdArgs); err == nil {
				return nil
			}
		}
	}

	return fmt.Errorf("no suitable terminal application found")
}

// GetRepo returns the underlying git repository (for completion and advanced operations)
func (m *Manager) GetRepo() git.Repository {
	return m.repo
}

// getOperationTimeout returns the timeout for operations
func (m *Manager) getOperationTimeout() time.Duration {
	if m.globalConfig != nil && m.globalConfig.Performance.OperationTimeout > 0 {
		return m.globalConfig.Performance.OperationTimeout
	}
	return 30 * time.Second // Default timeout
}
