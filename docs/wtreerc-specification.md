# .wtreerc Specification

## Overview

The `.wtreerc` file is a YAML configuration file that projects use to define their worktree setup behavior. It should be placed in the root of the git repository and defines project-specific hooks, file operations, and preferences.

## File Format

```yaml
# .wtreerc - Project worktree configuration
version: "1.0"  # Configuration format version

# Hook definitions - commands to run at specific events
hooks:
  pre_create: []    # Before worktree creation
  post_create: []   # After worktree creation, before editor
  pre_delete: []    # Before worktree deletion
  post_delete: []   # After worktree deletion
  pre_merge: []     # Before merge operation
  post_merge: []    # After merge operation

# File operations
copy_files: []      # Files/patterns to copy from main repo
link_files: []      # Files/patterns to symlink from main repo
ignore_files: []    # Files/patterns to never copy or link

# Naming and behavior
worktree_pattern: "{repo}-{branch}"  # Worktree directory naming
editor: ""          # Editor override for this project

# Execution settings
timeout: "5m"       # Hook execution timeout
allow_failure: false # Continue if hooks fail
verbose: false      # Show detailed hook output
```

## Hook Events

### `pre_create`
**When**: Before the git worktree is created
**Context**: Original repository directory
**Use cases**: 
- Validate prerequisites
- Check system requirements
- Prepare shared resources

**Example**:
```yaml
hooks:
  pre_create:
    - echo "Preparing to create worktree for {branch}"
    - ./scripts/check-prerequisites.sh
```

### `post_create`
**When**: After git worktree creation, after file operations, before editor opens
**Context**: New worktree directory
**Use cases**:
- Install dependencies
- Setup development environment
- Initialize databases
- Generate configuration files

**Example**:
```yaml
hooks:
  post_create:
    - cp .env.example .env
    - composer install
    - php artisan migrate
    - npm install && npm run build
```

### `pre_delete`
**When**: Before worktree deletion
**Context**: Worktree directory (before deletion)
**Use cases**:
- Backup important files
- Clean up resources
- Shutdown services

**Example**:
```yaml
hooks:
  pre_delete:
    - ./scripts/backup-logs.sh
    - docker-compose down
```

### `post_delete`
**When**: After worktree deletion
**Context**: Original repository directory
**Use cases**:
- Clean up shared resources
- Update global state

**Example**:
```yaml
hooks:
  post_delete:
    - echo "Cleaned up worktree for {branch}"
    - ./scripts/cleanup-shared-cache.sh
```

### `pre_merge` / `post_merge`
**When**: Before/after merge operations
**Context**: Original repository directory
**Use cases**:
- Run tests before merge
- Update documentation
- Deploy changes

**Example**:
```yaml
hooks:
  pre_merge:
    - npm run test
    - npm run lint
  post_merge:
    - ./scripts/update-docs.sh
    - ./scripts/deploy-staging.sh
```

## File Operations

### `copy_files`
Creates independent copies of files/directories from the main repo to the worktree.

**Syntax**: Supports glob patterns
**Behavior**: Files are copied and can be modified independently

**Examples**:
```yaml
copy_files:
  - .env                    # Copy single file
  - .env.*                  # Copy all .env files
  - config/*.yaml          # Copy all YAML files in config/
  - "storage/keys/*.pem"    # Copy certificate files
```

### `link_files`
Creates symbolic links from the worktree to files in the main repo.

**Syntax**: Supports glob patterns
**Behavior**: Changes in linked files affect all worktrees
**Use cases**: Shared dependencies, node_modules, vendor directories

**Examples**:
```yaml
link_files:
  - node_modules           # Share npm dependencies
  - vendor                 # Share composer dependencies  
  - .git/hooks            # Share git hooks
  - storage/app/public    # Share uploaded files
```

### `ignore_files`
Prevents files from being copied or linked, even if they match other patterns.

**Examples**:
```yaml
copy_files:
  - "config/*"
ignore_files:
  - "config/secrets.yaml"  # Don't copy sensitive files
```

## Variable Substitution

The following variables are available in hook commands and paths:

| Variable | Description | Example |
|----------|-------------|---------|
| `{repo}` | Repository name | `myapp` |
| `{branch}` | Current branch name | `feature/login` |
| `{target_branch}` | Target branch for merge | `main` |
| `{worktree_path}` | Full worktree path | `/path/to/myapp-feature-login` |
| `{repo_path}` | Main repository path | `/path/to/myapp` |

**Example**:
```yaml
hooks:
  post_create:
    - echo "Setting up {branch} in {worktree_path}"
    - ./scripts/setup.sh --branch={branch} --path={worktree_path}
```

## Configuration Examples

### Minimal Configuration
```yaml
# Just copy environment files
copy_files:
  - .env
```

### Basic Web Application
```yaml
hooks:
  post_create:
    - npm install
    - npm run build

copy_files:
  - .env
  - .env.local

link_files:
  - node_modules
```

### Advanced Laravel Application
```yaml
version: "1.0"

hooks:
  pre_create:
    - echo "Creating worktree for {branch}"
    
  post_create:
    - cp .env.example .env
    - composer install --no-dev --optimize-autoloader
    - php artisan key:generate
    - php artisan migrate --force
    - php artisan db:seed --class=DevelopmentSeeder
    - npm ci
    - npm run build
    - echo "Worktree ready at {worktree_path}"
    
  pre_delete:
    - php artisan queue:clear
    - php artisan cache:clear
    
  post_merge:
    - composer install --no-dev --optimize-autoloader
    - php artisan migrate --force
    - npm run build

copy_files:
  - .env
  - storage/oauth-private.key
  - storage/oauth-public.key

link_files:
  - node_modules
  - vendor
  - storage/app/public

worktree_pattern: "{repo}-{branch}"
editor: "cursor"
timeout: "10m"
allow_failure: false
verbose: true
```

### Microservice with Docker
```yaml
hooks:
  post_create:
    - docker-compose up -d db redis
    - sleep 5
    - go mod download
    - go run main.go migrate
    - make build
    
  pre_delete:
    - docker-compose down
    
copy_files:
  - .env
  - config/local.yaml

timeout: "15m"
```

### Python Django Project
```yaml
hooks:
  post_create:
    - python -m venv venv
    - source venv/bin/activate && pip install -r requirements.txt
    - source venv/bin/activate && python manage.py migrate
    - source venv/bin/activate && python manage.py collectstatic --noinput
    
copy_files:
  - .env
  - local_settings.py

worktree_pattern: "{repo}-branch-{branch}"
editor: "code"
```

## Error Handling

### Hook Failures
By default, if any hook fails, the entire operation stops. Control this with:

```yaml
allow_failure: true  # Continue even if hooks fail
```

### Timeout Protection
Prevent hanging operations:

```yaml
timeout: "10m"  # Kill hooks after 10 minutes
```

### Verbose Output
See detailed hook execution:

```yaml
verbose: true  # Show all hook output
```

## Environment Variables

Hooks run with these environment variables set:

| Variable | Value |
|----------|-------|
| `WTREE_EVENT` | Current hook event (pre_create, post_create, etc.) |
| `WTREE_BRANCH` | Branch name |
| `WTREE_REPO_PATH` | Main repository path |
| `WTREE_WORKTREE_PATH` | Worktree path |
| `WTREE_TARGET_BRANCH` | Target branch (for merge operations) |

**Example usage in scripts**:
```bash
#!/bin/bash
# scripts/setup.sh
echo "Setting up $WTREE_BRANCH"
echo "Repo: $WTREE_REPO_PATH"
echo "Worktree: $WTREE_WORKTREE_PATH"
```

## Best Practices

### 1. Keep Hooks Fast
- Use parallel operations where possible
- Cache downloads and builds
- Skip unnecessary operations

### 2. Handle Failures Gracefully
```yaml
hooks:
  post_create:
    - composer install || echo "Composer install failed, continuing..."
    - npm install || true  # Continue on failure
```

### 3. Use Absolute Paths in Scripts
```bash
# Good
./scripts/setup.sh

# Better - works from any directory
$WTREE_REPO_PATH/scripts/setup.sh
```

### 4. Test Hooks Regularly
Create a test worktree to verify hooks work correctly:
```bash
wtree test-branch
```

### 5. Document Project Setup
Include comments in `.wtreerc` to explain complex setups:
```yaml
hooks:
  post_create:
    # Install PHP dependencies
    - composer install
    # Setup database with test data
    - php artisan migrate:fresh --seed
    # Build frontend assets
    - npm run build
```

This specification provides a comprehensive, flexible system for projects to define their worktree setup behavior while keeping WTree itself completely generic.