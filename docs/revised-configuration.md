# Revised Configuration System - Generic Architecture

## Configuration Philosophy

The configuration system now separates **tool configuration** (how WTree works) from **project configuration** (what projects need). This maintains clear boundaries and keeps WTree generic.

## 1. WTree Tool Configuration

### 1.1 Global Configuration (`~/.config/wtree/config.yaml`)
```yaml
# Global WTree tool settings
version: "1.0"

# Editor preferences (can be overridden by project .wtreerc)
editor: "cursor"

# UI settings  
ui:
  colors: true
  progress_bars: true
  verbose: false
  confirm_destructive: true

# GitHub integration
github:
  cli_command: "gh"
  cache_timeout: "5m"

# Hook execution
hooks:
  timeout: "5m"
  allow_failure: false
  max_parallel: 3

# Path settings
paths:
  # Leave empty for auto-detection (parent of git repo)
  worktree_parent: ""

# Performance
performance:
  max_concurrent_operations: 3
  operation_timeout: "10m"
```

### 1.2 No More Hardcoded Project Assumptions
**Removed from WTree configuration**:
- ❌ `installers` (composer, npm, bun, etc.)
- ❌ `copy_env`, `generate_ssl` 
- ❌ `install_dependencies`
- ❌ `ssl_subject`

**Why**: These are project-specific concerns that belong in `.wtreerc`

### 1.3 Simplified Global Config Structure
```go
type WTreeConfig struct {
    // Editor preferences
    Editor string `yaml:"editor"`
    
    // UI settings
    UI UIConfig `yaml:"ui"`
    
    // GitHub settings
    GitHub GitHubConfig `yaml:"github"`
    
    // Hook execution settings
    Hooks HookConfig `yaml:"hooks"`
    
    // Path settings
    Paths PathConfig `yaml:"paths"`
    
    // Performance settings
    Performance PerformanceConfig `yaml:"performance"`
}

type UIConfig struct {
    Colors            bool `yaml:"colors"`
    ProgressBars      bool `yaml:"progress_bars"`
    Verbose           bool `yaml:"verbose"`
    ConfirmDestructive bool `yaml:"confirm_destructive"`
}

type GitHubConfig struct {
    CLICommand   string        `yaml:"cli_command"`
    CacheTimeout time.Duration `yaml:"cache_timeout"`
}

type HookConfig struct {
    Timeout     time.Duration `yaml:"timeout"`
    AllowFailure bool         `yaml:"allow_failure"`
    MaxParallel  int          `yaml:"max_parallel"`
}

type PathConfig struct {
    WorktreeParent string `yaml:"worktree_parent"`
}

type PerformanceConfig struct {
    MaxConcurrentOps  int           `yaml:"max_concurrent_operations"`
    OperationTimeout  time.Duration `yaml:"operation_timeout"`
}
```

## 2. Project Configuration (`.wtreerc`)

Projects define their own setup behavior via `.wtreerc` files. See `wtreerc-specification.md` for complete details.

### 2.1 Project Config Structure
```go
type ProjectConfig struct {
    Version string `yaml:"version"`
    
    // Hook definitions (project-specific commands)
    Hooks map[HookEvent][]string `yaml:"hooks"`
    
    // File operations
    CopyFiles   []string `yaml:"copy_files"`
    LinkFiles   []string `yaml:"link_files"`
    IgnoreFiles []string `yaml:"ignore_files"`
    
    // Naming and behavior overrides
    WorktreePattern string `yaml:"worktree_pattern"`
    Editor          string `yaml:"editor"`
    
    // Execution settings (overrides global)
    Timeout      time.Duration `yaml:"timeout"`
    AllowFailure bool          `yaml:"allow_failure"`
    Verbose      bool          `yaml:"verbose"`
}
```

## 3. Configuration Loading Strategy

### 3.1 Two-Phase Configuration Loading
```go
type ConfigManager struct {
    globalConfig  *WTreeConfig
    projectConfig *ProjectConfig
}

func (cm *ConfigManager) LoadConfigurations(repoPath string) error {
    // Phase 1: Load WTree tool configuration
    globalConfig, err := cm.loadGlobalConfig()
    if err != nil {
        return fmt.Errorf("failed to load global config: %w", err)
    }
    cm.globalConfig = globalConfig
    
    // Phase 2: Load project configuration
    projectConfig, err := cm.loadProjectConfig(repoPath)
    if err != nil {
        return fmt.Errorf("failed to load project config: %w", err)
    }
    cm.projectConfig = projectConfig
    
    return nil
}

func (cm *ConfigManager) loadGlobalConfig() (*WTreeConfig, error) {
    // Configuration hierarchy for WTree settings:
    // 1. Command line flags (highest priority)
    // 2. Environment variables (WTREE_*)
    // 3. User config file (~/.config/wtree/config.yaml)
    // 4. Built-in defaults (lowest priority)
    
    config := getDefaultWTreeConfig()
    
    // Load user config if exists
    userConfigPath := getUserConfigPath()
    if fileExists(userConfigPath) {
        if err := mergeConfigFromFile(config, userConfigPath); err != nil {
            return nil, err
        }
    }
    
    // Apply environment variables
    if err := applyEnvironmentVariables(config); err != nil {
        return nil, err
    }
    
    // Apply command line flags (handled by cobra)
    // This happens at command execution time
    
    return config, nil
}

func (cm *ConfigManager) loadProjectConfig(repoPath string) (*ProjectConfig, error) {
    // Project configuration only comes from .wtreerc
    // No inheritance or overrides - projects define their own behavior
    
    configPath := filepath.Join(repoPath, ".wtreerc")
    if !fileExists(configPath) {
        // Return minimal default config
        return &ProjectConfig{
            Version:         "1.0",
            Hooks:           make(map[HookEvent][]string),
            WorktreePattern: "{repo}-{branch}",
        }, nil
    }
    
    return parseProjectConfig(configPath)
}
```

### 3.2 Configuration Merging Rules

**Global Configuration**: Follows standard precedence (flags > env > file > defaults)

**Project Configuration**: No merging - each project defines its complete behavior

**Editor Resolution**: 
1. Command line flag `--editor`
2. Project `.wtreerc` `editor` setting  
3. Global config `editor` setting
4. Default (`cursor`)

## 4. Environment Variable Support

### 4.1 WTree Tool Settings
```bash
# UI settings
export WTREE_EDITOR="code"
export WTREE_COLORS="false"
export WTREE_VERBOSE="true"

# GitHub settings  
export WTREE_GITHUB_CLI="gh"
export WTREE_GITHUB_CACHE_TIMEOUT="10m"

# Hook settings
export WTREE_HOOK_TIMEOUT="15m"
export WTREE_HOOK_ALLOW_FAILURE="true"

# Path settings
export WTREE_WORKTREE_PARENT="/custom/worktrees"
```

### 4.2 Project Hook Environment
WTree provides environment variables to project hooks:
```bash
# Provided by WTree to project hooks
WTREE_EVENT="post_create"
WTREE_BRANCH="feature/login"  
WTREE_REPO_PATH="/path/to/repo"
WTREE_WORKTREE_PATH="/path/to/repo-feature-login"
WTREE_TARGET_BRANCH="main"  # For merge operations
```

## 5. Configuration Validation

### 5.1 Global Configuration Validation
```go
func (config *WTreeConfig) Validate() error {
    validators := []func(*WTreeConfig) error{
        validateEditor,
        validateTimeouts,
        validatePaths,
        validateGitHubSettings,
    }
    
    for _, validator := range validators {
        if err := validator(config); err != nil {
            return err
        }
    }
    
    return nil
}

func validateEditor(config *WTreeConfig) error {
    if config.Editor == "" {
        return nil // Empty is valid (will use default)
    }
    
    // Don't validate editor exists - project .wtreerc might override
    return nil
}

func validateTimeouts(config *WTreeConfig) error {
    if config.Hooks.Timeout <= 0 {
        return NewValidationError("hook timeout must be positive")
    }
    
    if config.Performance.OperationTimeout <= 0 {
        return NewValidationError("operation timeout must be positive")
    }
    
    return nil
}
```

### 5.2 Project Configuration Validation
```go
func (config *ProjectConfig) Validate(repoPath string) error {
    // Validate version compatibility
    if config.Version != "1.0" {
        return NewValidationError(fmt.Sprintf("unsupported .wtreerc version: %s", config.Version))
    }
    
    // Validate hook commands are not empty
    for event, hooks := range config.Hooks {
        for _, hook := range hooks {
            if strings.TrimSpace(hook) == "" {
                return NewValidationError(fmt.Sprintf("empty hook command in %s", event))
            }
        }
    }
    
    // Validate file patterns
    for _, pattern := range config.CopyFiles {
        if strings.Contains(pattern, "..") {
            return NewValidationError("file patterns cannot contain '..' for security")
        }
    }
    
    return nil
}
```

## 6. Configuration Examples

### 6.1 Minimal Global Config
```yaml
# ~/.config/wtree/config.yaml
editor: "cursor"
ui:
  colors: true
```

### 6.2 Advanced Global Config  
```yaml
# ~/.config/wtree/config.yaml
version: "1.0"

editor: "code"

ui:
  colors: true
  progress_bars: true
  verbose: false
  confirm_destructive: true

github:
  cli_command: "gh"
  cache_timeout: "10m"

hooks:
  timeout: "15m"
  allow_failure: false
  max_parallel: 2

paths:
  worktree_parent: "/Users/me/Dev/worktrees"

performance:
  max_concurrent_operations: 2
  operation_timeout: "20m"
```

### 6.3 Project-Specific Overrides
```yaml
# .wtreerc - Can override some global settings
editor: "vim"  # Override global editor for this project
timeout: "20m" # This project needs longer setup time
verbose: true  # Show detailed output for this project

hooks:
  post_create:
    - echo "Project-specific setup starting..."
    - make setup-dev-environment
```

## 7. Migration from Old Architecture

### 7.1 Automatic Migration Helper
```go
func MigrateOldConfiguration() error {
    // Check if user has old configuration with hardcoded installers
    oldConfigPath := getUserConfigPath()
    if !fileExists(oldConfigPath) {
        return nil
    }
    
    // Parse old config
    oldConfig, err := parseOldConfiguration(oldConfigPath)
    if err != nil {
        return err
    }
    
    // If old config has project-specific settings, warn user
    if hasProjectSpecificSettings(oldConfig) {
        fmt.Println("⚠️  Configuration migration needed!")
        fmt.Println("Your WTree config contains project-specific settings.")
        fmt.Println("These should be moved to .wtreerc files in your projects.")
        fmt.Println("See: wtree --help-migration")
        
        // Offer to create example .wtreerc
        return offerExampleWtreerc(oldConfig)
    }
    
    return nil
}
```

This revised configuration system cleanly separates tool behavior from project behavior, making WTree truly generic while maintaining all the flexibility needed for complex project setups.