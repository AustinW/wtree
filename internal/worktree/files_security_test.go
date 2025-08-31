package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileManager_SymlinkDetection tests detection of malicious symlinks
func TestFileManager_SymlinkDetection(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "wtree-security-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	outsideDir := filepath.Join(tmpDir, "outside")

	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(dstDir, 0755))
	require.NoError(t, os.MkdirAll(outsideDir, 0755))

	// Create a sensitive file outside the source directory
	sensitiveFile := filepath.Join(outsideDir, "sensitive.txt")
	require.NoError(t, os.WriteFile(sensitiveFile, []byte("sensitive data"), 0644))

	// Create a malicious symlink in source directory pointing outside
	symlinkPath := filepath.Join(srcDir, "malicious_link.txt")
	require.NoError(t, os.Symlink(sensitiveFile, symlinkPath))

	// Set up file manager with security restrictions
	fm := NewFileManager(false)
	err = fm.SetBasePath(srcDir)
	require.NoError(t, err)

	tests := []struct {
		name        string
		patterns    []string
		expectError bool
		description string
	}{
		{
			name:        "malicious symlink should be blocked",
			patterns:    []string{"malicious_link.txt"},
			expectError: true,
			description: "should detect and block symlinks pointing outside allowed directory",
		},
		{
			name:        "glob pattern matching malicious symlink should be blocked",
			patterns:    []string{"*.txt"},
			expectError: true,
			description: "should detect malicious symlinks even when matched by glob pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean destination
			os.RemoveAll(dstDir)
			os.MkdirAll(dstDir, 0755)

			err := fm.CopyFiles(tt.patterns, srcDir, dstDir, nil)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.True(t, strings.Contains(err.Error(), "symlink"), 
					"Error should mention symlink: %v", err)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestFileManager_PathBoundaryEnforcement tests path boundary validation
func TestFileManager_PathBoundaryEnforcement(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "wtree-boundary-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	allowedDir := filepath.Join(tmpDir, "allowed")
	forbiddenDir := filepath.Join(tmpDir, "forbidden")

	require.NoError(t, os.MkdirAll(allowedDir, 0755))
	require.NoError(t, os.MkdirAll(forbiddenDir, 0755))

	// Create files in both directories
	allowedFile := filepath.Join(allowedDir, "safe.txt")
	forbiddenFile := filepath.Join(forbiddenDir, "forbidden.txt")
	require.NoError(t, os.WriteFile(allowedFile, []byte("safe content"), 0644))
	require.NoError(t, os.WriteFile(forbiddenFile, []byte("forbidden content"), 0644))

	// Set up file manager with security restrictions
	fm := NewFileManager(false)
	err = fm.SetBasePath(allowedDir)
	require.NoError(t, err)

	tests := []struct {
		name        string
		testPath    string
		operation   string
		expectError bool
		description string
	}{
		{
			name:        "allowed path should pass validation",
			testPath:    allowedFile,
			operation:   "copy",
			expectError: false,
			description: "should allow operations within base directory",
		},
		{
			name:        "forbidden path should be blocked",
			testPath:    forbiddenFile,
			operation:   "copy",
			expectError: true,
			description: "should block operations outside base directory",
		},
		{
			name:        "path traversal attempt should be blocked",
			testPath:    filepath.Join(allowedDir, "../forbidden/forbidden.txt"),
			operation:   "copy",
			expectError: true,
			description: "should block path traversal attempts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fm.validatePathSecurity(tt.testPath, tt.operation)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestFileManager_NestedSymlinkAttack tests complex nested symlink attacks
func TestFileManager_NestedSymlinkAttack(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "wtree-nested-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	targetDir := filepath.Join(tmpDir, "target")
	forbiddenDir := filepath.Join(tmpDir, "forbidden")

	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.MkdirAll(forbiddenDir, 0755))

	// Create forbidden file
	forbiddenFile := filepath.Join(forbiddenDir, "secret.txt")
	require.NoError(t, os.WriteFile(forbiddenFile, []byte("secret content"), 0644))

	// Create nested symlinks: src/link1 -> target, target/link2 -> forbidden/secret.txt
	link1 := filepath.Join(srcDir, "link1")
	link2 := filepath.Join(targetDir, "link2")
	require.NoError(t, os.Symlink(targetDir, link1))
	require.NoError(t, os.Symlink(forbiddenFile, link2))

	// Set up file manager
	fm := NewFileManager(false)
	err = fm.SetBasePath(srcDir)
	require.NoError(t, err)

	// This should detect that the symlink chain leads outside allowed directory
	nestedSymlinkTarget := filepath.Join(link1, "link2")
	err = fm.validatePathSecurity(nestedSymlinkTarget, "copy")
	assert.Error(t, err, "Should detect nested symlink attack")
}

// TestFileManager_ResourceCleanup tests proper resource management
func TestFileManager_ResourceCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-resource-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	require.NoError(t, os.MkdirAll(srcDir, 0755))

	// Create source file
	srcFile := filepath.Join(srcDir, "test.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("test content"), 0644))

	// Create destination directory as a file to force copy failure
	require.NoError(t, os.WriteFile(dstDir, []byte("blocking file"), 0644))

	fm := NewFileManager(false)
	dstFile := filepath.Join(dstDir, "test.txt")

	// This should fail because dstDir is a file, not a directory
	err = fm.copyFile(srcFile, dstFile)
	assert.Error(t, err, "Copy should fail due to directory creation failure")

	// Verify no partial files were left behind
	_, err = os.Stat(dstFile)
	assert.True(t, os.IsNotExist(err), "Partial destination file should be cleaned up")
}

// TestFileManager_SecurityLogging tests security event logging
func TestFileManager_SecurityLogging(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-logging-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	outsideDir := filepath.Join(tmpDir, "outside")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(outsideDir, 0755))

	// Create malicious symlink
	outsideFile := filepath.Join(outsideDir, "target.txt")
	symlinkFile := filepath.Join(srcDir, "symlink.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside content"), 0644))
	require.NoError(t, os.Symlink(outsideFile, symlinkFile))

	fm := NewFileManager(true) // verbose logging
	err = fm.SetBasePath(srcDir)
	require.NoError(t, err)

	// This should trigger security logging
	err = fm.validatePathSecurity(symlinkFile, "test-operation")
	assert.Error(t, err, "Should detect security violation")

	// Note: In a real implementation, you might capture log output and verify
	// specific security messages were logged
}

// TestFileManager_LegitimateFilesWithDots tests that legitimate files are not blocked
func TestFileManager_LegitimateFilesWithDots(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-dots-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(dstDir, 0755))

	// Create legitimate files that contain dots but are not traversal attempts
	legitimateFiles := []string{
		"app.config.json",
		"file..backup",
		"script...old",
		"package-lock.json",
		"app.env.example",
	}

	for _, filename := range legitimateFiles {
		filepath := filepath.Join(srcDir, filename)
		require.NoError(t, os.WriteFile(filepath, []byte("content"), 0644))
	}

	fm := NewFileManager(false)
	err = fm.SetBasePath(srcDir)
	require.NoError(t, err)

	// All legitimate files should be allowed
	for _, filename := range legitimateFiles {
		t.Run(filename, func(t *testing.T) {
			srcPath := filepath.Join(srcDir, filename)
			err := fm.validatePathSecurity(srcPath, "copy")
			assert.NoError(t, err, "Legitimate file %s should be allowed", filename)
		})
	}

	// Test copying all legitimate files
	err = fm.CopyFiles([]string{"*"}, srcDir, dstDir, nil)
	assert.NoError(t, err, "Should successfully copy all legitimate files")

	// Verify all files were copied
	for _, filename := range legitimateFiles {
		dstPath := filepath.Join(dstDir, filename)
		assert.FileExists(t, dstPath, "File %s should be copied", filename)
	}
}

// TestFileManager_RaceConditionResistance tests resistance to race conditions
func TestFileManager_RaceConditionResistance(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wtree-race-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(dstDir, 0755))

	// Create a regular file
	regularFile := filepath.Join(srcDir, "regular.txt")
	require.NoError(t, os.WriteFile(regularFile, []byte("content"), 0644))

	fm := NewFileManager(false)
	err = fm.SetBasePath(srcDir)
	require.NoError(t, err)

	// Validate the file (should pass)
	err = fm.validatePathSecurity(regularFile, "copy")
	assert.NoError(t, err, "Regular file should pass validation")

	// Replace file with symlink (simulating TOCTOU attack)
	require.NoError(t, os.Remove(regularFile))
	outsideFile := filepath.Join(tmpDir, "outside.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("malicious"), 0644))
	require.NoError(t, os.Symlink(outsideFile, regularFile))

	// Re-validate (should now fail since it's a symlink)
	err = fm.validatePathSecurity(regularFile, "copy")
	assert.Error(t, err, "Should detect file was replaced with malicious symlink")
}

// BenchmarkSecurityValidation benchmarks the security validation performance
func BenchmarkSecurityValidation(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wtree-benchmark")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(b, os.MkdirAll(srcDir, 0755))

	testFile := filepath.Join(srcDir, "test.txt")
	require.NoError(b, os.WriteFile(testFile, []byte("content"), 0644))

	fm := NewFileManager(false)
	err = fm.SetBasePath(srcDir)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fm.validatePathSecurity(testFile, "benchmark")
	}
}