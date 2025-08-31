package worktree

import (
	"fmt"
	"os"
	"sync"

	"github.com/awhite/wtree/internal/git"
	"github.com/awhite/wtree/pkg/types"
)

// RollbackManager handles rollback operations when commands fail
type RollbackManager struct {
	repo git.Repository
	operations []RollbackOperation
	dependencies map[int][]int // operation index -> list of dependent operation indices
	mu sync.RWMutex
	failFast bool
	lastError error
}

// RollbackOperation represents a single operation that can be rolled back
type RollbackOperation struct {
	Type        RollbackType
	Description string
	Action      func() error
	Critical    bool // If true, failure of this operation stops the rollback
	ID          int  // Unique ID for dependency tracking
	DependsOn   []int // IDs of operations this depends on
}

// RollbackType defines the type of rollback operation
type RollbackType string

const (
	RollbackRemoveWorktree RollbackType = "remove_worktree"
	RollbackDeleteBranch   RollbackType = "delete_branch"
	RollbackRemoveFiles    RollbackType = "remove_files"
	RollbackCleanupLinks   RollbackType = "cleanup_links"
)

// NewRollbackManager creates a new rollback manager
func NewRollbackManager(repo git.Repository) *RollbackManager {
	return &RollbackManager{
		repo:         repo,
		operations:   make([]RollbackOperation, 0),
		dependencies: make(map[int][]int),
		failFast:     true, // Stop on critical failures by default
	}
}

// AddWorktreeCleanup adds worktree removal to rollback operations
func (rm *RollbackManager) AddWorktreeCleanup(path string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := len(rm.operations)
	op := RollbackOperation{
		Type:        RollbackRemoveWorktree,
		Description: fmt.Sprintf("Remove worktree at %s", path),
		Action: func() error {
			return rm.repo.RemoveWorktree(path, true) // force removal
		},
		Critical:    true, // Worktree cleanup is critical
		ID:          id,
	}
	rm.operations = append(rm.operations, op)
	return id
}

// AddBranchCleanup adds branch deletion to rollback operations
func (rm *RollbackManager) AddBranchCleanup(branch string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := len(rm.operations)
	op := RollbackOperation{
		Type:        RollbackDeleteBranch,
		Description: fmt.Sprintf("Delete branch %s", branch),
		Action: func() error {
			return rm.repo.DeleteBranch(branch, true) // force deletion
		},
		Critical:    false, // Branch cleanup is not critical
		ID:          id,
	}
	rm.operations = append(rm.operations, op)
	return id
}

// AddFileCleanup adds file/directory removal to rollback operations
func (rm *RollbackManager) AddFileCleanup(path string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := len(rm.operations)
	op := RollbackOperation{
		Type:        RollbackRemoveFiles,
		Description: fmt.Sprintf("Remove files at %s", path),
		Action: func() error {
			return os.RemoveAll(path)
		},
		Critical:    true, // File cleanup is critical
		ID:          id,
	}
	rm.operations = append(rm.operations, op)
	return id
}

// AddLinkCleanup adds symlink cleanup to rollback operations
func (rm *RollbackManager) AddLinkCleanup(links []string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := len(rm.operations)
	op := RollbackOperation{
		Type:        RollbackCleanupLinks,
		Description: fmt.Sprintf("Remove %d symbolic links", len(links)),
		Action: func() error {
			for _, link := range links {
				if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		},
		Critical:    false, // Link cleanup is not critical
		ID:          id,
	}
	rm.operations = append(rm.operations, op)
	return id
}

// Execute performs all rollback operations with dependency tracking and early failure handling
func (rm *RollbackManager) Execute() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if len(rm.operations) == 0 {
		return nil
	}

	var errors []error
	executed := make(map[int]bool)
	failed := make(map[int]bool)

	// Execute operations in reverse order (LIFO) with dependency checking
	for i := len(rm.operations) - 1; i >= 0; i-- {
		op := rm.operations[i]

		// Check if this operation should be skipped due to dependencies
		if rm.shouldSkipOperation(op, failed) {
			continue
		}

		// Execute the operation
		if err := op.Action(); err != nil {
			opError := fmt.Errorf("%s: %w", op.Description, err)
			errors = append(errors, opError)
			failed[op.ID] = true
			rm.lastError = opError

			// If this is a critical operation and we're in fail-fast mode, stop
			if op.Critical && rm.failFast {
				rm.clearOperations()
				return types.NewFileSystemError("rollback-critical-failure", "",
					fmt.Sprintf("critical rollback operation failed: %s", op.Description), err)
			}
		} else {
			executed[op.ID] = true
		}
	}

	// Clear operations after rollback attempt
	rm.clearOperations()

	if len(errors) > 0 {
		return types.NewFileSystemError("rollback", "",
			fmt.Sprintf("rollback completed with %d errors", len(errors)), 
			fmt.Errorf("errors: %v", errors))
	}

	return nil
}

// Clear removes all pending rollback operations
func (rm *RollbackManager) Clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.clearOperations()
}

// clearOperations clears operations without acquiring lock (internal use)
func (rm *RollbackManager) clearOperations() {
	rm.operations = make([]RollbackOperation, 0)
	rm.dependencies = make(map[int][]int)
	rm.lastError = nil
}

// HasOperations returns true if there are rollback operations pending
func (rm *RollbackManager) HasOperations() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.operations) > 0
}

// GetOperations returns a description of all pending operations
func (rm *RollbackManager) GetOperations() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	descriptions := make([]string, len(rm.operations))
	for i, op := range rm.operations {
		descriptions[i] = op.Description
	}
	return descriptions
}

// AddDependency adds a dependency relationship between operations
func (rm *RollbackManager) AddDependency(dependentID, dependsOnID int) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if dependentID < len(rm.operations) {
		rm.operations[dependentID].DependsOn = append(rm.operations[dependentID].DependsOn, dependsOnID)
		
		// Update reverse dependency map for quick lookups
		if rm.dependencies[dependsOnID] == nil {
			rm.dependencies[dependsOnID] = make([]int, 0)
		}
		rm.dependencies[dependsOnID] = append(rm.dependencies[dependsOnID], dependentID)
	}
}

// SetFailFast sets whether rollback should stop on critical failures
func (rm *RollbackManager) SetFailFast(failFast bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.failFast = failFast
}

// GetLastError returns the last error encountered during rollback
func (rm *RollbackManager) GetLastError() error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.lastError
}

// shouldSkipOperation determines if an operation should be skipped due to dependency failures
func (rm *RollbackManager) shouldSkipOperation(op RollbackOperation, failed map[int]bool) bool {
	// Check if any of the operations this depends on have failed
	for _, depID := range op.DependsOn {
		if failed[depID] {
			return true
		}
	}
	return false
}