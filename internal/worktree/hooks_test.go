package worktree

import (
	"testing"
	"time"

	"github.com/awhite/wtree/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestHookExecutor_expandCommand(t *testing.T) {
	config := &types.ProjectConfig{}
	executor := NewHookExecutor(config, 30*time.Second, false)

	ctx := types.HookContext{
		Branch:       "feature-branch",
		RepoPath:     "/path/to/repo",
		WorktreePath: "/path/to/worktree",
		TargetBranch: "main",
	}

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "no placeholders",
			command:  "echo hello",
			expected: "echo hello",
		},
		{
			name:     "branch placeholder",
			command:  "echo {branch}",
			expected: "echo feature-branch",
		},
		{
			name:     "multiple placeholders",
			command:  "cd {worktree_path} && git branch {branch}",
			expected: "cd /path/to/worktree && git branch feature-branch",
		},
		{
			name:     "repo name placeholder",
			command:  "echo {repo}",
			expected: "echo repo", // filepath.Base("/path/to/repo")
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.expandCommand(tt.command, ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHookExecutor_validateHookCommand(t *testing.T) {
	config := &types.ProjectConfig{}
	executor := NewHookExecutor(config, 30*time.Second, false)

	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:    "safe command",
			command: "npm install",
		},
		{
			name:        "dangerous rm command",
			command:     "rm -rf /",
			expectError: true,
		},
		{
			name:        "fork bomb",
			command:     ":(){ :|:& };:",
			expectError: true,
		},
		{
			name:        "dd command",
			command:     "dd if=/dev/zero of=/dev/sda",
			expectError: true,
		},
		{
			name:    "normal git command",
			command: "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.validateHookCommand(tt.command)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHookExecutor_buildEnvironment(t *testing.T) {
	config := &types.ProjectConfig{}
	executor := NewHookExecutor(config, 30*time.Second, false)

	ctx := types.HookContext{
		Event:        types.HookPostCreate,
		Branch:       "test-branch",
		RepoPath:     "/repo",
		WorktreePath: "/worktree",
		Environment: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	env := executor.buildEnvironment(ctx)

	// Check that our environment variables are present
	expectedVars := map[string]string{
		"WTREE_EVENT":         "post_create",
		"WTREE_BRANCH":        "test-branch",
		"WTREE_REPO_PATH":     "/repo",
		"WTREE_WORKTREE_PATH": "/worktree",
		"CUSTOM_VAR":          "custom_value",
	}

	for expectedKey, expectedValue := range expectedVars {
		found := false
		for _, envVar := range env {
			if envVar == expectedKey+"="+expectedValue {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected environment variable %s=%s not found", expectedKey, expectedValue)
	}
}

func TestHookExecutor_ValidateHooks(t *testing.T) {
	tests := []struct {
		name        string
		config      *types.ProjectConfig
		expectError bool
	}{
		{
			name: "valid hooks",
			config: &types.ProjectConfig{
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {"echo 'created'"},
					types.HookPreDelete:  {"echo 'deleting'"},
				},
			},
		},
		{
			name: "empty hook command",
			config: &types.ProjectConfig{
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {""},
				},
			},
			expectError: true,
		},
		{
			name: "dangerous hook command",
			config: &types.ProjectConfig{
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {"rm -rf /"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewHookExecutor(tt.config, 30*time.Second, false)
			err := executor.ValidateHooks()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
