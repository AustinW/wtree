package worktree

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/awhite/wtree/pkg/types"
)

// OperationLock provides file-based locking for worktree operations
type OperationLock struct {
	lockPath    string
	lockFile    *os.File
	pid         int
	operation   string
	acquired    bool
	timeout     time.Duration
	mu          sync.Mutex
	retryDelay  time.Duration
}

// LockType represents different types of operations that can be locked
type LockType string

const (
	LockTypeCreate  LockType = "create"
	LockTypeDelete  LockType = "delete" 
	LockTypeMerge   LockType = "merge"
	LockTypeSwitch  LockType = "switch"
	LockTypeCleanup LockType = "cleanup"
)

// LockManager manages multiple operation locks
type LockManager struct {
	lockDir string
	locks   map[string]*OperationLock
	mu      sync.RWMutex
}

// NewLockManager creates a new lock manager
func NewLockManager() (*LockManager, error) {
	lockDir, err := getLockDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	return &LockManager{
		lockDir: lockDir,
		locks:   make(map[string]*OperationLock),
	}, nil
}

// AcquireLock acquires a lock for the specified operation on the target path
func (lm *LockManager) AcquireLock(lockType LockType, targetPath string, timeout time.Duration) (*OperationLock, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Create a unique lock key based on the target path and operation type
	lockKey := generateLockKey(string(lockType), targetPath)
	
	// Check if we already have this lock
	if existingLock, exists := lm.locks[lockKey]; exists && existingLock.acquired {
		return nil, types.NewValidationError("acquire-lock", 
			fmt.Sprintf("lock already acquired for %s on %s", lockType, targetPath), nil)
	}

	// Create the lock
	lock, err := newOperationLock(lm.lockDir, lockKey, string(lockType), timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock: %w", err)
	}

	// Attempt to acquire the lock
	if err := lock.acquire(); err != nil {
		_ = lock.cleanup()
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	lm.locks[lockKey] = lock
	return lock, nil
}

// ReleaseLock releases a previously acquired lock
func (lm *LockManager) ReleaseLock(lock *OperationLock) error {
	if lock == nil {
		return nil
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Remove from our tracking
	for key, trackedLock := range lm.locks {
		if trackedLock == lock {
			delete(lm.locks, key)
			break
		}
	}

	return lock.Release()
}

// ReleaseAll releases all locks held by this manager
func (lm *LockManager) ReleaseAll() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	var errors []error
	for _, lock := range lm.locks {
		if err := lock.Release(); err != nil {
			errors = append(errors, err)
		}
	}

	lm.locks = make(map[string]*OperationLock)

	if len(errors) > 0 {
		return fmt.Errorf("failed to release %d locks: %v", len(errors), errors)
	}
	return nil
}

// newOperationLock creates a new operation lock
func newOperationLock(lockDir, lockKey, operation string, timeout time.Duration) (*OperationLock, error) {
	lockPath := filepath.Join(lockDir, lockKey+".lock")
	
	return &OperationLock{
		lockPath:   lockPath,
		pid:        os.Getpid(),
		operation:  operation,
		timeout:    timeout,
		retryDelay: 100 * time.Millisecond,
	}, nil
}

// acquire attempts to acquire the lock with retry logic
func (ol *OperationLock) acquire() error {
	ol.mu.Lock()
	defer ol.mu.Unlock()

	if ol.acquired {
		return types.NewValidationError("acquire-lock", "lock already acquired", nil)
	}

	startTime := time.Now()
	for {
		// Try to create the lock file exclusively
		file, err := os.OpenFile(ol.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			// Successfully created the lock file
			ol.lockFile = file
			ol.acquired = true
			
			// Write lock information to the file
			lockInfo := fmt.Sprintf("pid=%d\noperation=%s\ntime=%s\n", 
				ol.pid, ol.operation, time.Now().Format(time.RFC3339))
			
			if _, writeErr := file.WriteString(lockInfo); writeErr != nil {
				// Clean up on write failure
				file.Close()
				os.Remove(ol.lockPath)
				return fmt.Errorf("failed to write lock info: %w", writeErr)
			}
			
			_ = file.Sync() // Force write to disk
			return nil
		}

		// Check if it's a different error than "file exists"
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create lock file: %w", err)
		}

		// Check timeout
		if time.Since(startTime) >= ol.timeout {
			// Try to provide helpful information about who owns the lock
			if lockInfo, readErr := ol.readLockInfo(); readErr == nil {
				return types.NewValidationError("acquire-lock-timeout",
					fmt.Sprintf("timeout waiting for lock (held by %s)", lockInfo), err)
			}
			return types.NewValidationError("acquire-lock-timeout", 
				"timeout waiting for lock", err)
		}

		// Check if the existing lock is stale
		if ol.isLockStale() {
			if cleanupErr := ol.cleanupStaleLock(); cleanupErr == nil {
				continue // Try again after cleanup
			}
		}

		time.Sleep(ol.retryDelay)
	}
}

// Release releases the lock
func (ol *OperationLock) Release() error {
	ol.mu.Lock()
	defer ol.mu.Unlock()

	if !ol.acquired {
		return nil // Already released or never acquired
	}

	err := ol.cleanup()
	ol.acquired = false
	return err
}

// cleanup removes the lock file and closes the file handle
func (ol *OperationLock) cleanup() error {
	var errors []error

	if ol.lockFile != nil {
		if err := ol.lockFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close lock file: %w", err))
		}
		ol.lockFile = nil
	}

	if err := os.Remove(ol.lockPath); err != nil && !os.IsNotExist(err) {
		errors = append(errors, fmt.Errorf("failed to remove lock file: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("lock cleanup errors: %v", errors)
	}
	return nil
}

// isLockStale checks if an existing lock file is stale (process no longer exists)
func (ol *OperationLock) isLockStale() bool {
	lockInfo, err := ol.readLockInfo()
	if err != nil {
		return true // Assume stale if we can't read it
	}

	// Extract PID from lock info
	if pid := extractPIDFromLockInfo(lockInfo); pid > 0 {
		// Check if process still exists
		if runtime.GOOS == "windows" {
			return !processExistsWindows(pid)
		}
		return !processExistsUnix(pid)
	}

	return true // Assume stale if no valid PID
}

// cleanupStaleLock removes a stale lock file
func (ol *OperationLock) cleanupStaleLock() error {
	return os.Remove(ol.lockPath)
}

// readLockInfo reads the information from an existing lock file
func (ol *OperationLock) readLockInfo() (string, error) {
	data, err := os.ReadFile(ol.lockPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// generateLockKey creates a unique lock key for the operation and path
func generateLockKey(operation, targetPath string) string {
	// Create a hash of the target path to handle long paths and special characters
	hash := sha256.Sum256([]byte(targetPath))
	pathHash := fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes of hash
	
	return fmt.Sprintf("wtree-%s-%s", operation, pathHash)
}

// getLockDirectory returns the directory to use for lock files
func getLockDirectory() (string, error) {
	var lockDir string
	
	if runtime.GOOS == "windows" {
		lockDir = filepath.Join(os.TempDir(), "wtree-locks")
	} else {
		lockDir = filepath.Join("/tmp", "wtree-locks")
	}

	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return "", err
	}
	
	return lockDir, nil
}

// extractPIDFromLockInfo extracts the PID from lock file content
func extractPIDFromLockInfo(lockInfo string) int {
	// Split by lines to properly parse multi-line lock info
	lines := strings.Split(lockInfo, "\n")
	for _, line := range lines {
		if len(line) > 4 && line[:4] == "pid=" {
			if pid, err := strconv.Atoi(line[4:]); err == nil {
				return pid
			}
		}
	}
	return 0
}

// processExistsUnix checks if a process exists on Unix systems
func processExistsUnix(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// processExistsWindows checks if a process exists on Windows
func processExistsWindows(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	// On Windows, FindProcess always succeeds, so we need to check differently
	state, err := process.Wait()
	if err != nil {
		return true // Process still running
	}
	
	return !state.Exited()
}