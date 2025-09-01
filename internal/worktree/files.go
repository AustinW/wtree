package worktree

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/awhite/wtree/pkg/types"
)

// FileManager handles generic file operations for worktrees
type FileManager struct {
	verbose         bool
	allowedBasePath string // Base path that operations are restricted to
}

// NewFileManager creates a new file manager
func NewFileManager(verbose bool) *FileManager {
	return &FileManager{verbose: verbose}
}

// SetBasePath sets the base directory that all file operations must be within
func (fm *FileManager) SetBasePath(basePath string) error {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve base path: %w", err)
	}

	// Resolve symlinks in the base path to get canonical path
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		log.Printf("Warning: Could not resolve symlinks in base path %s: %v", abs, err)
		canonical = abs // Use absolute path if symlink resolution fails
	}

	fm.allowedBasePath = canonical
	return nil
}

// CopyFiles copies files matching the specified patterns from source to destination
func (fm *FileManager) CopyFiles(patterns []string, srcDir, dstDir string, ignorePatterns []string) error {
	var errs []error

	for _, pattern := range patterns {
		if err := fm.copyPattern(pattern, srcDir, dstDir, ignorePatterns); err != nil {
			errs = append(errs, fmt.Errorf("copy pattern %s: %w", pattern, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("file copy errors: %v", errs)
	}

	return nil
}

// LinkFiles creates symbolic links for files matching the specified patterns
func (fm *FileManager) LinkFiles(patterns []string, srcDir, dstDir string, ignorePatterns []string) error {
	var errs []error

	for _, pattern := range patterns {
		if err := fm.linkPattern(pattern, srcDir, dstDir, ignorePatterns); err != nil {
			errs = append(errs, fmt.Errorf("link pattern %s: %w", pattern, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("file link errors: %v", errs)
	}

	return nil
}

// copyPattern copies all files matching a specific pattern
func (fm *FileManager) copyPattern(pattern, srcDir, dstDir string, ignorePatterns []string) error {
	// Get absolute pattern path
	patternPath := filepath.Join(srcDir, pattern)

	// Find all matching files
	matches, err := filepath.Glob(patternPath)
	if err != nil {
		return fmt.Errorf("invalid pattern %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		if fm.verbose {
			fmt.Printf("    No files match pattern: %s\n", pattern)
		}
		return nil
	}

	for _, srcPath := range matches {
		// Security validation: Check for symlinks and path boundaries
		if err := fm.validatePathSecurity(srcPath, "copy"); err != nil {
			log.Printf("Security violation blocked copy operation: %v", err)
			return fmt.Errorf("security check failed for %s: %w", srcPath, err)
		}

		// Calculate relative path from source directory
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			continue
		}

		// Check if file should be ignored
		if fm.shouldIgnoreFile(relPath, ignorePatterns) {
			if fm.verbose {
				fmt.Printf("    Ignoring: %s\n", relPath)
			}
			continue
		}

		dstPath := filepath.Join(dstDir, relPath)

		// Skip if source doesn't exist
		if !fileExists(srcPath) {
			continue
		}

		// Copy file or directory
		if err := fm.copyFileOrDir(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
		}

		if fm.verbose {
			fmt.Printf("    Copied: %s\n", relPath)
		}
	}

	return nil
}

// linkPattern creates symbolic links for all files matching a specific pattern
func (fm *FileManager) linkPattern(pattern, srcDir, dstDir string, ignorePatterns []string) error {
	// Get absolute pattern path
	patternPath := filepath.Join(srcDir, pattern)

	// Find all matching files
	matches, err := filepath.Glob(patternPath)
	if err != nil {
		return fmt.Errorf("invalid pattern %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		if fm.verbose {
			fmt.Printf("    No files match pattern: %s\n", pattern)
		}
		return nil
	}

	for _, srcPath := range matches {
		// Security validation: Check for symlinks and path boundaries
		if err := fm.validatePathSecurity(srcPath, "link"); err != nil {
			log.Printf("Security violation blocked link operation: %v", err)
			return fmt.Errorf("security check failed for %s: %w", srcPath, err)
		}

		// Calculate relative path from source directory
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			continue
		}

		// Check if file should be ignored
		if fm.shouldIgnoreFile(relPath, ignorePatterns) {
			if fm.verbose {
				fmt.Printf("    Ignoring: %s\n", relPath)
			}
			continue
		}

		dstPath := filepath.Join(dstDir, relPath)

		// Skip if source doesn't exist
		if !fileExists(srcPath) {
			continue
		}

		// Create destination directory if needed
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", dstPath, err)
		}

		// Create symbolic link
		if err := os.Symlink(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", dstPath, srcPath, err)
		}

		if fm.verbose {
			fmt.Printf("    Linked: %s -> %s\n", relPath, srcPath)
		}
	}

	return nil
}

// copyFileOrDir copies a file or directory recursively
func (fm *FileManager) copyFileOrDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return fm.copyDir(src, dst)
	}
	return fm.copyFile(src, dst)
}

// copyFile copies a single file with proper resource management
func (fm *FileManager) copyFile(src, dst string) error {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			log.Printf("Warning: Failed to close source file %s: %v", src, closeErr)
		}
	}()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}

	// Track if operation completed successfully
	var success bool
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil {
			log.Printf("Warning: Failed to close destination file %s: %v", dst, closeErr)
		}
		// If operation failed, clean up the partial destination file
		if !success {
			if removeErr := os.Remove(dst); removeErr != nil {
				log.Printf("Warning: Failed to remove partial destination file %s: %v", dst, removeErr)
			}
		}
	}()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Sync to ensure data is written
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Copy permissions
	if chmodErr := os.Chmod(dst, srcInfo.Mode()); chmodErr != nil {
		log.Printf("Warning: Failed to copy file permissions for %s: %v", dst, chmodErr)
		// Don't treat permission copy failure as fatal
	}

	success = true // Mark operation as successful
	log.Printf("Successfully copied file: %s -> %s", src, dst)
	return nil
}

// copyDir copies a directory recursively
func (fm *FileManager) copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := fm.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := fm.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// shouldIgnoreFile checks if a file should be ignored based on ignore patterns
func (fm *FileManager) shouldIgnoreFile(filePath string, ignorePatterns []string) bool {
	for _, pattern := range ignorePatterns {
		// Use filepath.Match for pattern matching
		matched, err := filepath.Match(pattern, filePath)
		if err != nil {
			// If pattern is invalid, skip it
			continue
		}
		if matched {
			return true
		}

		// Also check if any parent directory matches
		dir := filepath.Dir(filePath)
		for dir != "." && dir != "/" {
			matched, err := filepath.Match(pattern, dir)
			if err == nil && matched {
				return true
			}
			dir = filepath.Dir(dir)
		}
	}

	return false
}

// ValidateFilePatterns validates that file patterns are safe and don't contain dangerous sequences
func (fm *FileManager) ValidateFilePatterns(patterns []string) error {
	for _, pattern := range patterns {
		if err := fm.validatePattern(pattern); err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
	}
	return nil
}

// validatePattern validates a single file pattern
func (fm *FileManager) validatePattern(pattern string) error {
	// Check for absolute paths
	if filepath.IsAbs(pattern) {
		return types.NewValidationError("file-pattern",
			"file patterns cannot be absolute paths", nil)
	}

	// Check for path traversal attempts
	if strings.Contains(pattern, "..") {
		return types.NewValidationError("file-pattern",
			"file patterns cannot contain '..' for security", nil)
	}

	// Clean the path and check if it's the same
	cleaned := filepath.Clean(pattern)
	if cleaned != pattern && cleaned != "./"+pattern {
		return types.NewValidationError("file-pattern",
			"file pattern contains suspicious path elements", nil)
	}

	return nil
}

// Security validation functions

// validatePathSecurity performs comprehensive security checks on file paths
func (fm *FileManager) validatePathSecurity(srcPath, operation string) error {
	// Get file info to check if it's a symlink
	fileInfo, err := os.Lstat(srcPath)
	if err != nil {
		return fmt.Errorf("cannot access file %s: %w", srcPath, err)
	}

	// Check if source is a symbolic link
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		log.Printf("Security check: Found symlink at %s during %s operation", srcPath, operation)

		// Resolve the symlink target
		target, err := filepath.EvalSymlinks(srcPath)
		if err != nil {
			return fmt.Errorf("cannot resolve symlink %s: %w", srcPath, err)
		}

		// Validate that symlink target is within allowed boundaries
		if err := fm.validatePathBounds(target, "symlink target"); err != nil {
			log.Printf("Security violation: Symlink %s points outside allowed directory: %s -> %s", srcPath, srcPath, target)
			return fmt.Errorf("symlink %s points outside allowed directory: %w", srcPath, err)
		}
	}

	// Validate source path boundaries
	if err := fm.validatePathBounds(srcPath, operation); err != nil {
		return err
	}

	return nil
}

// validatePathBounds ensures a path is within the allowed base directory
func (fm *FileManager) validatePathBounds(path, operation string) error {
	if fm.allowedBasePath == "" {
		// No restrictions set
		return nil
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path for %s: %w", path, err)
	}

	// Resolve any symlinks to get canonical path
	canonicalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		log.Printf("Warning: Could not resolve symlinks in %s: %v", absPath, err)
		canonicalPath = absPath // Use absolute path if symlink resolution fails
	}

	// Check if path is within allowed base directory
	relPath, err := filepath.Rel(fm.allowedBasePath, canonicalPath)
	if err != nil {
		return fmt.Errorf("cannot compute relative path from base directory: %w", err)
	}

	// Path is outside base directory if relative path starts with ../
	if strings.HasPrefix(relPath, "../") || relPath == ".." {
		log.Printf("Security violation: %s operation attempted outside allowed directory: %s (resolved to %s)", operation, path, canonicalPath)
		return fmt.Errorf("%s operation not allowed outside base directory %s", operation, fm.allowedBasePath)
	}

	return nil
}

// utility functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
