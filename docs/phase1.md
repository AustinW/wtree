# Phase 1: Core Functionality

## Goal
Establish the foundation CLI tool with feature parity to existing shell functions, plus essential robustness improvements.

## High-Level Topics

### 1. CLI Framework Setup
- Initialize Go project with Cobra CLI framework
- Set up basic command structure (`wtree`, `wtree pr`, `wtree merge`, `wtree delete`)
- Implement standardized help system and flag parsing
- Basic configuration file support (.wtreerc)

### 2. Core Worktree Operations
- **Create worktrees** (`wtree create <branch>` or `wtree <branch>`)
  - Branch creation from current branch if doesn't exist
  - Worktree creation at `../<repo>-<branch>` (parent dir of current git repo)
  - Environment setup (.env copy, SSL cert generation)
  - Optional dependency installation
- **Delete worktrees** (`wtree delete <branch>`)
  - Safe worktree removal with current directory checking
  - Branch cleanup with safety for 'main'
  - Force deletion options
- **Merge and cleanup** (`wtree merge <branch>`)
  - Auto-commit uncommitted changes
  - Merge to target branch
  - Complete worktree cleanup

### 3. GitHub PR Integration
- **PR worktree creation** (`wtree pr <number>`)
  - GitHub CLI integration and validation
  - PR metadata fetching and storage
  - Specialized PR worktree naming at `../<repo>-pr-<number>`
- **PR cleanup** (`wtree pr clean`)
  - List all PR worktrees
  - Selective and bulk cleanup options
  - Legacy worktree detection and cleanup

### 4. Essential Robustness
- **Input validation**
  - Git repository detection
  - Required tool availability (git, gh, jq, etc.)
  - Branch name sanitization
- **Error handling**
  - Graceful failure with helpful messages
  - Operation rollback on failure where possible
- **Safety checks**
  - Prevent deletion of current worktree
  - Confirm destructive operations
  - Protect main/master branches

### 5. Basic User Experience
- Consistent command interface and help system
- Clear error messages with suggested fixes
- Progress indication for long operations
- Colorized output for better readability

## Success Criteria
- Complete functional replacement of existing shell functions
- No breaking changes during development (single-user project)
- Improved error handling and user safety
- Foundation ready for Phase 2 enhancements