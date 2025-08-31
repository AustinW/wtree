package types

import "fmt"

// ErrorType represents the category of error
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

func (et ErrorType) String() string {
	switch et {
	case ErrorTypeValidation:
		return "validation"
	case ErrorTypeGit:
		return "git"
	case ErrorTypeFileSystem:
		return "filesystem"
	case ErrorTypeNetwork:
		return "network"
	case ErrorTypeGitHub:
		return "github"
	case ErrorTypeEnvironment:
		return "environment"
	case ErrorTypeUser:
		return "user"
	case ErrorTypeInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// WTreeError is the base interface for all WTree errors
type WTreeError interface {
	error
	Type() ErrorType
	Operation() string
	Context() map[string]interface{}
	Recoverable() bool
	SuggestedActions() []string
	UserMessage() string
}

// BaseError provides a base implementation of WTreeError
type BaseError struct {
	errType          ErrorType
	operation        string
	message          string
	cause            error
	context          map[string]interface{}
	recoverable      bool
	suggestedActions []string
}

func (be *BaseError) Error() string {
	if be.cause != nil {
		return fmt.Sprintf("%s: %s: %v", be.operation, be.message, be.cause)
	}
	return fmt.Sprintf("%s: %s", be.operation, be.message)
}

func (be *BaseError) Type() ErrorType                          { return be.errType }
func (be *BaseError) Operation() string                        { return be.operation }
func (be *BaseError) Context() map[string]interface{}          { return be.context }
func (be *BaseError) Recoverable() bool                        { return be.recoverable }
func (be *BaseError) SuggestedActions() []string               { return be.suggestedActions }
func (be *BaseError) UserMessage() string                      { return be.message }
func (be *BaseError) Unwrap() error                            { return be.cause }

// Specific error types

// ValidationError represents validation failures
type ValidationError struct {
	*BaseError
}

func NewValidationError(operation, message string, cause error) *ValidationError {
	return &ValidationError{
		BaseError: &BaseError{
			errType:     ErrorTypeValidation,
			operation:   operation,
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

// GitError represents git operation failures
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

// FileSystemError represents filesystem operation failures
type FileSystemError struct {
	*BaseError
	Path string
}

func NewFileSystemError(operation, path, message string, cause error) *FileSystemError {
	return &FileSystemError{
		BaseError: &BaseError{
			errType:   ErrorTypeFileSystem,
			operation: operation,
			message:   message,
			cause:     cause,
			context: map[string]interface{}{
				"path": path,
			},
			recoverable: true,
			suggestedActions: []string{
				"Check file permissions",
				"Verify disk space availability",
				"Ensure parent directories exist",
			},
		},
		Path: path,
	}
}

// ConfigError represents configuration-related failures
type ConfigError struct {
	*BaseError
}

func NewConfigError(operation, message string, cause error) *ConfigError {
	return &ConfigError{
		BaseError: &BaseError{
			errType:     ErrorTypeValidation,
			operation:   operation,
			message:     message,
			cause:       cause,
			recoverable: false,
			suggestedActions: []string{
				"Check configuration file syntax",
				"Verify configuration file permissions",
				"Run 'wtree config init' to create default config",
			},
		},
	}
}

// HookError represents hook execution failures  
type HookError struct {
	*BaseError
}

func NewHookError(operation, message string, cause error) *HookError {
	return &HookError{
		BaseError: &BaseError{
			errType:     ErrorTypeValidation,
			operation:   operation,
			message:     message,
			cause:       cause,
			recoverable: false,
			suggestedActions: []string{
				"Check hook command syntax",
				"Verify hook command exists and is executable",
				"Review hook configuration in .wtreerc",
			},
		},
	}
}