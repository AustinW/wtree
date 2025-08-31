# Configuration & Error Handling Strategies

## Configuration Architecture

### 1. Configuration Hierarchy

Configuration is loaded in priority order (highest to lowest):
1. **Command-line flags** (highest priority)
2. **Environment variables** (`WTREE_*`)
3. **Repository configuration** (`.wtree.yaml` in repo root)
4. **User configuration** (`~/.config/wtree/config.yaml`)
5. **Built-in defaults** (lowest priority)

### 1.1 Configuration Structure
```go
type Config struct {
    // Core settings
    Editor              string            `yaml:"editor" env:"WTREE_EDITOR"`
    WorktreeParent      string            `yaml:"worktree_parent" env:"WTREE_PARENT"`
    InstallDependencies bool              `yaml:"install_dependencies" env:"WTREE_INSTALL_DEPS"`
    ParallelInstalls    bool              `yaml:"parallel_installs"`
    
    // Environment setup
    CopyEnv         bool              `yaml:"copy_env"`
    GenerateSSL     bool              `yaml:"generate_ssl"`
    SSLSubject      string            `yaml:"ssl_subject"`
    Installers      map[string][]string `yaml:"installers"`
    
    // GitHub settings
    GitHubCLI       string            `yaml:"github_cli"`
    PRBranchFormat  string            `yaml:"pr_branch_format"`
    
    // UI settings
    Colors          bool              `yaml:"colors"`
    ProgressBars    bool              `yaml:"progress_bars"`
    Verbose         bool              `yaml:"verbose"`
    ConfirmDestructive bool           `yaml:"confirm_destructive"`
    
    // Advanced settings
    MaxConcurrent   int               `yaml:"max_concurrent"`
    CacheTimeout    time.Duration     `yaml:"cache_timeout"`
    OperationTimeout time.Duration    `yaml:"operation_timeout"`
}

// Default configuration
var DefaultConfig = &Config{
    Editor:              "cursor",
    WorktreeParent:      "", // Auto-detect
    InstallDependencies: true,
    ParallelInstalls:    true,
    
    CopyEnv:         true,
    GenerateSSL:     true,
    SSLSubject:      "/C=US/ST=CA/L=City/O=Org/CN=localhost",
    
    Installers: map[string][]string{
        "composer": {"composer", "install"},
        "bun":      {"bun", "install"},
        "npm":      {"npm", "install"},
        "pip":      {"pip", "install", "-r", "requirements.txt"},
        "bundle":   {"bundle", "install"},
    },
    
    GitHubCLI:       "gh",
    PRBranchFormat:  "pr-{number}-{branch}",
    
    Colors:             true,
    ProgressBars:       true,
    Verbose:            false,
    ConfirmDestructive: true,
    
    MaxConcurrent:      3,
    CacheTimeout:       5 * time.Minute,
    OperationTimeout:   10 * time.Minute,
}
```

### 1.2 Configuration Loading Implementation
```go
type ConfigLoader struct {
    sources []ConfigSource
    cache   *Config
    mu      sync.RWMutex
}

type ConfigSource interface {
    Load() (*Config, error)
    Priority() int
    Name() string
    Available() bool
}

func NewConfigLoader() *ConfigLoader {
    return &ConfigLoader{
        sources: []ConfigSource{
            &FlagSource{},
            &EnvSource{},
            &RepoConfigSource{},
            &UserConfigSource{},
            &DefaultSource{},
        },
    }
}

func (cl *ConfigLoader) LoadConfig(repoPath string) (*Config, error) {
    cl.mu.Lock()
    defer cl.mu.Unlock()
    
    // Start with default config
    config := *DefaultConfig
    
    // Apply each source in reverse priority order (lowest to highest)
    sources := make([]ConfigSource, len(cl.sources))
    copy(sources, cl.sources)
    sort.Slice(sources, func(i, j int) bool {
        return sources[i].Priority() < sources[j].Priority()
    })
    
    for _, source := range sources {
        if !source.Available() {
            continue
        }
        
        sourceConfig, err := source.Load()
        if err != nil {
            return nil, fmt.Errorf("failed to load config from %s: %w", source.Name(), err)
        }
        
        if sourceConfig != nil {
            if err := mergeConfigs(&config, sourceConfig); err != nil {
                return nil, fmt.Errorf("failed to merge config from %s: %w", source.Name(), err)
            }
        }
    }
    
    // Validate final configuration
    if err := validateConfig(&config, repoPath); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }
    
    cl.cache = &config
    return &config, nil
}

// Configuration sources implementation
type UserConfigSource struct{}

func (ucs *UserConfigSource) Load() (*Config, error) {
    configDir, err := os.UserConfigDir()
    if err != nil {
        return nil, err
    }
    
    configPath := filepath.Join(configDir, "wtree", "config.yaml")
    if !fileExists(configPath) {
        return nil, nil // No config file
    }
    
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }
    
    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config file: %w", err)
    }
    
    return &config, nil
}

func (ucs *UserConfigSource) Priority() int { return 2 }
func (ucs *UserConfigSource) Name() string { return "user config" }
func (ucs *UserConfigSource) Available() bool {
    configDir, err := os.UserConfigDir()
    if err != nil {
        return false
    }
    configPath := filepath.Join(configDir, "wtree", "config.yaml")
    return fileExists(configPath)
}
```

### 1.3 Configuration Validation
```go
func validateConfig(config *Config, repoPath string) error {
    validators := []func(*Config, string) error{
        validatePaths,
        validateTools,
        validateInstallers,
        validateTimeouts,
        validateLimits,
    }
    
    for _, validator := range validators {
        if err := validator(config, repoPath); err != nil {
            return err
        }
    }
    
    return nil
}

func validateTools(config *Config, repoPath string) error {
    // Validate required tools exist
    requiredTools := []string{"git"}
    
    if config.InstallDependencies {
        // Check only configured installers
        for tool := range config.Installers {
            requiredTools = append(requiredTools, tool)
        }
    }
    
    if config.GitHubCLI != "" {
        requiredTools = append(requiredTools, config.GitHubCLI)
    }
    
    var missing []string
    for _, tool := range requiredTools {
        if !isCommandAvailable(tool) {
            missing = append(missing, tool)
        }
    }
    
    if len(missing) > 0 {
        return NewValidationError(fmt.Sprintf("missing required tools: %s", 
            strings.Join(missing, ", ")))
    }
    
    return nil
}

func validatePaths(config *Config, repoPath string) error {
    // Validate worktree parent directory
    if config.WorktreeParent != "" {
        if !filepath.IsAbs(config.WorktreeParent) {
            return NewValidationError("worktree_parent must be absolute path")
        }
        
        if err := ensureDirectoryExists(config.WorktreeParent); err != nil {
            return NewValidationError(fmt.Sprintf("worktree_parent not accessible: %v", err))
        }
    } else {
        // Auto-detect parent directory
        config.WorktreeParent = filepath.Dir(repoPath)
    }
    
    return nil
}
```

## Error Handling Architecture

### 2.1 Error Type System
```go
// Base error interface
type WtreeError interface {
    error
    Type() ErrorType
    Operation() string
    Context() map[string]interface{}
    Recoverable() bool
    SuggestedActions() []string
    UserMessage() string
}

type ErrorType int
const (
    ErrorTypeValidation ErrorType = iota
    ErrorTypeGit
    ErrorTypeFileSystem
    ErrorTypeNetwork
    ErrorTypeGitHub
    ErrorTypeEnvironment
    ErrorTypeUser
    ErrorTypeInternal
)

// Base error implementation
type BaseError struct {
    errType         ErrorType
    operation       string
    message         string
    cause           error
    context         map[string]interface{}
    recoverable     bool
    suggestedActions []string
}

func (be *BaseError) Error() string {
    if be.cause != nil {
        return fmt.Sprintf("%s: %s: %v", be.operation, be.message, be.cause)
    }
    return fmt.Sprintf("%s: %s", be.operation, be.message)
}

func (be *BaseError) Type() ErrorType { return be.errType }
func (be *BaseError) Operation() string { return be.operation }
func (be *BaseError) Context() map[string]interface{} { return be.context }
func (be *BaseError) Recoverable() bool { return be.recoverable }
func (be *BaseError) SuggestedActions() []string { return be.suggestedActions }

func (be *BaseError) UserMessage() string {
    return be.message
}
```

### 2.2 Specific Error Types
```go
// Validation errors
type ValidationError struct {
    *BaseError
}

func NewValidationError(message string, cause error) *ValidationError {
    return &ValidationError{
        BaseError: &BaseError{
            errType:     ErrorTypeValidation,
            operation:   "validation",
            message:     message,
            cause:       cause,
            recoverable: false,
            suggestedActions: []string{
                "Check your command arguments and try again",
                "Verify you're in a git repository",
                "Run 'wtree --help' for usage information",
            },
        },
    }
}

// Git operation errors
type GitError struct {
    *BaseError
    Repository string
    Command    string
}

func NewGitError(operation, message string, cause error) *GitError {
    return &GitError{
        BaseError: &BaseError{
            errType:     ErrorTypeGit,
            operation:   operation,
            message:     message,
            cause:       cause,
            recoverable: true,
            suggestedActions: []string{
                "Check git repository status",
                "Verify branch exists and is accessible",
                "Ensure working directory is clean",
            },
        },
    }
}

// GitHub-specific errors
type GitHubError struct {
    *BaseError
    PRNumber   int
    Repository string
}

func NewGitHubError(operation, message string, cause error) *GitHubError {
    return &GitHubError{
        BaseError: &BaseError{
            errType:     ErrorTypeGitHub,
            operation:   operation,
            message:     message,
            cause:       cause,
            recoverable: true,
            suggestedActions: []string{
                "Check GitHub CLI authentication: gh auth status",
                "Verify repository access: gh repo view",
                "Ensure PR exists and is accessible",
            },
        },
    }
}

// File system errors
type FileSystemError struct {
    *BaseError
    Path string
}

func NewFileSystemError(operation, path, message string, cause error) *FileSystemError {
    return &FileSystemError{
        BaseError: &BaseError{
            errType:     ErrorTypeFileSystem,
            operation:   operation,
            message:     message,
            cause:       cause,
            recoverable: true,
            context: map[string]interface{}{
                "path": path,
            },
            suggestedActions: []string{
                "Check file permissions",
                "Verify disk space availability",
                "Ensure parent directories exist",
            },
        },
        Path: path,
    }
}
```

### 2.3 Error Handling Pipeline
```go
type ErrorHandler struct {
    config    *Config
    ui        *UIManager
    logger    *Logger
    handlers  map[ErrorType]ErrorProcessor
}

type ErrorProcessor interface {
    Process(err WtreeError) error
    CanRecover(err WtreeError) bool
    Recover(err WtreeError) error
}

func NewErrorHandler(config *Config, ui *UIManager) *ErrorHandler {
    eh := &ErrorHandler{
        config: config,
        ui:     ui,
        logger: NewLogger(config.Verbose),
        handlers: make(map[ErrorType]ErrorProcessor),
    }
    
    // Register error processors
    eh.handlers[ErrorTypeGit] = &GitErrorProcessor{eh}
    eh.handlers[ErrorTypeGitHub] = &GitHubErrorProcessor{eh}
    eh.handlers[ErrorTypeFileSystem] = &FileSystemErrorProcessor{eh}
    
    return eh
}

func (eh *ErrorHandler) HandleError(err error) error {
    // Convert to WtreeError if possible
    var wtreeErr WtreeError
    if errors.As(err, &wtreeErr) {
        return eh.processWtreeError(wtreeErr)
    }
    
    // Wrap unknown errors
    wtreeErr = NewInternalError("unexpected error", err)
    return eh.processWtreeError(wtreeErr)
}

func (eh *ErrorHandler) processWtreeError(err WtreeError) error {
    // Log error details
    eh.logger.Error("Operation failed", map[string]interface{}{
        "type":      err.Type(),
        "operation": err.Operation(),
        "message":   err.Error(),
        "context":   err.Context(),
    })
    
    // Try to recover if possible
    if processor, exists := eh.handlers[err.Type()]; exists {
        if processor.CanRecover(err) {
            if recoveryErr := processor.Recover(err); recoveryErr == nil {
                eh.ui.Success("Recovered from error: %s", err.UserMessage())
                return nil
            }
        }
    }
    
    // Display user-friendly error
    eh.displayUserError(err)
    return err
}

func (eh *ErrorHandler) displayUserError(err WtreeError) {
    // Display main error message
    eh.ui.Error("✗ %s", err.UserMessage())
    
    // Show context if available
    if context := err.Context(); len(context) > 0 {
        eh.ui.InfoIndented("Context:")
        for key, value := range context {
            eh.ui.InfoIndented("  %s: %v", key, value)
        }
    }
    
    // Show suggested actions
    if actions := err.SuggestedActions(); len(actions) > 0 {
        eh.ui.InfoIndented("\nSuggested actions:")
        for _, action := range actions {
            eh.ui.InfoIndented("  • %s", action)
        }
    }
    
    // Show verbose details if enabled
    if eh.config.Verbose {
        eh.ui.InfoIndented("\nDetailed error:")
        eh.ui.InfoIndented("  %v", err)
    }
}
```

### 2.4 Recovery Strategies
```go
// Git error recovery
type GitErrorProcessor struct {
    handler *ErrorHandler
}

func (gep *GitErrorProcessor) CanRecover(err WtreeError) bool {
    gitErr, ok := err.(*GitError)
    if !ok {
        return false
    }
    
    // Can recover from some git errors
    switch {
    case strings.Contains(gitErr.Error(), "already exists"):
        return true
    case strings.Contains(gitErr.Error(), "not found"):
        return false
    case strings.Contains(gitErr.Error(), "permission denied"):
        return false
    default:
        return false
    }
}

func (gep *GitErrorProcessor) Recover(err WtreeError) error {
    gitErr := err.(*GitError)
    
    switch {
    case strings.Contains(gitErr.Error(), "already exists"):
        // Offer to remove and recreate
        return gep.handleAlreadyExists(gitErr)
    default:
        return gitErr
    }
}

func (gep *GitErrorProcessor) handleAlreadyExists(err *GitError) error {
    if err.operation == "worktree_create" {
        // Worktree already exists - offer to open it
        if gep.handler.ui.Confirm("Worktree already exists. Open existing worktree?") {
            // Open existing worktree
            return gep.openExistingWorktree(err)
        }
    }
    
    return err
}

// Environment error recovery
type EnvironmentErrorProcessor struct {
    handler *ErrorHandler
}

func (eep *EnvironmentErrorProcessor) CanRecover(err WtreeError) bool {
    envErr, ok := err.(*EnvironmentError)
    if !ok {
        return false
    }
    
    // Can recover from dependency installation failures
    return strings.Contains(envErr.Error(), "dependency installation")
}

func (eep *EnvironmentErrorProcessor) Recover(err WtreeError) error {
    // Continue without dependencies
    eep.handler.ui.Warning("Continuing without dependency installation")
    return nil
}
```

### 2.5 Operation Rollback System
```go
type RollbackManager struct {
    operations []RollbackOperation
    mu         sync.Mutex
}

type RollbackOperation struct {
    Description string
    Rollback    func() error
    Priority    int  // Higher priority rolled back first
}

func (rm *RollbackManager) Register(description string, rollback func() error, priority int) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    rm.operations = append(rm.operations, RollbackOperation{
        Description: description,
        Rollback:    rollback,
        Priority:    priority,
    })
}

func (rm *RollbackManager) Execute() error {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    // Sort by priority (highest first)
    sort.Slice(rm.operations, func(i, j int) bool {
        return rm.operations[i].Priority > rm.operations[j].Priority
    })
    
    var errs []error
    for _, op := range rm.operations {
        if err := op.Rollback(); err != nil {
            errs = append(errs, fmt.Errorf("rollback '%s' failed: %w", 
                op.Description, err))
        }
    }
    
    return errors.Join(errs...)
}

// Usage in operations
func executeWorktreeCreation(ctx *OperationContext) error {
    rollback := NewRollbackManager()
    defer func() {
        if r := recover(); r != nil {
            rollback.Execute()
            panic(r)
        }
    }()
    
    // Create branch
    if err := ctx.repo.CreateBranch(branchName, sourceBranch); err != nil {
        return err
    }
    rollback.Register("delete branch", func() error {
        return ctx.repo.DeleteBranch(branchName, true)
    }, 2)
    
    // Create worktree
    if err := ctx.repo.CreateWorktree(path, branchName); err != nil {
        rollback.Execute()
        return err
    }
    rollback.Register("remove worktree", func() error {
        return ctx.repo.RemoveWorktree(path, true)
    }, 1)
    
    // Success - don't rollback
    rollback = NewRollbackManager()
    return nil
}
```

This comprehensive configuration and error handling system provides robust, user-friendly error management with intelligent recovery capabilities and detailed logging for debugging.