# Core Worktree Operations - Detailed Implementation

## Operation Lifecycle Overview

Each worktree operation follows a consistent lifecycle:
1. **Validation Phase** - Pre-flight checks
2. **Planning Phase** - Determine what needs to be done
3. **Execution Phase** - Perform operations with rollback support
4. **Finalization Phase** - Cleanup and user notification

## 1. Worktree Creation (`wtree [branch]`)

### 1.1 Validation Phase
```go
type CreateValidation struct {
    repo         *GitRepo
    branchName   string
    targetPath   string
    config       *Config
}

func (cv *CreateValidation) Validate() error {
    checks := []ValidationCheck{
        cv.validateGitRepo,
        cv.validateBranchName,
        cv.validateTargetPath,
        cv.validatePermissions,
        cv.validateDependencyTools,
        cv.validateDiskSpace,
    }
    
    for _, check := range checks {
        if err := check(); err != nil {
            return err
        }
    }
    return nil
}

func (cv *CreateValidation) validateBranchName() error {
    // Git branch name validation
    if strings.Contains(cv.branchName, "..") {
        return NewValidationError("invalid branch name: contains '..'")
    }
    
    // Filesystem safety
    if strings.ContainsAny(cv.branchName, `<>:"|?*`) {
        return NewValidationError("branch name contains filesystem-unsafe characters")
    }
    
    return nil
}

func (cv *CreateValidation) validateTargetPath() error {
    // Check if worktree already exists
    if exists, err := pathExists(cv.targetPath); err != nil {
        return NewValidationError("cannot check target path", err)
    } else if exists {
        return NewWorktreeExistsError(cv.targetPath)
    }
    
    // Check parent directory is writable
    parent := filepath.Dir(cv.targetPath)
    if !isWritable(parent) {
        return NewValidationError("parent directory not writable", 
            fmt.Errorf("cannot write to %s", parent))
    }
    
    return nil
}
```

### 1.2 Planning Phase
```go
type CreatePlan struct {
    BranchExists     bool
    CreateBranch     bool
    SourceBranch     string
    WorktreePath     string
    EnvSetupTasks    []EnvTask
    PostCreateTasks  []PostTask
}

func planCreate(ctx *OperationContext, branchName string) (*CreatePlan, error) {
    plan := &CreatePlan{
        WorktreePath: ctx.PathMgr.WorktreePath(branchName),
    }
    
    // Check if branch exists
    plan.BranchExists = ctx.Repo.BranchExists(branchName)
    plan.CreateBranch = !plan.BranchExists
    plan.SourceBranch = ctx.Repo.GetCurrentBranch()
    
    // Plan environment setup
    if ctx.Config.CopyEnv {
        plan.EnvSetupTasks = append(plan.EnvSetupTasks, 
            &CopyEnvTask{})
    }
    
    if ctx.Config.GenerateSSL {
        plan.EnvSetupTasks = append(plan.EnvSetupTasks,
            &GenerateSSLTask{})
    }
    
    if ctx.Config.InstallDependencies {
        plan.EnvSetupTasks = append(plan.EnvSetupTasks,
            &InstallDepsTask{})
    }
    
    // Plan post-create tasks
    if ctx.Config.Editor != "" {
        plan.PostCreateTasks = append(plan.PostCreateTasks,
            &OpenEditorTask{Editor: ctx.Config.Editor})
    }
    
    return plan, nil
}
```

### 1.3 Execution Phase
```go
func executeCreate(ctx *OperationContext, plan *CreatePlan) error {
    // Create execution tracker
    tracker := NewExecutionTracker()
    defer tracker.Rollback() // Rollback on panic or error
    
    // Step 1: Create branch if needed
    if plan.CreateBranch {
        if err := ctx.Repo.CreateBranch(plan.BranchExists, plan.SourceBranch); err != nil {
            return NewGitError("branch creation failed", err)
        }
        tracker.RegisterRollback(func() error {
            return ctx.Repo.DeleteBranch(plan.BranchExists, true)
        })
    }
    
    // Step 2: Create worktree
    if err := ctx.Repo.CreateWorktree(plan.WorktreePath, plan.BranchExists); err != nil {
        return NewGitError("worktree creation failed", err)
    }
    tracker.RegisterRollback(func() error {
        return ctx.Repo.RemoveWorktree(plan.WorktreePath, true)
    })
    
    // Step 3: Environment setup (parallel where possible)
    if err := executeEnvironmentSetup(ctx, plan.EnvSetupTasks); err != nil {
        return NewEnvSetupError("environment setup failed", err)
    }
    
    // Step 4: Post-create tasks
    if err := executePostCreateTasks(ctx, plan.PostCreateTasks); err != nil {
        // Non-fatal - log warning but continue
        ctx.UI.Warning("post-create tasks failed: %v", err)
    }
    
    // Success - disable rollback
    tracker.DisableRollback()
    return nil
}
```

### 1.4 Environment Setup Details
```go
type EnvSetupExecutor struct {
    worktreePath string
    repoPath     string
    config       *Config
    ui           *UIManager
}

func (ese *EnvSetupExecutor) Execute(tasks []EnvTask) error {
    // Group tasks by execution strategy
    sequential := []EnvTask{}
    parallel := []EnvTask{}
    
    for _, task := range tasks {
        if task.RequiresSequential() {
            sequential = append(sequential, task)
        } else {
            parallel = append(parallel, task)
        }
    }
    
    // Execute sequential tasks first
    for _, task := range sequential {
        if err := ese.executeTask(task); err != nil {
            return err
        }
    }
    
    // Execute parallel tasks
    return ese.executeParallelTasks(parallel)
}

func (ese *EnvSetupExecutor) executeParallelTasks(tasks []EnvTask) error {
    var g errgroup.Group
    g.SetLimit(3) // Limit concurrency
    
    for _, task := range tasks {
        task := task // Capture for goroutine
        g.Go(func() error {
            return ese.executeTask(task)
        })
    }
    
    return g.Wait()
}

// Specific task implementations
type CopyEnvTask struct{}

func (cet *CopyEnvTask) Execute(ctx *EnvContext) error {
    srcPath := filepath.Join(ctx.RepoPath, ".env")
    dstPath := filepath.Join(ctx.WorktreePath, ".env")
    
    if !fileExists(srcPath) {
        return nil // Not an error - just skip
    }
    
    if err := copyFile(srcPath, dstPath); err != nil {
        return fmt.Errorf("failed to copy .env: %w", err)
    }
    
    // Set restrictive permissions
    return os.Chmod(dstPath, 0600)
}

type GenerateSSLTask struct{}

func (gst *GenerateSSLTask) Execute(ctx *EnvContext) error {
    certDir := filepath.Join(ctx.WorktreePath, "storage", "certs")
    if err := os.MkdirAll(certDir, 0755); err != nil {
        return fmt.Errorf("failed to create cert directory: %w", err)
    }
    
    // Generate self-signed certificate
    return generateSelfSignedCert(
        filepath.Join(certDir, "aws-cloud-front-private.pem"),
        filepath.Join(certDir, "aws-cloud-front-private-cert.pem"),
        ctx.Config.SSLSubject,
    )
}

type InstallDepsTask struct{}

func (idt *InstallDepsTask) Execute(ctx *EnvContext) error {
    // Run dependency installers in parallel
    var g errgroup.Group
    
    for tool, commands := range ctx.Config.Installers {
        if !isCommandAvailable(tool) {
            ctx.UI.Warning("Skipping %s: command not found", tool)
            continue
        }
        
        tool, commands := tool, commands // Capture for goroutine
        g.Go(func() error {
            return runInstaller(ctx.WorktreePath, tool, commands)
        })
    }
    
    return g.Wait()
}
```

## 2. Worktree Deletion (`wtree delete`)

### 2.1 Deletion Safety Checks
```go
type DeleteValidation struct {
    branches     []string
    worktrees    map[string]*WorktreeInfo
    currentPath  string
}

func (dv *DeleteValidation) Validate() error {
    for _, branch := range dv.branches {
        wt, exists := dv.worktrees[branch]
        if !exists {
            return NewValidationError(fmt.Sprintf("no worktree for branch '%s'", branch))
        }
        
        // Safety: Don't delete current worktree
        if strings.HasPrefix(dv.currentPath, wt.Path) {
            return NewSafetyError(fmt.Sprintf(
                "cannot delete current worktree '%s'\nPlease navigate elsewhere first", 
                wt.Path))
        }
        
        // Check for unmerged changes
        if !wt.IsClean && !dv.force {
            return NewUnmergedChangesError(branch, wt.Path)
        }
    }
    
    return nil
}
```

### 2.2 Interactive Confirmation
```go
func confirmDeletion(ui *UIManager, worktrees []*WorktreeInfo, keepBranches bool) error {
    ui.Info("The following worktrees will be deleted:")
    
    for _, wt := range worktrees {
        status := "clean"
        if !wt.IsClean {
            status = fmt.Sprintf("dirty (%d files changed)", wt.ChangedFiles)
        }
        
        ui.InfoIndented("• %s (%s) - %s", wt.Branch, wt.Path, status)
        
        if !keepBranches && wt.Branch != "main" {
            ui.InfoIndented("  └── Branch '%s' will also be deleted", wt.Branch)
        }
    }
    
    return ui.Confirm("Continue with deletion?")
}
```

### 2.3 Deletion Execution
```go
func executeDelete(ctx *OperationContext, worktrees []*WorktreeInfo, keepBranches bool) error {
    var errs []error
    
    for _, wt := range worktrees {
        if err := deleteSingleWorktree(ctx, wt, keepBranches); err != nil {
            errs = append(errs, fmt.Errorf("failed to delete %s: %w", wt.Branch, err))
            continue
        }
        
        ctx.UI.Success("Deleted worktree: %s", wt.Path)
    }
    
    return errors.Join(errs...)
}

func deleteSingleWorktree(ctx *OperationContext, wt *WorktreeInfo, keepBranch bool) error {
    // 1. Remove worktree registration
    if err := ctx.Repo.RemoveWorktree(wt.Path, true); err != nil {
        // Log but don't fail - continue with directory cleanup
        ctx.UI.Warning("Git worktree removal failed: %v", err)
    }
    
    // 2. Remove leftover directory
    if err := os.RemoveAll(wt.Path); err != nil {
        return fmt.Errorf("failed to remove directory: %w", err)
    }
    
    // 3. Delete branch if requested
    if !keepBranch && wt.Branch != "main" {
        if err := ctx.Repo.DeleteBranch(wt.Branch, ctx.Config.ForceDelete); err != nil {
            ctx.UI.Warning("Failed to delete branch '%s': %v", wt.Branch, err)
        } else {
            ctx.UI.Success("Deleted branch: %s", wt.Branch)
        }
    }
    
    return nil
}
```

## 3. Worktree Merging (`wtree merge`)

### 3.1 Merge Validation and Planning
```go
type MergeValidation struct {
    sourceBranch string
    targetBranch string
    worktree     *WorktreeInfo
    repo         *GitRepo
}

func (mv *MergeValidation) Validate() error {
    // Source worktree must exist
    if mv.worktree == nil {
        return NewValidationError(fmt.Sprintf("no worktree for branch '%s'", mv.sourceBranch))
    }
    
    // Target branch must exist
    if !mv.repo.BranchExists(mv.targetBranch) {
        return NewValidationError(fmt.Sprintf("target branch '%s' does not exist", mv.targetBranch))
    }
    
    // Check for conflicts before starting
    conflicts, err := mv.repo.PredictMergeConflicts(mv.sourceBranch, mv.targetBranch)
    if err != nil {
        return NewGitError("conflict prediction failed", err)
    }
    
    if len(conflicts) > 0 {
        return NewMergeConflictsError(conflicts)
    }
    
    return nil
}
```

### 3.2 Auto-commit Uncommitted Changes
```go
func autoCommitChanges(ctx *OperationContext, worktreePath string) error {
    // Switch to worktree context
    oldDir, err := os.Getwd()
    if err != nil {
        return err
    }
    defer os.Chdir(oldDir)
    
    if err := os.Chdir(worktreePath); err != nil {
        return fmt.Errorf("cannot access worktree: %w", err)
    }
    
    // Check for uncommitted changes
    status, err := ctx.Repo.GetStatus()
    if err != nil {
        return fmt.Errorf("cannot get git status: %w", err)
    }
    
    if status.IsClean() {
        return nil // Nothing to commit
    }
    
    // Add all changes
    if err := ctx.Repo.AddAll(); err != nil {
        return fmt.Errorf("failed to stage changes: %w", err)
    }
    
    // Commit with descriptive message
    message := fmt.Sprintf("chore: auto-commit before merge\n\nAuto-committed by wtree before merging to %s", 
        ctx.targetBranch)
    
    if err := ctx.Repo.Commit(message); err != nil {
        return fmt.Errorf("failed to commit changes: %w", err)
    }
    
    ctx.UI.Success("Auto-committed uncommitted changes")
    return nil
}
```

### 3.3 Merge Execution and Cleanup
```go
func executeMerge(ctx *OperationContext, sourceBranch, targetBranch string) error {
    // 1. Auto-commit changes in source worktree
    worktree := ctx.findWorktree(sourceBranch)
    if err := autoCommitChanges(ctx, worktree.Path); err != nil {
        return fmt.Errorf("auto-commit failed: %w", err)
    }
    
    // 2. Switch to target branch in main repo
    if err := ctx.Repo.Checkout(targetBranch); err != nil {
        return NewGitError("checkout failed", err)
    }
    
    // 3. Merge the source branch
    mergeMsg := fmt.Sprintf("feat: merge '%s' into '%s'", sourceBranch, targetBranch)
    if err := ctx.Repo.Merge(sourceBranch, mergeMsg); err != nil {
        return NewMergeError("merge failed", err)
    }
    
    ctx.UI.Success("Merged '%s' into '%s'", sourceBranch, targetBranch)
    
    // 4. Cleanup ALL worktrees (the destructive part!)
    return cleanupAllWorktrees(ctx, targetBranch)
}

func cleanupAllWorktrees(ctx *OperationContext, protectedBranch string) error {
    worktrees, err := ctx.Repo.ListWorktrees()
    if err != nil {
        return fmt.Errorf("failed to list worktrees: %w", err)
    }
    
    var errs []error
    for _, wt := range worktrees {
        if wt.IsMainRepo || wt.Branch == protectedBranch {
            continue // Skip main repo and target branch
        }
        
        if err := deleteSingleWorktree(ctx, wt, false); err != nil {
            errs = append(errs, err)
        }
    }
    
    return errors.Join(errs...)
}
```

## 4. Worktree Listing (`wtree list`)

### 4.1 Status Collection
```go
type WorktreeStatus struct {
    Branch       string
    Path         string
    Type         string  // "branch", "pr"
    Status       string  // "clean", "dirty", "ahead", "behind"
    ChangedFiles int
    Ahead        int
    Behind       int
    PRNumber     int     // if type == "pr"
    LastCommit   time.Time
}

func collectWorktreeStatus(ctx *OperationContext) ([]*WorktreeStatus, error) {
    worktrees, err := ctx.Repo.ListWorktrees()
    if err != nil {
        return nil, err
    }
    
    var statuses []*WorktreeStatus
    var g errgroup.Group
    var mu sync.Mutex
    
    for _, wt := range worktrees {
        if wt.IsMainRepo {
            continue // Skip main repository
        }
        
        wt := wt // Capture for goroutine
        g.Go(func() error {
            status, err := getDetailedWorktreeStatus(ctx, wt)
            if err != nil {
                return err
            }
            
            mu.Lock()
            statuses = append(statuses, status)
            mu.Unlock()
            
            return nil
        })
    }
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    // Sort by branch name
    sort.Slice(statuses, func(i, j int) bool {
        return statuses[i].Branch < statuses[j].Branch
    })
    
    return statuses, nil
}
```

### 4.2 Rich Status Display
```go
func displayWorktreeList(ui *UIManager, statuses []*WorktreeStatus, detailed bool) {
    if len(statuses) == 0 {
        ui.Info("No active worktrees found")
        return
    }
    
    // Create table with dynamic sizing
    table := ui.NewTable()
    table.SetHeaders("BRANCH", "PATH", "STATUS", "TYPE")
    
    if detailed {
        table.SetHeaders("BRANCH", "PATH", "STATUS", "CHANGES", "TYPE")
    }
    
    for _, status := range statuses {
        row := []string{
            colorBranch(status.Branch),
            truncatePath(status.Path, 50),
            colorStatus(status.Status),
            formatType(status.Type, status.PRNumber),
        }
        
        if detailed {
            changes := formatChanges(status.ChangedFiles, status.Ahead, status.Behind)
            row = append(row[:3], changes, row[3])
        }
        
        table.AddRow(row...)
    }
    
    table.Render()
    ui.Info("\n%d worktrees active", len(statuses))
}

func colorStatus(status string) string {
    switch status {
    case "clean":
        return ui.Green("clean")
    case "dirty":
        return ui.Yellow("dirty")
    case "ahead":
        return ui.Blue("ahead")
    case "behind":
        return ui.Red("behind")
    default:
        return status
    }
}
```

This detailed implementation provides a robust foundation for all core worktree operations with proper error handling, rollback capabilities, and user-friendly interfaces.