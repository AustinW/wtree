# WTree - Git Worktree Management Tool

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()

WTree is a powerful, generic git worktree management tool that streamlines your development workflow across any project type. It provides an intuitive CLI interface for creating, managing, and navigating between git worktrees with advanced features like interactive selection, smart cleanup, and multi-editor support.

## Features

### Core Worktree Management

- **Create & Delete**: Easy worktree creation with automatic path generation
- **Smart Switching**: Navigate between worktrees with shell integration
- **Interactive Mode**: Fuzzy-finding interface for branch selection
- **Status Tracking**: Comprehensive worktree status with git information

### Advanced UX Features

- **Dry-run Support**: Preview operations before execution
- **Multi-step Progress**: Visual progress indicators for complex operations
- **Shell Completion**: Tab completion for branches, paths, and commands
- **Rich Terminal Output**: Colorized output, spinners, and progress bars

### Editor Integration

- **Multi-editor Support**: VS Code, Cursor, vim, nvim, JetBrains IDEs, and more
- **Simultaneous Opening**: Open the same worktree in multiple editors
- **Terminal Integration**: Automatic terminal launching with worktree context

### Smart Operations

- **Intelligent Cleanup**: Auto-detect merged branches and stale worktrees
- **Project Configuration**: Per-project settings via `.wtreerc` files
- **Hook System**: Pre/post operation hooks for custom workflows

## Quick Start

### Installation

#### Option 1: Install Script (Recommended)
```bash
# One-line install (downloads latest release)
curl -sSL https://raw.githubusercontent.com/awhite/wtree/main/install.sh | bash
```

#### Option 2: Go Install (For Go users)
```bash
# Install directly from GitHub (requires Go 1.21+)
go install github.com/awhite/wtree@latest
```

#### Option 3: Homebrew (macOS/Linux)
```bash
# Add tap and install
brew tap awhite/tap
brew install wtree
```

#### Option 4: Package Managers

**Windows (Scoop):**
```bash
scoop bucket add awhite https://github.com/awhite/scoop-bucket
scoop install wtree
```

**Windows (Winget):**
```bash
winget install awhite.wtree
```

#### Option 5: Download Pre-built Binaries
1. Go to [Releases](https://github.com/awhite/wtree/releases)
2. Download the binary for your OS/architecture
3. Extract and move to your PATH

#### Option 6: Build from Source
```bash
git clone https://github.com/awhite/wtree.git
cd wtree
make install
```

### Basic Usage

```bash
# Create a worktree for an existing branch
wtree create feature-branch

# Create a new branch and worktree
wtree create -b new-feature main

# List all worktrees with status
wtree status

# Switch to a worktree (outputs shell command)
eval "$(wtree switch main)"

# Interactive branch selection
wtree interactive

# Smart cleanup of merged/stale worktrees
wtree cleanup --merged-only

# Open worktree in multiple editors
wtree editors feature-branch --editors code,vim --terminal
```

### Shell Integration

Enable tab completion for your shell:

```bash
# Bash
source <(wtree completion bash)

# Zsh
wtree completion zsh > "${fpath[1]}/_wtree"

# Fish
wtree completion fish | source
```

## Commands

| Command       | Description                   | Example                            |
| ------------- | ----------------------------- | ---------------------------------- |
| `create`      | Create a new worktree         | `wtree create -b feature main`     |
| `delete`      | Delete a worktree             | `wtree delete feature-branch`      |
| `list`        | List all worktrees            | `wtree list`                       |
| `status`      | Show detailed worktree status | `wtree status --verbose`           |
| `switch`      | Switch to a worktree          | `eval "$(wtree switch main)"`      |
| `cleanup`     | Smart worktree cleanup        | `wtree cleanup --dry-run`          |
| `interactive` | Interactive mode              | `wtree interactive --create`       |
| `editors`     | Open in multiple editors      | `wtree editors --editors code,vim` |
| `completion`  | Generate shell completions    | `wtree completion bash`            |

## Configuration

WTree supports both global and project-specific configuration:

### Global Configuration (`~/.config/wtree/config.yaml`)

```yaml
# Editor preferences
editor: cursor

# Worktree naming patterns
naming:
  pattern: "{{.ParentDir}}-{{.Branch}}"
  sanitize: true

# Automatic operations
auto:
  open_editor: false
  cleanup_on_delete: true

# UI preferences
ui:
  colors: true
  progress_bars: true
```

### Project Configuration (`.wtreerc`)

```yaml
# Project-specific editor
editor: code

# Custom setup hooks
hooks:
  post_create:
    - "npm install"
    - "cp .env.example .env"

# Project naming override
naming:
  pattern: "{{.ProjectName}}-{{.Branch}}"
```

## Use Cases

### Feature Development

```bash
# Start new feature
wtree create -b feature/auth main
eval "$(wtree switch feature/auth)"

# Open in your preferred editor
wtree editors . --terminal
```

### Code Review

```bash
# Quick worktree for PR review
wtree create pr-review
# ... make changes ...
wtree delete pr-review
```

### Parallel Development

```bash
# Work on multiple features simultaneously
wtree create -b feature-a main
wtree create -b feature-b main
wtree create -b hotfix main

# Switch between them easily
eval "$(wtree switch feature-a)"
# ... work on feature A ...
eval "$(wtree switch hotfix)"
# ... handle urgent fix ...
```

### Cleanup & Maintenance

```bash
# See what can be cleaned up
wtree cleanup --dry-run --verbose

# Clean up merged branches automatically
wtree cleanup --merged-only --auto
```

## Advanced Features

### Interactive Mode

Launch interactive mode for visual branch selection:

```bash
wtree interactive              # Browse all branches
wtree interactive --create     # Create mode
wtree interactive --cleanup    # Cleanup mode
wtree interactive --switch     # Switch mode
```

### Dry-run Operations

Preview changes before execution:

```bash
wtree create -b feature main --dry-run
wtree delete old-branch --dry-run
wtree cleanup --dry-run
```

### Multi-editor Workflows

Open the same worktree in multiple tools:

```bash
# Development setup: editor + terminal
wtree editors feature-branch --editors cursor --terminal

# Review setup: multiple editors
wtree editors . --editors code,vim
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:

- Setting up the development environment
- Code style and standards
- Testing procedures
- Pull request process

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by the power of git worktrees for parallel development
- Built with [Cobra](https://github.com/spf13/cobra) for the CLI interface
- Thanks to the Go community for excellent tooling and libraries

---

**Happy coding with WTree!**

For more detailed documentation, examples, and troubleshooting, visit our [documentation](docs/).
