# Phase 1: Detailed Implementation Plan

## Project Overview
Build a robust CLI tool in Go that replaces existing shell functions with enhanced safety, validation, and user experience.

## 1. Project Structure & Setup

### Go Module Structure
```
wtree/
├── cmd/
│   ├── root.go                 # Root cobra command setup
│   ├── create.go               # wtree [branch] / wtree create
│   ├── delete.go               # wtree delete [branch]
│   ├── merge.go                # wtree merge [branch]
│   ├── list.go                 # wtree list
│   └── pr/
│       ├── create.go           # wtree pr [number]
│       └── clean.go            # wtree pr clean
├── internal/
│   ├── config/
│   │   ├── config.go           # Configuration management
│   │   └── defaults.go         # Default settings
│   ├── git/
│   │   ├── repository.go       # Git repo operations
│   │   ├── worktree.go         # Worktree management
│   │   └── validation.go       # Git state validation
│   ├── github/
│   │   ├── client.go           # GitHub API client
│   │   ├── pr.go               # PR operations
│   │   └── auth.go             # Authentication handling
│   ├── ui/
│   │   ├── output.go           # Formatted output
│   │   ├── progress.go         # Progress indicators
│   │   └── prompts.go          # User prompts
│   └── worktree/
│       ├── manager.go          # Core worktree logic
│       ├── environment.go      # Environment setup (.env, certs)
│       ├── dependencies.go     # Dependency installation
│       └── cleanup.go          # Cleanup operations
├── pkg/
│   └── types/
│       ├── worktree.go         # Worktree types
│       ├── config.go           # Config types
│       └── errors.go           # Error types
├── docs/
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yml
└── README.md
```

### Key Dependencies
```go
// CLI Framework
github.com/spf13/cobra         // Command structure
github.com/spf13/viper         // Configuration management

// Git Operations  
github.com/go-git/go-git/v5     // Pure Go git implementation
github.com/go-git/go-billy/v5   // Filesystem abstraction

// GitHub Integration
github.com/cli/go-gh            // GitHub CLI integration
github.com/google/go-github/v57 // GitHub API client

// UI/UX
github.com/fatih/color          // Colored output
github.com/schollz/progressbar/v3 // Progress bars
github.com/AlecAivazis/survey/v2  // Interactive prompts

// Utilities
github.com/pkg/errors           // Error wrapping
gopkg.in/yaml.v3               // YAML config parsing
```

## 2. CLI Command Design

### Command Hierarchy
```
wtree
├── [branch-name]               # Create worktree (default action)
├── create <branch-name>        # Explicit create (alias for above)
├── delete <branch-name>...     # Delete worktree(s)
├── merge <branch-name>         # Merge and cleanup all
├── list                        # List all worktrees
└── pr
    ├── <number>                # Create PR worktree
    ├── clean [number]...       # Clean PR worktree(s)
    └── list                    # List PR worktrees
```

### Global Flags
```bash
--dry-run          # Show what would happen
--force            # Skip confirmations
--verbose, -v      # Detailed output
--config           # Config file path
--no-install       # Skip dependency installation
--editor           # Override default editor
```

### Command-Specific Flags
```bash
# wtree [branch] / wtree create
--install, -i      # Force install dependencies
--no-env          # Skip .env copying
--no-ssl          # Skip SSL cert generation

# wtree delete
--keep-branch     # Keep branch, remove worktree only
--all             # Delete all worktrees

# wtree merge  
--no-commit       # Skip auto-commit
--target          # Target branch (default: current)

# wtree pr clean
--all             # Clean all PR worktrees
--merged          # Clean only merged PRs
```

## 3. Core Implementation Details

### 3.1 Repository Detection & Validation
```go
type Repository struct {
    Path     string    // Absolute path to repo root
    Name     string    // Repository name
    Parent   string    // Parent directory for worktrees
    Remote   string    // Primary remote URL
    Branch   string    // Current branch
}

// Validation checks:
- Must be inside git repository
- Repository must have at least one commit
- Working directory must be clean (configurable)
- Remote 'origin' should exist for PR operations
```

### 3.2 Worktree Management
```go
type Worktree struct {
    Name       string    // Branch name
    Path       string    // Absolute worktree path
    Branch     string    // Associated branch
    Status     string    // clean, dirty, etc.
    IsPR       bool      // Is this a PR worktree
    PRNumber   int       // PR number if applicable
    Created    time.Time // Creation timestamp
}

// Core operations:
1. Path calculation: parentDir + "/" + repoName + "-" + branchName
2. Branch creation if not exists
3. Worktree creation with git worktree add
4. Environment setup (parallel where possible)
5. Editor opening
```

### 3.3 Environment Setup Pipeline
```go
// Sequential operations:
1. Create worktree directory
2. Copy .env file (if exists)
3. Generate SSL certificates (background)
4. Install dependencies (parallel: composer & bun)
5. Create metadata file (.wtree-info)

// .wtree-info format (YAML):
created: 2024-01-15T10:30:00Z
branch: feature/login
repo: myapp
type: branch|pr
pr_number: 123  # if type == pr
pr_branch: feature/user-auth
pr_title: "Add user authentication"
dependencies_installed: true
```

### 3.4 GitHub PR Integration
```go
type PRInfo struct {
    Number      int       `json:"number"`
    Title       string    `json:"title"`
    Branch      string    `json:"head_ref"`
    BaseBranch  string    `json:"base_ref"`
    Author      string    `json:"author_login"`
    State       string    `json:"state"`
    URL         string    `json:"html_url"`
    Mergeable   bool      `json:"mergeable"`
}

// Integration steps:
1. Validate gh CLI availability and auth
2. Fetch PR info via gh CLI (not API directly)
3. Create local tracking branch: pr-{number}-{branch}
4. Create worktree with PR-specific naming
5. Store enhanced metadata for cleanup
```

## 4. Error Handling Strategy

### Error Categories & Recovery
```go
type WtreeError struct {
    Type      ErrorType
    Operation string
    Path      string
    Cause     error
    Rollback  func() error  // Optional rollback function
}

// Error types:
- ValidationError   # Pre-flight checks failed
- GitError         # Git operations failed  
- FileSystemError  # File/directory operations failed
- NetworkError     # GitHub/remote operations failed
- UserError        # User input/workflow errors
```

### Rollback Strategy
```go
// Operations that need rollback:
1. Worktree creation failure -> cleanup partial directory
2. Branch creation failure -> delete created worktree
3. Environment setup failure -> offer to keep partial setup
4. Dependency install failure -> continue but warn user
```

## 5. Configuration System

### Configuration Hierarchy
1. Global config: `~/.config/wtree/config.yaml`
2. Repository config: `<repo>/.wtree.yaml`
3. Environment variables: `WTREE_*`
4. Command flags (highest priority)

### Configuration Schema
```yaml
# Global settings
editor: "cursor"
install_dependencies: true
parallel_installs: true
confirm_destructive: true

# Path settings
worktree_parent: ""  # empty = auto-detect from repo

# Environment setup
copy_env: true
generate_ssl: true
ssl_subject: "/C=US/ST=CA/L=City/O=Org/CN=localhost"

# GitHub settings
github_cli: "gh"
pr_branch_format: "pr-{number}-{branch}"

# Dependencies
installers:
  php: ["composer", "install"]
  node: ["bun", "install"] 
  # Could add: python: ["pip", "install", "-r", "requirements.txt"]

# Output settings
colors: true
progress_bars: true
verbose: false
```

## 6. Implementation Phases

### Phase 1A: Foundation (Week 1)
- [ ] Project setup with Go modules
- [ ] Cobra CLI framework with basic commands
- [ ] Git repository detection and validation
- [ ] Basic worktree creation (no environment setup)
- [ ] Simple error handling and logging

### Phase 1B: Core Operations (Week 2)
- [ ] Complete worktree lifecycle (create, delete, merge)
- [ ] Environment setup pipeline (.env, SSL, dependencies)
- [ ] Configuration system implementation
- [ ] Enhanced error handling with rollback

### Phase 1C: GitHub Integration (Week 3)
- [ ] GitHub CLI integration and validation
- [ ] PR worktree creation with metadata
- [ ] PR cleanup operations
- [ ] Legacy worktree detection and migration

### Phase 1D: Polish & Testing (Week 4)
- [ ] Comprehensive error messages
- [ ] User experience improvements (colors, progress)
- [ ] Edge case handling and validation
- [ ] Documentation and testing

## 7. Testing Strategy

### Unit Tests
- Git operations with test repositories
- Configuration parsing and validation
- Error handling and rollback scenarios
- Path calculation and naming logic

### Integration Tests  
- End-to-end worktree lifecycle
- GitHub CLI integration (with mocks)
- Dependency installation scenarios
- Multi-platform compatibility

### Manual Test Scenarios
- Various repository structures and states
- Different dependency combinations
- GitHub authentication states
- Error recovery workflows

## Success Metrics
- [ ] All existing shell function capabilities replicated
- [ ] Improved error handling and recovery
- [ ] Zero data loss during operations
- [ ] Sub-second response time for common operations
- [ ] Clear, actionable error messages
- [ ] Comprehensive help and documentation