# Revised Worktree Operations - Generic Implementation

## Core Philosophy Change

WTree operations are now **purely focused on git worktree management** with project-specific behavior delegated to `.wtreerc` hooks. This creates clean separation between tool functionality and project needs.

## 1. Revised Operation Lifecycle

### 1.1 Generic Operation Pattern
```go
type Operation struct {
    Name          string
    PreHook       HookEvent
    PostHook      HookEvent
    CoreLogic     func(ctx *OperationContext) error
    Rollback      func(ctx *OperationContext) error
}

func (op *Operation) Execute(ctx *OperationContext) error {
    // 1. Execute pre-hook (project-defined)
    if err := ctx.ExecuteHooks(op.PreHook); err != nil {
        return fmt.Errorf("pre-hook failed: %w", err)
    }
    
    // 2. Execute core WTree logic
    if err := op.CoreLogic(ctx); err != nil {
        // Attempt rollback if core logic fails
        if op.Rollback != nil {
            if rollbackErr := op.Rollback(ctx); rollbackErr != nil {
                return fmt.Errorf("operation failed and rollback failed: %v, %w", err, rollbackErr)
            }
        }
        return err
    }
    
    // 3. Execute post-hook (project-defined)
    if err := ctx.ExecuteHooks(op.PostHook); err != nil {
        // Post-hook failure is non-fatal by default
        ctx.UI.Warning("Post-hook failed: %v", err)
        if !ctx.ProjectConfig.AllowFailure {
            return fmt.Errorf("post-hook failed: %w", err)
        }
    }
    
    return nil
}
```

### 1.2 Operation Context
```go
type OperationContext struct {
    // Git operations
    Git GitRepository
    
    // Configuration
    GlobalConfig  *WTreeConfig
    ProjectConfig *ProjectConfig
    
    // Path management
    RepoPath      string
    WorktreePath  string
    Branch        string
    TargetBranch  string  // For merge operations
    
    // UI and logging
    UI            UIManager
    Logger        Logger
    
    // Hook execution
    HookExecutor  *HookExecutor
    
    // Operation tracking
    rollbacks     []func() error
}

func (ctx *OperationContext) ExecuteHooks(event HookEvent) error {
    return ctx.HookExecutor.ExecuteHooks(event, HookContext{
        Event:        event,
        WorktreePath: ctx.WorktreePath,
        RepoPath:     ctx.RepoPath,
        Branch:       ctx.Branch,
        TargetBranch: ctx.TargetBranch,
        Environment:  ctx.buildHookEnvironment(),
    })
}
```

## 2. Create Operation (Revised)

### 2.1 Core Create Logic
```go
func CreateWorktree(ctx *OperationContext) error {
    op := &Operation{
        Name:      "create_worktree",
        PreHook:   HookPreCreate,
        PostHook:  HookPostCreate,
        CoreLogic: executeCreateCore,
        Rollback:  rollbackCreate,
    }
    
    return op.Execute(ctx)
}

func executeCreateCore(ctx *OperationContext) error {
    // 1. Calculate worktree path using project pattern
    ctx.WorktreePath = calculateWorktreePath(ctx.RepoPath, ctx.Branch, ctx.ProjectConfig.WorktreePattern)
    
    // 2. Validate worktree creation
    if err := validateCreateWorktree(ctx); err != nil {
        return err
    }
    
    // 3. Create branch if it doesn't exist
    if !ctx.Git.BranchExists(ctx.Branch) {
        if err := ctx.Git.CreateBranch(ctx.Branch, ctx.Git.GetCurrentBranch()); err != nil {
            return fmt.Errorf("failed to create branch: %w", err)
        }
        // Register rollback for branch creation
        ctx.registerRollback(func() error {
            return ctx.Git.DeleteBranch(ctx.Branch, true)
        })
    }
    
    // 4. Create git worktree
    if err := ctx.Git.CreateWorktree(ctx.WorktreePath, ctx.Branch); err != nil {
        return fmt.Errorf("failed to create worktree: %w", err)
    }
    // Register rollback for worktree creation
    ctx.registerRollback(func() error {
        return ctx.Git.RemoveWorktree(ctx.WorktreePath, true)
    })
    
    // 5. Perform file operations (copy/link files as defined in .wtreerc)
    if err := performFileOperations(ctx); err != nil {
        ctx.UI.Warning("File operations failed: %v", err)
        // Non-fatal - continue with worktree creation
    }
    
    // Success - clear rollbacks
    ctx.clearRollbacks()
    
    ctx.UI.Success("Worktree created: %s", ctx.WorktreePath)
    return nil
}
```

### 2.2 File Operations (Generic Implementation)
```go
func performFileOperations(ctx *OperationContext) error {
    var errs []error
    
    // Copy files as defined in project config
    for _, pattern := range ctx.ProjectConfig.CopyFiles {
        if err := copyFiles(pattern, ctx.RepoPath, ctx.WorktreePath, ctx.ProjectConfig.IgnoreFiles); err != nil {
            errs = append(errs, fmt.Errorf("copy %s: %w", pattern, err))
        }
    }
    
    // Create symlinks as defined in project config
    for _, pattern := range ctx.ProjectConfig.LinkFiles {
        if err := linkFiles(pattern, ctx.RepoPath, ctx.WorktreePath, ctx.ProjectConfig.IgnoreFiles); err != nil {
            errs = append(errs, fmt.Errorf("link %s: %w", pattern, err))
        }
    }
    
    return errors.Join(errs...)
}

func copyFiles(pattern, srcDir, dstDir string, ignorePatterns []string) error {
    matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
    if err != nil {
        return err
    }
    
    for _, srcPath := range matches {
        // Calculate relative path
        relPath, err := filepath.Rel(srcDir, srcPath)
        if err != nil {
            continue
        }
        
        // Check if file should be ignored
        if shouldIgnoreFile(relPath, ignorePatterns) {
            continue
        }
        
        dstPath := filepath.Join(dstDir, relPath)
        
        // Skip if source doesn't exist
        if !fileExists(srcPath) {
            continue
        }
        
        // Create destination directory
        if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
            return err
        }
        
        // Copy file or directory
        if err := copyFileOrDir(srcPath, dstPath); err != nil {
            return err
        }
    }
    
    return nil
}

func linkFiles(pattern, srcDir, dstDir string, ignorePatterns []string) error {
    matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
    if err != nil {
        return err
    }
    
    for _, srcPath := range matches {
        relPath, err := filepath.Rel(srcDir, srcPath)
        if err != nil {
            continue
        }
        
        if shouldIgnoreFile(relPath, ignorePatterns) {
            continue
        }
        
        dstPath := filepath.Join(dstDir, relPath)
        
        if !fileExists(srcPath) {
            continue
        }
        
        // Create destination directory
        if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
            return err
        }
        
        // Create symlink
        if err := os.Symlink(srcPath, dstPath); err != nil {
            return err
        }
    }
    
    return nil
}
```

## 3. Delete Operation (Revised)

### 3.1 Core Delete Logic
```go
func DeleteWorktree(ctx *OperationContext) error {
    op := &Operation{
        Name:      "delete_worktree",
        PreHook:   HookPreDelete,
        PostHook:  HookPostDelete,
        CoreLogic: executeDeleteCore,
        // No rollback for delete - it's inherently destructive
    }
    
    return op.Execute(ctx)
}

func executeDeleteCore(ctx *OperationContext) error {
    // 1. Validate deletion is safe
    if err := validateDeleteWorktree(ctx); err != nil {
        return err
    }
    
    // 2. Remove git worktree registration
    if err := ctx.Git.RemoveWorktree(ctx.WorktreePath, ctx.GlobalConfig.ForceDelete); err != nil {
        ctx.UI.Warning("Git worktree removal failed: %v", err)
        // Continue with directory cleanup
    }
    
    // 3. Remove worktree directory
    if err := os.RemoveAll(ctx.WorktreePath); err != nil {
        return fmt.Errorf("failed to remove worktree directory: %w", err)
    }
    
    // 4. Delete branch if requested and safe
    if ctx.shouldDeleteBranch() {
        if err := ctx.Git.DeleteBranch(ctx.Branch, ctx.GlobalConfig.ForceDelete); err != nil {
            ctx.UI.Warning("Failed to delete branch '%s': %v", ctx.Branch, err)
        } else {
            ctx.UI.Success("Deleted branch: %s", ctx.Branch)
        }
    }
    
    ctx.UI.Success("Deleted worktree: %s", ctx.WorktreePath)
    return nil
}

func validateDeleteWorktree(ctx *OperationContext) error {
    // Safety check: not deleting current worktree
    currentDir, err := os.Getwd()
    if err != nil {
        return err
    }
    
    if strings.HasPrefix(currentDir, ctx.WorktreePath) {
        return NewSafetyError("Cannot delete current worktree. Please navigate elsewhere first.")
    }
    
    // Check for uncommitted changes (unless forced)
    if !ctx.GlobalConfig.ForceDelete {
        if status, err := ctx.Git.GetWorktreeStatus(ctx.WorktreePath); err == nil && !status.IsClean {
            return NewValidationError("Worktree has uncommitted changes. Use --force to override.")
        }
    }
    
    return nil
}
```

## 4. Merge Operation (Revised)

### 4.1 Core Merge Logic  
```go
func MergeWorktree(ctx *OperationContext) error {
    op := &Operation{
        Name:      "merge_worktree",
        PreHook:   HookPreMerge,
        PostHook:  HookPostMerge,
        CoreLogic: executeMergeCore,
        Rollback:  rollbackMerge,
    }
    
    return op.Execute(ctx)
}

func executeMergeCore(ctx *OperationContext) error {
    // 1. Validate merge operation
    if err := validateMerge(ctx); err != nil {
        return err
    }
    
    // 2. Auto-commit changes in source worktree if needed
    if err := autoCommitWorktreeChanges(ctx); err != nil {
        return fmt.Errorf("failed to auto-commit changes: %w", err)
    }
    
    // 3. Switch to target branch in main repo
    originalBranch := ctx.Git.GetCurrentBranch()
    if err := ctx.Git.Checkout(ctx.TargetBranch); err != nil {
        return fmt.Errorf("failed to checkout target branch: %w", err)
    }
    // Register rollback to restore original branch
    ctx.registerRollback(func() error {
        return ctx.Git.Checkout(originalBranch)
    })
    
    // 4. Perform merge
    mergeMsg := fmt.Sprintf("feat: merge '%s' into '%s'", ctx.Branch, ctx.TargetBranch)
    if err := ctx.Git.Merge(ctx.Branch, mergeMsg); err != nil {
        return fmt.Errorf("merge failed: %w", err)
    }
    
    ctx.UI.Success("Merged '%s' into '%s'", ctx.Branch, ctx.TargetBranch)
    
    // 5. Clean up ALL worktrees (the destructive part)
    if err := cleanupAllWorktrees(ctx); err != nil {
        ctx.UI.Warning("Worktree cleanup failed: %v", err)
        // Don't fail the merge operation for cleanup failures
    }
    
    // Success - clear rollbacks
    ctx.clearRollbacks()
    return nil
}

func cleanupAllWorktrees(ctx *OperationContext) error {
    worktrees, err := ctx.Git.ListWorktrees()
    if err != nil {
        return err
    }
    
    var errs []error
    for _, wt := range worktrees {
        if wt.IsMainRepo || wt.Branch == ctx.TargetBranch {
            continue // Skip main repo and target branch
        }
        
        // Create temporary context for cleanup
        cleanupCtx := &OperationContext{
            Git:           ctx.Git,
            GlobalConfig:  ctx.GlobalConfig,
            ProjectConfig: ctx.ProjectConfig,
            RepoPath:     ctx.RepoPath,
            WorktreePath: wt.Path,
            Branch:       wt.Branch,
            UI:           ctx.UI,
            Logger:       ctx.Logger,
            HookExecutor: ctx.HookExecutor,
        }
        
        if err := deleteWorktreeWithoutHooks(cleanupCtx); err != nil {
            errs = append(errs, fmt.Errorf("failed to cleanup %s: %w", wt.Branch, err))
        }
    }
    
    return errors.Join(errs...)
}
```

## 5. List Operation (Revised)

### 5.1 Generic List Implementation
```go
func ListWorktrees(ctx *OperationContext) error {
    // No hooks needed for list operation - it's read-only
    
    worktrees, err := ctx.Git.ListWorktrees()
    if err != nil {
        return fmt.Errorf("failed to list worktrees: %w", err)
    }
    
    if len(worktrees) == 0 {
        ctx.UI.Info("No worktrees found")
        return nil
    }
    
    // Collect status information
    statuses := make([]*WorktreeStatus, 0, len(worktrees))
    for _, wt := range worktrees {
        if wt.IsMainRepo {
            continue // Skip main repository in listing
        }
        
        status := &WorktreeStatus{
            Branch:     wt.Branch,
            Path:       wt.Path,
            Type:       determineWorktreeType(wt.Path),
            Status:     getWorktreeGitStatus(ctx.Git, wt.Path),
            PRNumber:   extractPRNumber(wt.Path),
            LastCommit: getLastCommitTime(ctx.Git, wt.Branch),
        }
        
        statuses = append(statuses, status)
    }
    
    // Display formatted list
    displayWorktreeList(ctx.UI, statuses)
    return nil
}

func determineWorktreeType(path string) string {
    // Check if this is a PR worktree based on path pattern
    if strings.Contains(filepath.Base(path), "-pr-") {
        return "pr"
    }
    return "branch"
}

func getWorktreeGitStatus(git GitRepository, path string) string {
    status, err := git.GetWorktreeStatus(path)
    if err != nil {
        return "unknown"
    }
    
    if !status.IsClean {
        return "dirty"
    }
    if status.Ahead > 0 {
        return "ahead"
    }
    if status.Behind > 0 {
        return "behind"
    }
    return "clean"
}
```

## 6. Hook Integration Points

### 6.1 Hook Execution Context
```go
func (ctx *OperationContext) buildHookEnvironment() map[string]string {
    env := make(map[string]string)
    
    // Copy current environment
    for _, e := range os.Environ() {
        pair := strings.SplitN(e, "=", 2)
        if len(pair) == 2 {
            env[pair[0]] = pair[1]
        }
    }
    
    // Add WTree-specific variables
    env["WTREE_BRANCH"] = ctx.Branch
    env["WTREE_REPO_PATH"] = ctx.RepoPath
    env["WTREE_WORKTREE_PATH"] = ctx.WorktreePath
    env["WTREE_TARGET_BRANCH"] = ctx.TargetBranch
    
    return env
}
```

### 6.2 Hook Command Expansion
```go
func expandHookCommand(cmd string, ctx HookContext) string {
    replacements := map[string]string{
        "{repo}":          filepath.Base(ctx.RepoPath),
        "{branch}":        ctx.Branch,
        "{target_branch}": ctx.TargetBranch,
        "{worktree_path}": ctx.WorktreePath,
        "{repo_path}":     ctx.RepoPath,
    }
    
    expanded := cmd
    for placeholder, value := range replacements {
        expanded = strings.ReplaceAll(expanded, placeholder, value)
    }
    
    return expanded
}
```

## 7. Error Handling in Generic Operations

### 7.1 Operation-Specific Error Types
```go
type OperationError struct {
    Operation string
    Phase     string  // "validation", "core", "hook"
    Cause     error
}

func NewOperationError(op, phase string, cause error) *OperationError {
    return &OperationError{
        Operation: op,
        Phase:     phase,
        Cause:     cause,
    }
}

func (oe *OperationError) Error() string {
    return fmt.Sprintf("%s operation failed in %s phase: %v", oe.Operation, oe.Phase, oe.Cause)
}
```

### 7.2 Hook Failure Handling
```go
func (ctx *OperationContext) handleHookFailure(event HookEvent, err error) error {
    if ctx.ProjectConfig.AllowFailure {
        ctx.UI.Warning("Hook %s failed but continuing due to allow_failure: %v", event, err)
        return nil
    }
    
    return NewOperationError("hook_execution", string(event), err)
}
```

This revised operations system maintains the core git worktree functionality while delegating all project-specific behavior to `.wtreerc` hooks, making WTree truly generic and extensible.