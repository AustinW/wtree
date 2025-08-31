# Generic Project Architecture - Revised Phase 1

## Core Philosophy Change

**WTree is a generic git worktree manager** that works with ANY project type. It should not make assumptions about:
- Programming languages or frameworks
- Dependency management systems
- Environment setup requirements
- File structures or naming conventions

**Project-specific behavior is delegated** to the project itself via a `.wtreerc` configuration file.

## 1. Revised Core Responsibilities

### What WTree Does (Core Functionality)
```go
// WTree's core responsibilities
type CoreWorktreeManager struct {
    // Git operations
    CreateWorktree(path, branch string) error
    DeleteWorktree(path string) error  
    ListWorktrees() ([]*WorktreeInfo, error)
    
    // Path management
    CalculateWorktreePath(branch string) string
    ValidatePath(path string) error
    
    // Project hook execution
    ExecuteProjectHooks(event HookEvent, context HookContext) error
    
    // Basic file operations
    CopyFiles(patterns []string, from, to string) error
}
```

### What Projects Define (via .wtreerc)
```yaml
# .wtreerc - Project-specific configuration
hooks:
  # Called after worktree creation, before opening editor
  post_create:
    - copy .env .env.example
    - make setup-worktree
    - ./scripts/install-deps.sh
  
  # Called before worktree deletion
  pre_delete:
    - make cleanup-worktree
  
  # Called after successful merge
  post_merge:
    - ./scripts/cleanup-build-cache.sh

# Files to copy from main repo to worktree
copy_files:
  - .env
  - .env.local
  - config/local.yaml

# Files to link (symlink instead of copy)
link_files:
  - node_modules  # Share node_modules across worktrees
  - vendor        # Share composer vendor across worktrees

# Editor preferences (can override global config)
editor: cursor

# Custom worktree naming pattern
worktree_pattern: "{repo}-{branch}"  # Default pattern
```

## 2. Hook System Architecture

### 2.1 Hook Events
```go
type HookEvent string

const (
    HookPreCreate   HookEvent = "pre_create"   // Before worktree creation
    HookPostCreate  HookEvent = "post_create"  // After worktree creation
    HookPreDelete   HookEvent = "pre_delete"   // Before worktree deletion
    HookPostDelete  HookEvent = "post_delete"  // After worktree deletion
    HookPreMerge    HookEvent = "pre_merge"    // Before merge operation
    HookPostMerge   HookEvent = "post_merge"   // After merge operation
)

type HookContext struct {
    Event         HookEvent
    WorktreePath  string
    RepoPath      string
    Branch        string
    TargetBranch  string  // For merge operations
    Environment   map[string]string
}
```

### 2.2 Hook Execution Engine
```go
type HookExecutor struct {
    config     *ProjectConfig
    ui         *UIManager
    timeout    time.Duration
}

func (he *HookExecutor) ExecuteHooks(event HookEvent, ctx HookContext) error {
    hooks := he.config.Hooks[event]
    if len(hooks) == 0 {
        return nil // No hooks defined
    }
    
    he.ui.Info("Running %s hooks...", event)
    
    for i, hookCmd := range hooks {
        if err := he.executeHook(hookCmd, ctx, i+1, len(hooks)); err != nil {
            return fmt.Errorf("hook failed: %s: %w", hookCmd, err)
        }
    }
    
    return nil
}

func (he *HookExecutor) executeHook(cmd string, ctx HookContext, current, total int) error {
    he.ui.Progress("Running hook %d/%d: %s", current, total, cmd)
    
    // Parse command with environment variable substitution
    expandedCmd := he.expandCommand(cmd, ctx)
    
    // Execute in worktree directory
    execCtx, cancel := context.WithTimeout(context.Background(), he.timeout)
    defer cancel()
    
    command := exec.CommandContext(execCtx, "sh", "-c", expandedCmd)
    command.Dir = ctx.WorktreePath
    command.Env = he.buildEnvironment(ctx)
    
    // Capture output for logging
    output, err := command.CombinedOutput()
    if err != nil {
        he.ui.Error("Hook failed: %s", string(output))
        return err
    }
    
    if he.config.Verbose {
        he.ui.Success("Hook output: %s", string(output))
    }
    
    return nil
}

func (he *HookExecutor) expandCommand(cmd string, ctx HookContext) string {
    replacements := map[string]string{
        "{worktree_path}": ctx.WorktreePath,
        "{repo_path}":     ctx.RepoPath,
        "{branch}":        ctx.Branch,
        "{target_branch}": ctx.TargetBranch,
    }
    
    expanded := cmd
    for placeholder, value := range replacements {
        expanded = strings.ReplaceAll(expanded, placeholder, value)
    }
    
    return expanded
}
```

### 2.3 Project Configuration Loading
```go
type ProjectConfig struct {
    // Hook definitions
    Hooks map[HookEvent][]string `yaml:"hooks"`
    
    // File operations
    CopyFiles []string `yaml:"copy_files"`
    LinkFiles []string `yaml:"link_files"`
    
    // Naming and paths
    WorktreePattern string `yaml:"worktree_pattern"`
    
    // Editor override
    Editor string `yaml:"editor"`
    
    // Advanced settings
    Timeout       time.Duration `yaml:"timeout"`
    AllowFailure  bool         `yaml:"allow_failure"`
    Verbose       bool         `yaml:"verbose"`
}

func LoadProjectConfig(repoPath string) (*ProjectConfig, error) {
    configPath := filepath.Join(repoPath, ".wtreerc")
    
    // Return default config if no .wtreerc exists
    if !fileExists(configPath) {
        return &ProjectConfig{
            Hooks:           make(map[HookEvent][]string),
            WorktreePattern: "{repo}-{branch}",
            Timeout:         5 * time.Minute,
            AllowFailure:    false,
        }, nil
    }
    
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read .wtreerc: %w", err)
    }
    
    var config ProjectConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse .wtreerc: %w", err)
    }
    
    // Apply defaults
    if config.WorktreePattern == "" {
        config.WorktreePattern = "{repo}-{branch}"
    }
    if config.Timeout == 0 {
        config.Timeout = 5 * time.Minute
    }
    
    return &config, nil
}
```

## 3. Revised Operation Flow

### 3.1 Generic Worktree Creation
```go
func (wm *WorktreeManager) CreateWorktree(branch string) error {
    // 1. Standard validation (git repo, permissions, etc.)
    if err := wm.validateCreation(branch); err != nil {
        return err
    }
    
    // 2. Load project configuration
    projectConfig, err := LoadProjectConfig(wm.repoPath)
    if err != nil {
        return fmt.Errorf("failed to load project config: %w", err)
    }
    
    // 3. Calculate worktree path using project pattern
    worktreePath := wm.calculatePath(branch, projectConfig.WorktreePattern)
    
    // 4. Execute pre-create hooks
    ctx := HookContext{
        Event:        HookPreCreate,
        WorktreePath: worktreePath,
        RepoPath:     wm.repoPath,
        Branch:       branch,
    }
    
    if err := wm.hookExecutor.ExecuteHooks(HookPreCreate, ctx); err != nil {
        return fmt.Errorf("pre-create hooks failed: %w", err)
    }
    
    // 5. Create git worktree (core wtree functionality)
    if err := wm.git.CreateWorktree(worktreePath, branch); err != nil {
        return fmt.Errorf("git worktree creation failed: %w", err)
    }
    
    // 6. Perform file operations defined in project config
    if err := wm.performFileOperations(projectConfig, ctx); err != nil {
        // Non-fatal - log warning
        wm.ui.Warning("File operations failed: %v", err)
    }
    
    // 7. Execute post-create hooks
    ctx.Event = HookPostCreate
    if err := wm.hookExecutor.ExecuteHooks(HookPostCreate, ctx); err != nil {
        wm.ui.Warning("Post-create hooks failed: %v", err)
        // Continue - worktree is created, hooks are optional
    }
    
    // 8. Open in editor (respects project config override)
    editor := projectConfig.Editor
    if editor == "" {
        editor = wm.globalConfig.Editor
    }
    
    if err := wm.openInEditor(worktreePath, editor); err != nil {
        wm.ui.Warning("Failed to open editor: %v", err)
    }
    
    wm.ui.Success("Worktree created: %s", worktreePath)
    return nil
}
```

### 3.2 File Operations (Generic)
```go
func (wm *WorktreeManager) performFileOperations(config *ProjectConfig, ctx HookContext) error {
    var errs []error
    
    // Copy files
    for _, pattern := range config.CopyFiles {
        if err := wm.copyFiles(pattern, ctx.RepoPath, ctx.WorktreePath); err != nil {
            errs = append(errs, fmt.Errorf("copy %s: %w", pattern, err))
        }
    }
    
    // Create symlinks
    for _, pattern := range config.LinkFiles {
        if err := wm.linkFiles(pattern, ctx.RepoPath, ctx.WorktreePath); err != nil {
            errs = append(errs, fmt.Errorf("link %s: %w", pattern, err))
        }
    }
    
    return errors.Join(errs...)
}

func (wm *WorktreeManager) copyFiles(pattern, from, to string) error {
    // Use filepath.Glob to find matching files
    matches, err := filepath.Glob(filepath.Join(from, pattern))
    if err != nil {
        return err
    }
    
    for _, srcPath := range matches {
        // Calculate relative path and destination
        relPath, err := filepath.Rel(from, srcPath)
        if err != nil {
            continue
        }
        
        dstPath := filepath.Join(to, relPath)
        
        // Skip if file doesn't exist in source
        if !fileExists(srcPath) {
            continue
        }
        
        // Create destination directory if needed
        if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
            return err
        }
        
        // Copy file
        if err := copyFile(srcPath, dstPath); err != nil {
            return err
        }
        
        wm.ui.Info("Copied: %s", relPath)
    }
    
    return nil
}
```

## 4. Example Project Configurations

### 4.1 Laravel PHP Project
```yaml
# .wtreerc for Laravel project
hooks:
  post_create:
    - cp .env.example .env
    - composer install --no-dev --optimize-autoloader
    - php artisan key:generate
    - php artisan migrate
    - npm install
    - npm run build

copy_files:
  - .env
  - storage/oauth-*.key

link_files:
  - node_modules
  - vendor

editor: cursor
```

### 4.2 Node.js/React Project
```yaml
# .wtreerc for Node.js project
hooks:
  post_create:
    - npm ci
    - npm run build:dev
    
  pre_delete:
    - npm run clean

copy_files:
  - .env.local
  - .env.development

link_files:
  - node_modules

worktree_pattern: "{repo}-{branch}"
timeout: 10m
```

### 4.3 Go Project
```yaml
# .wtreerc for Go project
hooks:
  post_create:
    - go mod download
    - go mod tidy
    - make setup
    
  post_merge:
    - make clean

copy_files:
  - config.local.yaml
  - .env

editor: code
```

### 4.4 Python Project
```yaml
# .wtreerc for Python project
hooks:
  post_create:
    - python -m venv venv
    - source venv/bin/activate && pip install -r requirements.txt
    - source venv/bin/activate && python manage.py migrate

copy_files:
  - .env
  - local_settings.py

worktree_pattern: "{repo}-branch-{branch}"
```

## 5. Benefits of This Approach

### ✅ **True Genericity**
- Works with any programming language/framework
- No hardcoded assumptions about project structure
- Each project defines its own setup requirements

### ✅ **Flexible Configuration**
- Projects can define complex setup workflows
- Supports both simple file copying and complex build processes
- Hooks can run any shell command or script

### ✅ **Maintainable Codebase**
- WTree focuses only on git worktree management
- Project-specific logic stays in the project
- Clear separation of concerns

### ✅ **Extensible**
- Easy to add new hook events
- Projects can evolve their setup processes independently
- No need to update WTree for new project types

This revised architecture makes WTree truly generic while maintaining all the power and flexibility needed for complex project setups.