# CLI Interface Design - Phase 1

## Command Structure Philosophy

### 1. Primary Commands (90% of usage)
```bash
wtree feature/login              # Create worktree (most common)
wtree pr 1945                   # Create PR worktree (most common)
wtree delete feature/login      # Delete worktree
wtree merge feature/login       # Merge and cleanup
```

### 2. Discovery & Management Commands
```bash
wtree list                      # Show all worktrees
wtree pr list                   # Show PR worktrees
wtree pr clean 1945            # Clean specific PR
wtree pr clean --all           # Clean all PRs
```

## Detailed Command Specifications

### `wtree [branch-name]` - Create Worktree
**Purpose**: Create a new worktree for development
**Aliases**: `wtree create <branch>`

```bash
# Usage examples
wtree feature/login             # Create from current branch
wtree hotfix/bug-123           # Create from current branch
wtree -i develop               # Create with dependency install
wtree --no-env staging         # Create without .env copy

# With explicit flags
wtree feature/login \
  --install \
  --editor=code \
  --no-ssl
```

**Behavior**:
1. Validate git repository and current state
2. Sanitize branch name for filesystem safety
3. Check if worktree already exists
4. Create branch from current if doesn't exist
5. Create worktree at `../<repo>-<branch>`
6. Setup environment (.env, SSL, deps)
7. Open in configured editor

**Error Cases**:
- Not in git repository → Clear error with suggestion
- Worktree already exists → Show path, offer to open
- Branch name invalid → Suggest valid alternative
- No write permissions → Show permission error
- Dependency tools missing → Install anyway, show warning

### `wtree delete <branch>...` - Delete Worktrees
**Purpose**: Clean up development worktrees

```bash
# Usage examples  
wtree delete feature/login              # Delete single worktree
wtree delete feature/login hotfix/bug   # Delete multiple
wtree delete --all                      # Delete all worktrees
wtree delete --keep-branch feature/old  # Keep branch, remove worktree
wtree delete -f feature/experimental    # Force delete unmerged
```

**Behavior**:
1. Validate each branch/worktree exists
2. Safety check: not currently inside worktree
3. Show what will be deleted (worktree + branch)
4. Confirm destructive operations (unless --force)
5. Remove worktree and cleanup directories
6. Delete local branch (unless --keep-branch or main)

**Interactive Confirmations**:
```
Delete worktree for 'feature/login'?
  Path: /Users/awhite/Projects/myapp-feature-login
  Branch: feature/login (unmerged, 3 commits ahead)
  [y]es / [n]o / [k]eep branch / [s]kip: 
```

### `wtree merge <branch>` - Merge and Cleanup
**Purpose**: Merge feature branch and cleanup ALL worktrees

```bash
# Usage examples
wtree merge feature/login          # Merge to current branch
wtree merge feature/login main     # Merge to specific branch
wtree merge --no-commit feature/x  # Don't auto-commit dirty changes
wtree merge --keep-worktrees feat  # Merge but don't cleanup all
```

**Behavior**:
1. Find worktree for specified branch
2. Auto-commit any uncommitted changes in worktree
3. Switch to target branch in main repo
4. Merge the feature branch
5. **Cleanup ALL worktrees** (destructive!)
6. Delete all temporary branches

**Warning Display**:
```
⚠️  DESTRUCTIVE OPERATION ⚠️

This will:
  • Auto-commit changes in 'feature/login' worktree
  • Merge 'feature/login' → 'main'
  • DELETE ALL worktrees:
    - /Users/awhite/Projects/myapp-feature-login
    - /Users/awhite/Projects/myapp-hotfix-bug-123
    - /Users/awhite/Projects/myapp-pr-1945
  • Delete branches: feature/login, hotfix/bug-123, pr-1945-user-auth

Continue? [y/N]: 
```

### `wtree list` - Show Worktrees
**Purpose**: Display all active worktrees with status

```bash
wtree list                  # Show all worktrees
wtree list --status        # Include git status
wtree list --branches      # Include branch relationships
```

**Output Format**:
```
Active Worktrees:
┌─────────────────────────┬─────────────────────────────────────────────────┬─────────┬─────────┐
│ BRANCH                  │ PATH                                            │ STATUS  │ TYPE    │
├─────────────────────────┼─────────────────────────────────────────────────┼─────────┼─────────┤
│ feature/login          │ /Users/awhite/Projects/myapp-feature-login      │ clean   │ branch  │
│ hotfix/bug-123         │ /Users/awhite/Projects/myapp-hotfix-bug-123     │ dirty   │ branch  │
│ pr-1945-user-auth      │ /Users/awhite/Projects/myapp-pr-1945            │ clean   │ pr #1945│
└─────────────────────────┴─────────────────────────────────────────────────┴─────────┴─────────┘

3 worktrees active
```

### `wtree pr <number>` - Create PR Worktree
**Purpose**: Create worktree for GitHub PR review

```bash
# Usage examples
wtree pr 1945                  # Create PR worktree
wtree pr 1945 --no-install    # Skip dependency installation
wtree pr 1945 --editor=code   # Open in specific editor
```

**Behavior**:
1. Validate GitHub CLI is installed and authenticated
2. Fetch PR information from GitHub
3. Display PR details (title, author, branch)
4. Create tracking branch: `pr-<number>-<branch>`
5. Create worktree at `../<repo>-pr-<number>`
6. Setup environment with PR metadata
7. Open in editor

**PR Information Display**:
```
Fetching PR #1945...

PR #1945: Add user authentication system
Author: john-doe
Branch: feature/user-auth → main
State: open

Creating worktree at: /Users/awhite/Projects/myapp-pr-1945
```

### `wtree pr clean [number]...` - Clean PR Worktrees
**Purpose**: Cleanup PR worktrees after review

```bash
# Usage examples
wtree pr clean                    # List all PR worktrees
wtree pr clean 1945              # Clean specific PR
wtree pr clean 1945 2001 2010    # Clean multiple PRs
wtree pr clean --all             # Clean all PR worktrees
wtree pr clean --merged          # Clean only merged PRs
```

**Behavior**:
1. If no args, list all PR worktrees with details
2. For each specified PR, remove worktree and tracking branch
3. Clean up PR metadata files
4. Show summary of cleaned items

## Global Options

### Standard Flags (Available on all commands)
```bash
--dry-run              # Show what would happen, don't execute
--verbose, -v          # Detailed output
--quiet, -q           # Minimal output
--force, -f           # Skip confirmations
--config FILE         # Use specific config file
--no-color           # Disable colored output
```

### Environment Flags (Available on create commands)
```bash
--install, -i         # Force dependency installation
--no-install         # Skip dependency installation
--no-env            # Skip .env file copying
--no-ssl            # Skip SSL certificate generation
--editor EDITOR     # Override default editor (cursor, code, vim)
```

## Help System Design

### Context-Aware Help
```bash
wtree --help                    # Main help
wtree create --help            # Create-specific help
wtree pr --help               # PR command group help
wtree pr clean --help         # PR clean-specific help
```

### Help Content Structure
```
wtree create - Create a new worktree

USAGE:
    wtree [create] <branch-name> [OPTIONS]

DESCRIPTION:
    Creates a new git worktree for parallel development without affecting
    your main workspace. The worktree will be created at:
    
        ../<repo-name>-<branch-name>

EXAMPLES:
    wtree feature/login                 Create worktree for feature/login
    wtree -i hotfix/bug                Create with dependency install
    wtree --no-env staging             Create without .env copying

OPTIONS:
    -i, --install              Install dependencies after creation
        --no-env              Skip .env file copying  
        --no-ssl              Skip SSL certificate generation
        --editor <editor>     Override default editor
        
GLOBAL OPTIONS:
    --dry-run                  Show what would happen
    -v, --verbose             Detailed output
    -f, --force               Skip confirmations
    
SEE ALSO:
    wtree delete               Delete worktrees
    wtree merge               Merge and cleanup
    wtree list                List active worktrees
```

## Output Design Philosophy

### 1. Progressive Disclosure
- Brief output by default
- `--verbose` for detailed information  
- `--quiet` for minimal output
- Progress indicators for long operations

### 2. Visual Hierarchy
```bash
# Success (green)
✓ Worktree created: /Users/awhite/Projects/myapp-feature-login

# Warning (yellow)  
⚠ Dependency installation failed, continuing...

# Error (red)
✗ Error: Not a git repository

# Info (blue)
ℹ Using editor: cursor
```

### 3. Actionable Errors
```bash
✗ Error: GitHub CLI not authenticated

Suggested actions:
  • Run 'gh auth login' to authenticate
  • Check 'gh auth status' for current state
  • Visit https://cli.github.com for installation help
```

### 4. Progress Indication
```bash
Creating worktree for 'feature/login'...
├─ Creating branch feature/login ✓
├─ Creating worktree at ../myapp-feature-login ✓  
├─ Copying .env file ✓
├─ Generating SSL certificates ✓
├─ Installing composer dependencies ⣾ (15s)
├─ Installing bun dependencies ✓
└─ Opening in cursor ✓

Worktree ready at: /Users/awhite/Projects/myapp-feature-login
```

This interface design prioritizes clarity, safety, and efficiency while maintaining the simplicity that makes the original shell functions so useful.