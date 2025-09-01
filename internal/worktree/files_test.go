package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileManager_CopyFiles(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "wtree-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(dstDir, 0755))

	// Create test files
	testFile := filepath.Join(srcDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))
	
	testDir := filepath.Join(srcDir, "testdir")
	require.NoError(t, os.MkdirAll(testDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "nested.txt"), []byte("nested"), 0644))

	fm := NewFileManager(false)

	tests := []struct {
		name         string
		patterns     []string
		ignorePatterns []string
		expectError  bool
		expectedFiles []string
	}{
		{
			name:     "copy single file",
			patterns: []string{"test.txt"},
			expectedFiles: []string{"test.txt"},
		},
		{
			name:     "copy directory",
			patterns: []string{"testdir"},
			expectedFiles: []string{"testdir/nested.txt"},
		},
		{
			name:     "glob pattern",
			patterns: []string{"*.txt"},
			expectedFiles: []string{"test.txt"},
		},
		{
			name:           "ignore pattern",
			patterns:       []string{"*"},
			ignorePatterns: []string{"test.txt"},
			expectedFiles:  []string{"testdir/nested.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean destination
			os.RemoveAll(dstDir)
			_ = os.MkdirAll(dstDir, 0755)

			err := fm.CopyFiles(tt.patterns, srcDir, dstDir, tt.ignorePatterns)
			
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			
			// Check expected files exist
			for _, expectedFile := range tt.expectedFiles {
				expectedPath := filepath.Join(dstDir, expectedFile)
				assert.FileExists(t, expectedPath)
			}
		})
	}
}

func TestFileManager_shouldIgnoreFile(t *testing.T) {
	fm := NewFileManager(false)

	tests := []struct {
		name           string
		filePath       string
		ignorePatterns []string
		expected       bool
	}{
		{
			name:           "no patterns",
			filePath:       "file.txt",
			ignorePatterns: []string{},
			expected:       false,
		},
		{
			name:           "exact match",
			filePath:       "ignore.txt",
			ignorePatterns: []string{"ignore.txt"},
			expected:       true,
		},
		{
			name:           "glob pattern",
			filePath:       "file.log",
			ignorePatterns: []string{"*.log"},
			expected:       true,
		},
		{
			name:           "directory match",
			filePath:       "logs/debug.txt",
			ignorePatterns: []string{"logs"},
			expected:       true,
		},
		{
			name:           "no match",
			filePath:       "keep.txt",
			ignorePatterns: []string{"*.log", "temp"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fm.shouldIgnoreFile(tt.filePath, tt.ignorePatterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileManager_ValidateFilePatterns(t *testing.T) {
	fm := NewFileManager(false)

	tests := []struct {
		name        string
		patterns    []string
		expectError bool
	}{
		{
			name:     "valid patterns",
			patterns: []string{"*.txt", "src/**", "file.js"},
		},
		{
			name:        "absolute path",
			patterns:    []string{"/etc/passwd"},
			expectError: true,
		},
		{
			name:        "path traversal",
			patterns:    []string{"../../../etc/passwd"},
			expectError: true,
		},
		{
			name:        "suspicious path",
			patterns:    []string{"./../../file"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fm.ValidateFilePatterns(tt.patterns)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}