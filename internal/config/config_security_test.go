package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awhite/wtree/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManager_PathTraversalValidation tests comprehensive path traversal validation
func TestManager_PathTraversalValidation(t *testing.T) {
	manager := NewManager()

	// Create temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "wtree-config-security")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		pattern     string
		expectError bool
		description string
	}{
		// Safe patterns that should be allowed
		{
			name:        "simple filename",
			pattern:     "config.json",
			expectError: false,
			description: "simple filenames should be allowed",
		},
		{
			name:        "relative path",
			pattern:     "src/config.json",
			expectError: false,
			description: "relative paths within project should be allowed",
		},
		{
			name:        "glob pattern",
			pattern:     "*.json",
			expectError: false,
			description: "glob patterns should be allowed",
		},
		{
			name:        "file with dots in name",
			pattern:     "app..config",
			expectError: false,
			description: "files with dots in names should be allowed (was previously blocked)",
		},
		{
			name:        "file with multiple dots",
			pattern:     "file...backup",
			expectError: false,
			description: "files with multiple dots should be allowed",
		},
		{
			name:        "nested directory",
			pattern:     "src/components/Button.tsx",
			expectError: false,
			description: "nested directory paths should be allowed",
		},

		// Dangerous patterns that should be blocked
		{
			name:        "absolute path",
			pattern:     "/etc/passwd",
			expectError: true,
			description: "absolute paths should be blocked",
		},
		{
			name:        "simple path traversal",
			pattern:     "../config.json",
			expectError: true,
			description: "simple path traversal should be blocked",
		},
		{
			name:        "nested path traversal",
			pattern:     "../../etc/passwd",
			expectError: true,
			description: "nested path traversal should be blocked",
		},
		{
			name:        "path traversal in middle",
			pattern:     "src/../../../etc/passwd",
			expectError: true,
			description: "path traversal in middle of path should be blocked",
		},
		{
			name:        "just parent directory",
			pattern:     "..",
			expectError: true,
			description: "bare parent directory reference should be blocked",
		},
		{
			name:        "parent directory with trailing slash",
			pattern:     "../",
			expectError: true,
			description: "parent directory with slash should be blocked",
		},
		{
			name:        "complex traversal attempt",
			pattern:     "src/./../../bin/malicious",
			expectError: true,
			description: "complex traversal attempts should be blocked",
		},
		{
			name:        "Windows-style absolute path",
			pattern:     "C:\\Windows\\System32\\config",
			expectError: true,
			description: "Windows absolute paths should be blocked",
		},
		{
			name:        "empty pattern",
			pattern:     "",
			expectError: true,
			description: "empty patterns should be blocked",
		},
		{
			name:        "whitespace only pattern",
			pattern:     "   ",
			expectError: true,
			description: "whitespace-only patterns should be blocked",
		},

		// Edge cases that could bypass naive validation
		{
			name:        "dot-dot with extra characters",
			pattern:     "..malicious",
			expectError: false,
			description: "patterns starting with .. but not path traversal should be allowed",
		},
		{
			name:        "double-dot in filename",
			pattern:     "config..backup",
			expectError: false,
			description: "double dots within filename should be allowed",
		},
		{
			name:        "path traversal with normalization",
			pattern:     "src/.././../etc/passwd",
			expectError: true,
			description: "path traversal that needs normalization should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateFilePattern(tt.pattern, tmpDir)

			if tt.expectError {
				assert.Error(t, err, "Expected error for pattern: %s (%s)", tt.pattern, tt.description)
				if err != nil {
					// Verify the error message is meaningful
					assert.True(t, len(err.Error()) > 0, "Error message should not be empty")
				}
			} else {
				assert.NoError(t, err, "Expected no error for pattern: %s (%s)", tt.pattern, tt.description)
			}
		})
	}
}

// TestManager_ProjectConfigSecurityValidation tests full project config validation
func TestManager_ProjectConfigSecurityValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-project-validation")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager()

	tests := []struct {
		name        string
		config      *types.ProjectConfig
		expectError bool
		description string
	}{
		{
			name: "safe configuration",
			config: &types.ProjectConfig{
				Version: "1.0",
				CopyFiles: []string{
					"package.json",
					"*.env.example",
					"src/**/*.ts",
				},
				LinkFiles: []string{
					"node_modules",
					".git/hooks",
				},
			},
			expectError: false,
			description: "safe file patterns should be allowed",
		},
		{
			name: "configuration with legitimate dot files",
			config: &types.ProjectConfig{
				Version: "1.0",
				CopyFiles: []string{
					"app..config.json",
					"backup...old",
					"script.production.js",
				},
			},
			expectError: false,
			description: "legitimate files with dots should be allowed",
		},
		{
			name: "malicious copy files configuration",
			config: &types.ProjectConfig{
				Version: "1.0",
				CopyFiles: []string{
					"package.json",
					"../../../etc/passwd", // Malicious!
				},
			},
			expectError: true,
			description: "path traversal in copy files should be blocked",
		},
		{
			name: "malicious link files configuration",
			config: &types.ProjectConfig{
				Version: "1.0",
				LinkFiles: []string{
					"/etc/passwd", // Absolute path - malicious!
				},
			},
			expectError: true,
			description: "absolute paths in link files should be blocked",
		},
		{
			name: "mixed safe and malicious patterns",
			config: &types.ProjectConfig{
				Version: "1.0",
				CopyFiles: []string{
					"*.json",      // Safe
					"../secrets",  // Malicious!
				},
			},
			expectError: true,
			description: "mixed patterns with any malicious should be blocked",
		},
		{
			name: "empty pattern in list",
			config: &types.ProjectConfig{
				Version: "1.0",
				CopyFiles: []string{
					"file.txt",
					"",  // Empty pattern
					"other.txt",
				},
			},
			expectError: true,
			description: "empty patterns should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateProjectConfig(tt.config, tmpDir)

			if tt.expectError {
				assert.Error(t, err, "Expected error for config: %s", tt.description)
				// Check that it's specifically a validation error
				assert.True(t, strings.Contains(err.Error(), "invalid file pattern") || 
						   strings.Contains(err.Error(), "validation"), 
					"Should be a validation error: %v", err)
			} else {
				assert.NoError(t, err, "Expected no error for config: %s", tt.description)
			}
		})
	}
}

// TestManager_SymlinkInRepoPath tests handling of symlinks in repository path
func TestManager_SymlinkInRepoPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-symlink-repo")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create actual repository directory
	realRepoDir := filepath.Join(tmpDir, "real-repo")
	require.NoError(t, os.MkdirAll(realRepoDir, 0755))

	// Create symlink to repository
	symlinkRepoDir := filepath.Join(tmpDir, "symlink-repo")
	require.NoError(t, os.Symlink(realRepoDir, symlinkRepoDir))

	manager := NewManager()

	// Test validation with symlinked repository path
	err = manager.validateFilePattern("config.json", symlinkRepoDir)
	assert.NoError(t, err, "Should handle symlinked repository paths correctly")

	// Test that traversal is still blocked even with symlinked repo path
	err = manager.validateFilePattern("../outside.txt", symlinkRepoDir)
	assert.Error(t, err, "Should still block traversal attempts with symlinked repo path")
}

// TestManager_ConcurrentValidation tests thread safety of validation
func TestManager_ConcurrentValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-concurrent")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager()

	// Run multiple validations concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Mix of safe and unsafe patterns
			patterns := []string{
				"config.json",
				"../../../etc/passwd",
				"src/components/*.tsx",
				"/absolute/path",
			}
			
			for _, pattern := range patterns {
				_ = manager.validateFilePattern(pattern, tmpDir)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without deadlock or panic, the test passes
	assert.True(t, true, "Concurrent validation should not cause issues")
}

// TestManager_EdgeCasePatterns tests edge cases that could bypass security
func TestManager_EdgeCasePatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-edge-cases")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager()

	edgeCases := []struct {
		pattern     string
		expectError bool
		description string
	}{
		{
			pattern:     "./normal-file.txt",
			expectError: false,
			description: "current directory prefix should be allowed",
		},
		{
			pattern:     "./../../escape.txt",
			expectError: true,
			description: "traversal with current directory prefix should be blocked",
		},
		{
			pattern:     "normal/../file.txt",
			expectError: true,
			description: "traversal in middle of path should be blocked",
		},
		{
			pattern:     "normal/./file.txt",
			expectError: false,
			description: "current directory in path should be allowed",
		},
		{
			pattern:     strings.Repeat("a", 1000),
			expectError: false,
			description: "very long filename should be allowed",
		},
		{
			pattern:     "file\x00name.txt",
			expectError: false,
			description: "null byte in filename should be handled gracefully",
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.description, func(t *testing.T) {
			err := manager.validateFilePattern(tc.pattern, tmpDir)
			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				assert.NoError(t, err, tc.description)
			}
		})
	}
}

// BenchmarkPathValidation benchmarks the path validation performance
func BenchmarkPathValidation(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wtree-benchmark")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager()
	
	testPatterns := []string{
		"simple.txt",
		"src/components/Button.tsx",
		"../../../etc/passwd",
		"*.json",
		"app..config.backup",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pattern := range testPatterns {
			_ = manager.validateFilePattern(pattern, tmpDir)
		}
	}
}