# Technical Architecture - Phase 1

## Core Design Principles

### 1. Fail-Fast Validation
All operations start with comprehensive validation before making any changes:
- Git repository state
- File system permissions
- Required tools availability
- Branch name validity
- Path conflicts

### 2. Transactional Operations
Each operation should be atomic where possible:
- Create all resources or none
- Rollback on failure
- Clear error states
- Consistent final state

### 3. Path Management Strategy
```go
type PathManager struct {
    RepoRoot    string  // /Users/awhite/Projects/myapp
    RepoName    string  // myapp
    ParentDir   string  // /Users/awhite/Projects
    WorktreeDir string  // /Users/awhite/Projects/myapp-feature-login
}

// Path calculation logic:
func (pm *PathManager) WorktreePath(branch string) string {
    // Sanitize branch name: feature/login -> feature-login
    clean := strings.ReplaceAll(branch, "/", "-")
    clean = strings.ReplaceAll(clean, "\\", "-")
    return filepath.Join(pm.ParentDir, pm.RepoName+"-"+clean)
}
```

### 4. Git Operations Abstraction
```go
type GitRepo interface {
    // Repository queries
    GetCurrentBranch() (string, error)
    BranchExists(name string) bool
    IsClean() bool
    GetRemotes() ([]Remote, error)
    
    // Branch operations
    CreateBranch(name, from string) error
    DeleteBranch(name string, force bool) error
    
    // Worktree operations
    CreateWorktree(path, branch string) error
    RemoveWorktree(path string, force bool) error
    ListWorktrees() ([]WorktreeInfo, error)
    
    // Advanced operations
    Merge(branch string, message string) error
    Fetch(remote string) error
    IsAhead(branch string) (int, error)
    IsBehind(branch string) (int, error)
}
```

## Detailed Component Design

### 1. Command Processing Pipeline
```go
// Each command follows this pipeline:
func ExecuteCommand(cmd *cobra.Command, args []string) error {
    // 1. Parse and validate arguments
    params, err := parseArguments(cmd, args)
    if err != nil {
        return NewUserError("invalid arguments", err)
    }
    
    // 2. Load configuration
    config, err := loadConfig(params.ConfigPath)
    if err != nil {
        return NewConfigError("config load failed", err)
    }
    
    // 3. Initialize context
    ctx, err := NewOperationContext(config, params)
    if err != nil {
        return NewValidationError("context init failed", err)
    }
    
    // 4. Pre-flight validation
    if err := ctx.Validate(); err != nil {
        return err
    }
    
    // 5. Execute operation with rollback
    return ctx.Execute()
}
```

### 2. Operation Context Design
```go
type OperationContext struct {
    Config     *Config
    Repo       GitRepo
    PathMgr    *PathManager
    UI         *UIManager
    GitHub     *GitHubClient  // nil if not needed
    
    // Operation tracking
    rollbacks  []func() error
    created    []string       // paths created
    modified   []string       // paths modified
}

func (ctx *OperationContext) Execute() error {
    defer ctx.rollbackOnPanic()
    
    // Register rollback for any created resources
    ctx.addRollback(func() error {
        return ctx.cleanupCreatedPaths()
    })
    
    // Execute the main operation
    return ctx.executeMain()
}
```

### 3. Environment Setup Architecture
```go
type EnvironmentSetup struct {
    WorktreePath string
    RepoPath     string
    Config       *Config
}

func (es *EnvironmentSetup) Setup() error {
    // Use goroutines for parallel operations where safe
    var g errgroup.Group
    
    // Sequential: Must happen first
    if err := es.copyEnvFile(); err != nil {
        return err
    }
    
    // Parallel: Independent operations
    g.Go(es.generateSSLCerts)
    g.Go(es.installComposerDeps)
    g.Go(es.installNodeDeps)
    
    // Wait for all parallel operations
    if err := g.Wait(); err != nil {
        return err
    }
    
    // Sequential: Final steps
    return es.writeMetadata()
}
```

### 4. GitHub Integration Design
```go
type GitHubClient struct {
    cli        *gh.Client     // GitHub CLI client
    httpClient *http.Client   // Direct API client (fallback)
    authCache  *AuthCache     // Authentication caching
}

type PRWorkflow struct {
    client    *GitHubClient
    repo      GitRepo
    pathMgr   *PathManager
}

func (pw *PRWorkflow) CreatePRWorktree(prNum int) error {
    // 1. Fetch PR info via gh CLI
    prInfo, err := pw.client.GetPR(prNum)
    if err != nil {
        return NewGitHubError("PR fetch failed", err)
    }
    
    // 2. Create tracking branch
    localBranch := fmt.Sprintf("pr-%d-%s", prNum, prInfo.Branch)
    fetchRef := fmt.Sprintf("pull/%d/head:%s", prNum, localBranch)
    
    if err := pw.repo.Fetch("origin", fetchRef); err != nil {
        return NewGitError("PR fetch failed", err)
    }
    
    // 3. Create worktree
    wtPath := pw.pathMgr.PRWorktreePath(prNum)
    if err := pw.repo.CreateWorktree(wtPath, localBranch); err != nil {
        return NewWorktreeError("worktree creation failed", err)
    }
    
    // 4. Setup environment and metadata
    return pw.setupPREnvironment(wtPath, prInfo)
}
```

## Error Handling Architecture

### 1. Error Type Hierarchy
```go
type WtreeError interface {
    error
    Type() ErrorType
    Operation() string
    Recoverable() bool
    SuggestedActions() []string
}

type ErrorType int
const (
    ErrorValidation ErrorType = iota
    ErrorGit
    ErrorFileSystem
    ErrorNetwork
    ErrorUser
    ErrorInternal
)

// Concrete error types
type ValidationError struct {
    op       string
    path     string
    cause    error
    suggestions []string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %v", e.op, e.cause)
}

func (e *ValidationError) SuggestedActions() []string {
    return e.suggestions
}
```

### 2. Recovery Strategies
```go
type RecoveryStrategy interface {
    CanRecover(err error) bool
    Recover(ctx *OperationContext, err error) error
}

// Example: Git worktree already exists
type WorktreeExistsRecovery struct{}

func (r *WorktreeExistsRecovery) Recover(ctx *OperationContext, err error) error {
    // Offer to remove existing and recreate
    if ctx.UI.Confirm("Worktree exists. Remove and recreate?") {
        // Implement removal and retry logic
        return ctx.retryOperation()
    }
    return err
}
```

## Configuration Architecture

### 1. Configuration Loading Strategy
```go
type ConfigLoader struct {
    sources []ConfigSource
}

type ConfigSource interface {
    Load() (*Config, error)
    Priority() int
    Available() bool
}

// Loading order (highest priority first):
var defaultSources = []ConfigSource{
    &FlagSource{},           // Command line flags
    &EnvSource{},            // Environment variables  
    &RepoConfigSource{},     // Repository .wtree.yaml
    &UserConfigSource{},     // ~/.config/wtree/config.yaml
    &DefaultSource{},        // Built-in defaults
}
```

### 2. Configuration Schema Validation
```go
type ConfigValidator struct {
    schema *jsonschema.Schema
}

func (cv *ConfigValidator) Validate(config *Config) error {
    // Validate required tools exist
    if config.InstallDependencies {
        for tool := range config.Installers {
            if !isCommandAvailable(tool) {
                return NewValidationError(
                    "missing dependency installer",
                    fmt.Errorf("%s not found in PATH", tool),
                )
            }
        }
    }
    
    // Validate paths are writable
    if err := validateWritablePath(config.WorktreeParent); err != nil {
        return NewValidationError("invalid worktree parent", err)
    }
    
    return nil
}
```

## Performance Considerations

### 1. Parallel Operations
- SSL certificate generation (background)
- Dependency installations (parallel: composer + bun)
- File system operations (where safe)
- Git operations (fetch while setting up environment)

### 2. Caching Strategy
```go
type OperationCache struct {
    gitInfo    *sync.Map  // Repository information
    prInfo     *sync.Map  // PR metadata cache
    toolCheck  *sync.Map  // Tool availability cache
}

// Cache git repository information to avoid repeated queries
func (cache *OperationCache) GetRepoInfo(path string) (*RepoInfo, error) {
    if cached, ok := cache.gitInfo.Load(path); ok {
        return cached.(*RepoInfo), nil
    }
    
    info, err := queryRepoInfo(path)
    if err == nil {
        cache.gitInfo.Store(path, info)
    }
    return info, err
}
```

### 3. Resource Management
```go
type ResourceManager struct {
    maxConcurrent int
    semaphore     chan struct{}
    cleanup       []func() error
}

func (rm *ResourceManager) AcquireResource() {
    rm.semaphore <- struct{}{}
}

func (rm *ResourceManager) ReleaseResource() {
    <-rm.semaphore
}

func (rm *ResourceManager) Cleanup() error {
    var errs []error
    for _, cleanup := range rm.cleanup {
        if err := cleanup(); err != nil {
            errs = append(errs, err)
        }
    }
    return errors.Join(errs...)
}
```

## Security Considerations

### 1. Path Traversal Prevention
```go
func sanitizeBranchName(branch string) string {
    // Remove path traversal attempts
    branch = filepath.Clean(branch)
    branch = strings.ReplaceAll(branch, "..", "")
    
    // Replace filesystem-unsafe characters
    unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
    for _, char := range unsafe {
        branch = strings.ReplaceAll(branch, char, "-")
    }
    
    return branch
}

func validateWorktreePath(path, repoRoot string) error {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return err
    }
    
    // Ensure worktree is not inside the main repository
    if strings.HasPrefix(absPath, repoRoot) {
        return errors.New("worktree cannot be inside main repository")
    }
    
    return nil
}
```

### 2. Command Injection Prevention
```go
func safeExecute(cmd string, args ...string) error {
    // Never use shell execution for user input
    command := exec.Command(cmd, args...)
    
    // Set secure environment
    command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
    
    return command.Run()
}
```

This architecture provides a solid foundation for Phase 1 implementation with proper separation of concerns, comprehensive error handling, and security considerations.