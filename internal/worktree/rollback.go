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
	failFastExplicitlySet bool // Track if SetFailFast was explicitly called
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

	// In fail-fast mode, execute critical operations first, then non-critical  
	if rm.failFast {
		// Phase 1: Execute critical operations first in reverse order
		for i := len(rm.operations) - 1; i >= 0; i-- {
			op := rm.operations[i]
			if !op.Critical {
				continue
			}

			// Check if this operation should be skipped due to dependencies
			if rm.shouldSkipOperation(op, failed) {
				continue
			}

			// Execute the critical operation
			if err := op.Action(); err != nil {
				opError := fmt.Errorf("%s: %w", op.Description, err)
				rm.lastError = opError
				
				// If SetFailFast was explicitly called, fail immediately and don't execute non-critical operations
				if rm.failFastExplicitlySet {
					rm.clearOperations()
					return types.NewFileSystemError("rollback-critical-failure", "",
						fmt.Sprintf("critical rollback operation failed: %s", op.Description), err)
				} else {
					// Default behavior: continue to non-critical operations even if critical ones fail
					failed[op.ID] = true
					errors = append(errors, opError)
				}
			} else {
				executed[op.ID] = true
			}
		}

		// Phase 2: Execute non-critical operations in reverse order (only if not in strict fail-fast mode or no critical failures)
		if !rm.failFastExplicitlySet || len(errors) == 0 {
			for i := len(rm.operations) - 1; i >= 0; i-- {
				op := rm.operations[i]
				if op.Critical || executed[op.ID] {
					continue
				}

				// Check if this operation should be skipped due to dependencies
				if rm.shouldSkipOperation(op, failed) {
					continue
				}

				// Execute the non-critical operation
				if err := op.Action(); err != nil {
					opError := fmt.Errorf("%s: %w", op.Description, err)
					errors = append(errors, opError)
					failed[op.ID] = true
					rm.lastError = opError
				} else {
					executed[op.ID] = true
				}
			}
		}
	} else {
		// Execute operations in dependency-aware order
		remaining := make([]int, 0, len(rm.operations))
		for i := range rm.operations {
			remaining = append(remaining, i)
		}

		// Keep executing operations until none remain
		for len(remaining) > 0 {
			progress := false
			newRemaining := make([]int, 0)

			for _, i := range remaining {
				op := rm.operations[i]

				// Check if this operation should be skipped due to dependencies
				if rm.shouldSkipOperation(op, failed) {
					continue // Skip this operation permanently
				}

				// Check if all dependencies have been executed successfully
				canExecute := true
				for _, depID := range op.DependsOn {
					if !executed[depID] && !failed[depID] {
						canExecute = false
						break
					}
				}

				if canExecute {
					// Execute the operation
					if err := op.Action(); err != nil {
						opError := fmt.Errorf("%s: %w", op.Description, err)
						errors = append(errors, opError)
						failed[op.ID] = true
						rm.lastError = opError
					} else {
						executed[op.ID] = true
					}
					progress = true
				} else {
					// Can't execute yet, keep for next iteration
					newRemaining = append(newRemaining, i)
				}
			}

			remaining = newRemaining

			// If no progress was made and operations remain, we have a dependency cycle or unreachable operations
			if !progress && len(remaining) > 0 {
				for _, i := range remaining {
					op := rm.operations[i]
					opError := fmt.Errorf("operation cannot be executed due to dependency constraints: %s", op.Description)
					errors = append(errors, opError)
					failed[op.ID] = true
					rm.lastError = opError
				}
				break
			}
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
	// Don't clear lastError - it should persist after operations are cleared
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
	rm.failFastExplicitlySet = true
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