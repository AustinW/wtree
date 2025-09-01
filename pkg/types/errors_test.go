package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationError(t *testing.T) {
	err := NewValidationError("test-operation", "test message", errors.New("underlying error"))

	assert.Equal(t, ErrorTypeValidation, err.Type())
	assert.Equal(t, "test-operation", err.Operation())
	assert.Equal(t, "test message", err.UserMessage())
	assert.NotNil(t, err.Unwrap())

	expectedError := "test-operation: test message: underlying error"
	assert.Equal(t, expectedError, err.Error())
}

func TestGitError(t *testing.T) {
	err := NewGitError("test-operation", "git command failed", errors.New("exit status 1"))

	assert.Equal(t, ErrorTypeGit, err.Type())
	assert.Equal(t, "test-operation", err.Operation())
	assert.Equal(t, "git command failed", err.UserMessage())
	assert.NotNil(t, err.Unwrap())

	expectedError := "test-operation: git command failed: exit status 1"
	assert.Equal(t, expectedError, err.Error())
}

func TestFileSystemError(t *testing.T) {
	err := NewFileSystemError("file-copy", "/test/path", "failed to copy file", nil)

	assert.Equal(t, ErrorTypeFileSystem, err.Type())
	assert.Equal(t, "file-copy", err.Operation())
	assert.Equal(t, "failed to copy file", err.UserMessage())
	assert.Nil(t, err.Unwrap())
	assert.Equal(t, "/test/path", err.Path)

	expectedError := "file-copy: failed to copy file"
	assert.Equal(t, expectedError, err.Error())
}

func TestConfigError(t *testing.T) {
	err := NewConfigError("config-load", "invalid yaml format", errors.New("yaml parse error"))

	assert.Equal(t, ErrorTypeValidation, err.Type())
	assert.Equal(t, "config-load", err.Operation())
	assert.Equal(t, "invalid yaml format", err.UserMessage())
	assert.NotNil(t, err.Unwrap())

	expectedError := "config-load: invalid yaml format: yaml parse error"
	assert.Equal(t, expectedError, err.Error())
}

func TestHookError(t *testing.T) {
	err := NewHookError("post-create", "hook execution failed", errors.New("command not found"))

	assert.Equal(t, ErrorTypeValidation, err.Type())
	assert.Equal(t, "post-create", err.Operation())
	assert.Equal(t, "hook execution failed", err.UserMessage())
	assert.NotNil(t, err.Unwrap())

	expectedError := "post-create: hook execution failed: command not found"
	assert.Equal(t, expectedError, err.Error())
}

func TestErrorWithoutUnderlying(t *testing.T) {
	err := NewValidationError("test", "message only", nil)

	expectedError := "test: message only"
	assert.Equal(t, expectedError, err.Error())
}

func TestIsWTreeError(t *testing.T) {
	wtreeErr := NewGitError("test", "message", nil)
	regularErr := errors.New("regular error")

	// Test with WTreeError
	var err error = wtreeErr
	if wtreeError, ok := err.(WTreeError); ok {
		assert.Equal(t, ErrorTypeGit, wtreeError.Type())
		assert.Equal(t, "test", wtreeError.Operation())
	} else {
		t.Fatal("Expected WTreeError")
	}

	// Test with regular error
	err = regularErr
	if _, ok := err.(WTreeError); ok {
		t.Fatal("Should not be WTreeError")
	}
}
