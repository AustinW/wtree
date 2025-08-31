package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/awhite/wtree/internal/git"
	"github.com/awhite/wtree/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGitRepo is a mock implementation for testing
type MockGitRepo struct {
	removedWorktrees []string
	deletedBranches  []string
	removeError      error
	deleteError      error
}

func (m *MockGitRepo) GetCurrentBranch() (string, error)                        { return "main", nil }
func (m *MockGitRepo) BranchExists(name string) bool                           { return true }
func (m *MockGitRepo) IsClean() (bool, error)                                  { return true, nil }
func (m *MockGitRepo) GetRepoRoot() (string, error)                            { return "/repo", nil }
func (m *MockGitRepo) GetRepoName() string                                     { return "test-repo" }
func (m *MockGitRepo) GetParentDir() string                                    { return "/parent" }
func (m *MockGitRepo) CreateBranch(name, from string) error                    { return nil }
func (m *MockGitRepo) CreateWorktree(path, branch string) error                { return nil }
func (m *MockGitRepo) ListWorktrees() ([]*types.WorktreeInfo, error)             { return nil, nil }
func (m *MockGitRepo) GetWorktreeStatus(path string) (*git.WorktreeStatus, error) { return nil, nil }
func (m *MockGitRepo) Merge(branch string, message string) error              { return nil }
func (m *MockGitRepo) Checkout(branch string) error                           { return nil }
func (m *MockGitRepo) Fetch(remote string, refspec ...string) error           { return nil }

func (m *MockGitRepo) RemoveWorktree(path string, force bool) error {
	if m.removeError != nil {
		return m.removeError
	}
	m.removedWorktrees = append(m.removedWorktrees, path)
	return nil
}

func (m *MockGitRepo) DeleteBranch(name string, force bool) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deletedBranches = append(m.deletedBranches, name)
	return nil
}

func (m *MockGitRepo) ListBranches() ([]string, error) {
	return []string{"main", "feature1", "feature2"}, nil
}

func TestRollbackManager_AddOperations(t *testing.T) {
	mockRepo := &MockGitRepo{}
	rm := NewRollbackManager(mockRepo)

	// Test adding different types of operations
	worktreeID := rm.AddWorktreeCleanup("/path/to/worktree")
	branchID := rm.AddBranchCleanup("test-branch")
	fileID := rm.AddFileCleanup("/path/to/files")
	linkID := rm.AddLinkCleanup([]string{"/link1", "/link2"})

	assert.True(t, rm.HasOperations())
	assert.Len(t, rm.operations, 4)

	// Test that IDs are correctly assigned
	assert.Equal(t, 0, worktreeID)
	assert.Equal(t, 1, branchID)
	assert.Equal(t, 2, fileID)
	assert.Equal(t, 3, linkID)

	descriptions := rm.GetOperations()
	assert.Contains(t, descriptions, "Remove worktree at /path/to/worktree")
	assert.Contains(t, descriptions, "Delete branch test-branch")
	assert.Contains(t, descriptions, "Remove files at /path/to/files")
	assert.Contains(t, descriptions, "Remove 2 symbolic links")
}

func TestRollbackManager_Execute(t *testing.T) {
	mockRepo := &MockGitRepo{}
	rm := NewRollbackManager(mockRepo)

	// Add operations
	rm.AddWorktreeCleanup("/worktree1")
	rm.AddBranchCleanup("branch1")
	rm.AddWorktreeCleanup("/worktree2")

	// Execute rollback
	err := rm.Execute()
	assert.NoError(t, err)

	// Check operations were executed in reverse order (LIFO)
	assert.Equal(t, []string{"/worktree2", "/worktree1"}, mockRepo.removedWorktrees)
	assert.Equal(t, []string{"branch1"}, mockRepo.deletedBranches)

	// Operations should be cleared after execution
	assert.False(t, rm.HasOperations())
}

func TestRollbackManager_FileCleanup(t *testing.T) {
	// Create temp files for testing
	tmpDir, err := os.MkdirTemp("", "rollback-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	mockRepo := &MockGitRepo{}
	rm := NewRollbackManager(mockRepo)

	rm.AddFileCleanup(testFile)
	
	// File should exist before rollback
	assert.FileExists(t, testFile)

	// Execute rollback
	err = rm.Execute()
	assert.NoError(t, err)

	// File should be removed after rollback
	assert.NoFileExists(t, testFile)
}

func TestRollbackManager_Clear(t *testing.T) {
	mockRepo := &MockGitRepo{}
	rm := NewRollbackManager(mockRepo)

	rm.AddWorktreeCleanup("/path")
	rm.AddBranchCleanup("branch")

	assert.True(t, rm.HasOperations())

	rm.Clear()

	assert.False(t, rm.HasOperations())
	assert.Empty(t, rm.GetOperations())
}

func TestRollbackManager_ExecuteWithErrors(t *testing.T) {
	mockRepo := &MockGitRepo{
		removeError: assert.AnError,
	}
	rm := NewRollbackManager(mockRepo)

	rm.AddWorktreeCleanup("/worktree")
	rm.AddBranchCleanup("branch")

	// Should continue with other operations even if one fails
	err := rm.Execute()
	assert.Error(t, err)

	// Branch should still be deleted even though worktree removal failed
	assert.Equal(t, []string{"branch"}, mockRepo.deletedBranches)
}

func TestRollbackManager_DependencyTracking(t *testing.T) {
	mockRepo := &MockGitRepo{}
	rm := NewRollbackManager(mockRepo)

	// Add operations with dependencies
	worktreeID := rm.AddWorktreeCleanup("/worktree")
	branchID := rm.AddBranchCleanup("branch")
	fileID := rm.AddFileCleanup("/files")

	// Branch cleanup depends on worktree cleanup
	rm.AddDependency(branchID, worktreeID)
	// File cleanup depends on worktree cleanup
	rm.AddDependency(fileID, worktreeID)

	assert.True(t, rm.HasOperations())
	assert.Len(t, rm.operations, 3)

	// Execute rollback
	err := rm.Execute()
	assert.NoError(t, err)

	// All operations should be executed
	assert.Equal(t, []string{"/worktree"}, mockRepo.removedWorktrees)
	assert.Equal(t, []string{"branch"}, mockRepo.deletedBranches)
}

func TestRollbackManager_CriticalFailure(t *testing.T) {
	mockRepo := &MockGitRepo{
		removeError: assert.AnError, // Critical worktree operation will fail
	}
	rm := NewRollbackManager(mockRepo)
	rm.SetFailFast(true)

	// Add critical operation that will fail
	rm.AddWorktreeCleanup("/worktree")
	rm.AddBranchCleanup("branch")

	// Should stop immediately on critical failure
	err := rm.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "critical rollback operation failed")

	// Branch should NOT be deleted due to early termination
	assert.Empty(t, mockRepo.deletedBranches)

	// Should have recorded the last error
	lastErr := rm.GetLastError()
	assert.Error(t, lastErr)
}

func TestRollbackManager_NonCriticalFailure(t *testing.T) {
	mockRepo := &MockGitRepo{
		deleteError: assert.AnError, // Non-critical branch operation will fail
	}
	rm := NewRollbackManager(mockRepo)
	rm.SetFailFast(true)

	// Add operations - branch cleanup is non-critical
	rm.AddWorktreeCleanup("/worktree")
	rm.AddBranchCleanup("branch")

	// Should continue even when non-critical operation fails
	err := rm.Execute()
	assert.Error(t, err) // Should still report the error
	assert.NotContains(t, err.Error(), "critical rollback operation failed")

	// Worktree should be deleted despite branch failure
	assert.Equal(t, []string{"/worktree"}, mockRepo.removedWorktrees)
}

func TestRollbackManager_DependencySkipping(t *testing.T) {
	mockRepo := &MockGitRepo{
		removeError: assert.AnError, // Worktree removal will fail
	}
	rm := NewRollbackManager(mockRepo)
	rm.SetFailFast(false) // Don't fail fast so we can test dependency skipping

	// Add operations with dependencies
	worktreeID := rm.AddWorktreeCleanup("/worktree")
	branchID := rm.AddBranchCleanup("branch")
	fileID := rm.AddFileCleanup("/files")

	// Branch and file cleanup depend on worktree cleanup
	rm.AddDependency(branchID, worktreeID)
	rm.AddDependency(fileID, worktreeID)

	// Execute rollback
	err := rm.Execute()
	assert.Error(t, err)

	// Worktree removal should have been attempted and failed
	assert.Empty(t, mockRepo.removedWorktrees)
	
	// Dependent operations should have been skipped
	assert.Empty(t, mockRepo.deletedBranches)
}

func TestRollbackManager_ConcurrentAccess(t *testing.T) {
	mockRepo := &MockGitRepo{}
	rm := NewRollbackManager(mockRepo)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Add operations concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rm.AddFileCleanup(fmt.Sprintf("/file-%d", id))
		}(i)
	}

	wg.Wait()

	// Should have all operations
	assert.Len(t, rm.operations, numGoroutines)
	assert.True(t, rm.HasOperations())

	// Execute should work correctly
	err := rm.Execute()
	assert.NoError(t, err)
	assert.False(t, rm.HasOperations())
}

func TestRollbackManager_FailFastToggle(t *testing.T) {
	mockRepo := &MockGitRepo{}
	rm := NewRollbackManager(mockRepo)

	// Default should be fail fast
	assert.True(t, rm.failFast)

	// Should be able to toggle
	rm.SetFailFast(false)
	assert.False(t, rm.failFast)

	rm.SetFailFast(true)
	assert.True(t, rm.failFast)
}