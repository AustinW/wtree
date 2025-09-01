package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awhite/wtree/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_LoadProjectConfig(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
		expected    *types.ProjectConfig
	}{
		{
			name: "valid config",
			configData: `
version: "1.0"
worktree_pattern: "{repo}-{branch}"
copy_files:
  - ".env.example"
link_files:
  - "node_modules"
hooks:
  post_create:
    - "echo 'created'"
`,
			expectError: false,
			expected: &types.ProjectConfig{
				Version:         "1.0",
				WorktreePattern: "{repo}-{branch}",
				CopyFiles:       []string{".env.example"},
				LinkFiles:       []string{"node_modules"},
				Hooks: map[types.HookEvent][]string{
					types.HookPostCreate: {"echo 'created'"},
				},
			},
		},
		{
			name:        "default config when no file",
			configData:  "",
			expectError: false,
			expected:    types.DefaultProjectConfig(),
		},
		{
			name:        "invalid yaml",
			configData:  "invalid: yaml: content:",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
					tmpDir, err := os.MkdirTemp("", "wtree-test")
		require.NoError(t, err)
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				t.Logf("Warning: failed to clean up temp dir: %v", err)
			}
		}()

			// Create .wtreerc if config data provided
			if tt.configData != "" && tt.name != "default config when no file" {
				configPath := filepath.Join(tmpDir, ".wtreerc")
				err := os.WriteFile(configPath, []byte(tt.configData), 0644)
				require.NoError(t, err)
			}

			// Test loading
			manager := NewManager()
			config, err := manager.LoadProjectConfig(tmpDir)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected != nil {
					assert.Equal(t, tt.expected.Version, config.Version)
					assert.Equal(t, tt.expected.WorktreePattern, config.WorktreePattern)
					assert.Equal(t, tt.expected.CopyFiles, config.CopyFiles)
					assert.Equal(t, tt.expected.LinkFiles, config.LinkFiles)
				}
			}
		})
	}
}

func TestManager_validateProjectConfig(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name        string
		config      *types.ProjectConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &types.ProjectConfig{
				Version:   "1.0",
				CopyFiles: []string{"file.txt"},
			},
			expectError: false,
		},
		{
			name: "invalid version",
			config: &types.ProjectConfig{
				Version: "2.0",
			},
			expectError: true,
		},
		{
			name: "absolute file path",
			config: &types.ProjectConfig{
				Version:   "1.0",
				CopyFiles: []string{"/absolute/path"},
			},
			expectError: true,
		},
		{
			name: "path traversal with dots",
			config: &types.ProjectConfig{
				Version:   "1.0",
				CopyFiles: []string{"../../passwd"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateProjectConfig(tt.config, "/tmp")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_ResolveEditor(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name           string
		globalConfig   *types.WTreeConfig
		projectConfig  *types.ProjectConfig
		expectedEditor string
	}{
		{
			name:           "project override",
			globalConfig:   &types.WTreeConfig{Editor: "vim"},
			projectConfig:  &types.ProjectConfig{Editor: "code"},
			expectedEditor: "code",
		},
		{
			name:           "global fallback",
			globalConfig:   &types.WTreeConfig{Editor: "vim"},
			projectConfig:  &types.ProjectConfig{},
			expectedEditor: "vim",
		},
		{
			name:           "default fallback",
			globalConfig:   &types.WTreeConfig{},
			projectConfig:  nil,
			expectedEditor: "cursor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.ResolveEditor(tt.globalConfig, tt.projectConfig)
			assert.Equal(t, tt.expectedEditor, result)
		})
	}
}
