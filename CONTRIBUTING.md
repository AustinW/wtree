# Contributing to WTree

Thank you for your interest in contributing to WTree! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- A terminal/shell environment

### Development Setup

1. **Fork and Clone**

   ```bash
   git clone https://github.com/YOUR_USERNAME/wtree.git
   cd wtree
   ```

2. **Install Dependencies**

   ```bash
   go mod download
   ```

3. **Build and Test**

   ```bash
   # Build the project
   go build -o wtree

   # Run tests
   go test ./...

   # Run with verbose output
   go test -v ./...
   ```

4. **Verify Installation**
   ```bash
   ./wtree --help
   ```

## Project Structure

```
wtree/
├── cmd/                    # CLI commands and entry points
│   ├── create.go          # Create worktree command
│   ├── delete.go          # Delete worktree command
│   ├── interactive.go     # Interactive mode
│   └── ...
├── internal/              # Internal packages
│   ├── config/           # Configuration management
│   ├── git/              # Git operations
│   ├── ui/               # User interface components
│   └── worktree/         # Core worktree logic
├── pkg/                  # Public packages
│   └── types/            # Shared types and structures
├── docs/                 # Documentation
├── main.go              # Application entry point
├── go.mod               # Go module definition
└── go.sum               # Go module checksums
```

## Development Guidelines

### Code Style

- **Go Formatting**: Use `gofmt` or `goimports` to format your code
- **Linting**: Run `golangci-lint run` before submitting
- **Comments**: Document exported functions and complex logic
- **Error Handling**: Always handle errors appropriately

### Naming Conventions

- **Files**: Use snake_case (e.g., `worktree_manager.go`)
- **Functions**: Use PascalCase for exported, camelCase for unexported
- **Variables**: Use camelCase
- **Constants**: Use PascalCase or ALL_CAPS for package-level constants

### Testing

- Write tests for new functionality
- Maintain or improve test coverage
- Use table-driven tests where appropriate
- Mock external dependencies (git, filesystem, etc.)

**Example Test:**

```go
func TestWorktreeManager_Create(t *testing.T) {
    tests := []struct {
        name        string
        branchName  string
        options     CreateOptions
        expectError bool
    }{
        {
            name:       "valid branch creation",
            branchName: "feature-test",
            options:    CreateOptions{CreateBranch: true},
            expectError: false,
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Contribution Types

### Bug Fixes

- Search existing issues before creating a new one
- Include steps to reproduce the bug
- Provide system information (OS, Go version, etc.)
- Include proposed fix or workaround if available

### New Features

- Discuss large features in an issue first
- Follow the existing command structure and patterns
- Add appropriate tests and documentation
- Update relevant help text and examples

### Documentation

- Fix typos, improve clarity, add examples
- Keep README.md up to date with new features
- Update command help text and descriptions
- Add code comments for complex logic

### Performance Improvements

- Include benchmarks showing improvement
- Ensure changes don't break existing functionality
- Document any breaking changes

## Pull Request Process

### Before Submitting

1. **Create a Branch**

   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/issue-description
   ```

2. **Make Your Changes**

   - Follow the coding guidelines above
   - Write or update tests
   - Update documentation if needed

3. **Test Your Changes**

   ```bash
   go test ./...
   go build -o wtree
   ./wtree --help  # Basic smoke test
   ```

4. **Commit Your Changes**

   ```bash
   git add .
   git commit -m "feat: add interactive branch selection

   - Implement fuzzy-finding for branch selection
   - Add support for create/cleanup/switch modes
   - Include comprehensive error handling

   Closes #123"
   ```

### Commit Message Format

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types:**

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `style:` Code style changes (formatting, etc.)
- `refactor:` Code refactoring
- `test:` Adding or updating tests
- `chore:` Maintenance tasks

**Examples:**

```
feat(interactive): add fuzzy-finding for branch selection
fix(cleanup): handle edge case with empty worktree list
docs: update installation instructions for macOS
test(manager): add unit tests for Create method
```

### Pull Request Guidelines

1. **Fill out the PR template completely**
2. **Link related issues** (e.g., "Closes #123", "Fixes #456")
3. **Provide clear description** of changes and motivation
4. **Include screenshots/demos** for UI changes
5. **Update documentation** if behavior changes
6. **Ensure CI passes** (tests, linting, formatting)

### Code Review Process

- All PRs require at least one review
- Address feedback promptly and professionally
- Make requested changes in separate commits
- Once approved, maintainers will merge the PR

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/worktree

# Run with verbose output
go test -v ./...

# Run specific test
go test -run TestWorktreeManager_Create ./internal/worktree
```

### Writing Tests

- Place tests in `*_test.go` files
- Use the standard `testing` package
- Create mocks for external dependencies
- Test both success and error cases
- Use table-driven tests for multiple scenarios

### Test Categories

1. **Unit Tests**: Test individual functions/methods
2. **Integration Tests**: Test component interactions
3. **End-to-end Tests**: Test complete workflows

## Reporting Issues

### Bug Reports

Include the following information:

- **WTree version**: `wtree --version`
- **Go version**: `go version`
- **Operating system**: OS and version
- **Steps to reproduce**: Clear, minimal steps
- **Expected behavior**: What should happen
- **Actual behavior**: What actually happened
- **Error messages**: Full error output
- **Additional context**: Screenshots, logs, etc.

### Feature Requests

- Explain the use case and motivation
- Describe the proposed solution
- Consider alternative approaches
- Indicate willingness to contribute

## Performance Considerations

- Profile code changes that might affect performance
- Consider memory usage for large repositories
- Optimize for common use cases
- Avoid blocking operations in UI code

## Security

- Never commit secrets, tokens, or credentials
- Be cautious with file operations and path traversal
- Validate user input appropriately
- Follow secure coding practices

## Recognition

Contributors are recognized in several ways:

- Listed in release notes for significant contributions
- Added to CONTRIBUTORS.md (if it exists)
- Mentioned in commit messages and PR descriptions

## Getting Help

- **Documentation**: Check the `docs/` directory
- **Issues**: Browse existing issues for similar problems
- **Discussions**: Use GitHub Discussions for questions
- **Code**: Look at existing implementations for patterns

## Checklist for Contributors

Before submitting a PR, ensure:

- [ ] Code follows project style guidelines
- [ ] Tests are written and pass (`go test ./...`)
- [ ] Documentation is updated if needed
- [ ] Commit messages follow conventional format
- [ ] PR description is clear and complete
- [ ] Related issues are linked
- [ ] No breaking changes without discussion

Thank you for contributing to WTree! 🌳
