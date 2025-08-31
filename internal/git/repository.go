package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/awhite/wtree/pkg/types"
)

// Repository provides git operations interface
type Repository interface {
	// Repository queries
	GetCurrentBranch() (string, error)
	BranchExists(name string) bool
	IsClean() (bool, error)
	GetRepoRoot() (string, error)
	GetRepoName() string
	GetParentDir() string

	// Branch operations
	CreateBranch(name, from string) error
	DeleteBranch(name string, force bool) error
	ListBranches() ([]string, error)

	// Worktree operations
	CreateWorktree(path, branch string) error
	RemoveWorktree(path string, force bool) error
	ListWorktrees() ([]*types.WorktreeInfo, error)

	// Status operations
	GetWorktreeStatus(path string) (*WorktreeStatus, error)

	// Advanced operations
	Merge(branch string, message string) error
	Checkout(branch string) error
	Fetch(remote string, refspec ...string) error
}

// GitRepo implements Repository interface using git commands
type GitRepo struct {
	repoRoot   string
	repoName   string
	parentDir  string
	workingDir string
}

// WorktreeStatus represents the git status of a worktree
type WorktreeStatus struct {
	IsClean      bool
	ChangedFiles int
	Ahead        int
	Behind       int
}

// NewRepository creates a new git repository instance
func NewRepository(workingDir string) (Repository, error) {
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	repo := &GitRepo{workingDir: workingDir}

	// Get repository root
	root, err := repo.GetRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}
	repo.repoRoot = root
	repo.repoName = filepath.Base(root)
	repo.parentDir = filepath.Dir(root)

	return repo, nil
}

// GetRepoRoot returns the root directory of the git repository
func (r *GitRepo) GetRepoRoot() (string, error) {
	if r.repoRoot != "" {
		return r.repoRoot, nil
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = r.workingDir
	output, err := cmd.Output()
	if err != nil {
		return "", types.NewGitError("repo-root", "failed to get repository root", err)
	}

	root := strings.TrimSpace(string(output))
	return root, nil
}

// GetRepoName returns the name of the repository
func (r *GitRepo) GetRepoName() string {
	return r.repoName
}

// GetParentDir returns the parent directory of the repository
func (r *GitRepo) GetParentDir() string {
	return r.parentDir
}

// GetCurrentBranch returns the current branch name
func (r *GitRepo) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", types.NewGitError("current-branch", "failed to get current branch", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch == "HEAD" {
		return "", types.NewGitError("current-branch", "detached HEAD state", nil)
	}

	return branch, nil
}

// BranchExists checks if a branch exists
func (r *GitRepo) BranchExists(name string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	cmd.Dir = r.repoRoot
	err := cmd.Run()
	return err == nil
}

// IsClean checks if the working directory is clean
func (r *GitRepo) IsClean() (bool, error) {
	cmd := exec.Command("git", "diff-index", "--quiet", "HEAD", "--")
	cmd.Dir = r.repoRoot
	err := cmd.Run()
	if err != nil {
		// Check if it's because there are differences
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, types.NewGitError("status", "failed to check repository status", err)
	}
	return true, nil
}

// CreateBranch creates a new branch from the specified base branch
func (r *GitRepo) CreateBranch(name, from string) error {
	if r.BranchExists(name) {
		return types.NewGitError("create-branch", fmt.Sprintf("branch '%s' already exists", name), nil)
	}

	cmd := exec.Command("git", "branch", name, from)
	cmd.Dir = r.repoRoot
	if err := cmd.Run(); err != nil {
		return types.NewGitError("create-branch", 
			fmt.Sprintf("failed to create branch '%s' from '%s'", name, from), err)
	}

	return nil
}

// DeleteBranch deletes a branch
func (r *GitRepo) DeleteBranch(name string, force bool) error {
	args := []string{"branch"}
	if force {
		args = append(args, "-D")
	} else {
		args = append(args, "-d")
	}
	args = append(args, name)

	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoRoot
	if err := cmd.Run(); err != nil {
		return types.NewGitError("delete-branch", 
			fmt.Sprintf("failed to delete branch '%s'", name), err)
	}

	return nil
}

// ListBranches returns a list of all local branches
func (r *GitRepo) ListBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = r.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, types.NewGitError("list-branches", 
			"failed to list branches", err)
	}

	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if branch != "" {
			result = append(result, branch)
		}
	}

	return result, nil
}

// CreateWorktree creates a new worktree
func (r *GitRepo) CreateWorktree(path, branch string) error {
	// Ensure path doesn't exist
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return types.NewGitError("create-worktree", 
			fmt.Sprintf("path already exists: %s", path), nil)
	}

	cmd := exec.Command("git", "worktree", "add", path, branch)
	cmd.Dir = r.repoRoot
	if err := cmd.Run(); err != nil {
		return types.NewGitError("create-worktree", 
			fmt.Sprintf("failed to create worktree at '%s' for branch '%s'", path, branch), err)
	}

	return nil
}

// RemoveWorktree removes a worktree
func (r *GitRepo) RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoRoot
	if err := cmd.Run(); err != nil {
		return types.NewGitError("remove-worktree", 
			fmt.Sprintf("failed to remove worktree at '%s'", path), err)
	}

	return nil
}

// ListWorktrees returns a list of all worktrees
func (r *GitRepo) ListWorktrees() ([]*types.WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = r.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, types.NewGitError("list-worktrees", "failed to list worktrees", err)
	}

	return r.parseWorktreeList(string(output))
}

// parseWorktreeList parses the output of git worktree list --porcelain
func (r *GitRepo) parseWorktreeList(output string) ([]*types.WorktreeInfo, error) {
	var worktrees []*types.WorktreeInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var current *types.WorktreeInfo
	for _, line := range lines {
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = &types.WorktreeInfo{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if strings.HasPrefix(line, "HEAD ") && current != nil {
			// Extract commit hash if needed
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			current.Branch = branch
		} else if line == "bare" && current != nil {
			current.IsMainRepo = true
		}
	}

	if current != nil {
		worktrees = append(worktrees, current)
	}

	// Determine main repository
	for _, wt := range worktrees {
		if wt.Path == r.repoRoot {
			wt.IsMainRepo = true
			break
		}
	}

	return worktrees, nil
}

// GetWorktreeStatus returns the git status of a worktree
func (r *GitRepo) GetWorktreeStatus(path string) (*WorktreeStatus, error) {
	status := &WorktreeStatus{}

	// Check if working directory is clean
	cmd := exec.Command("git", "diff-index", "--quiet", "HEAD", "--")
	cmd.Dir = path
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			status.IsClean = false
		} else {
			return nil, types.NewGitError("worktree-status", "failed to check worktree status", err)
		}
	} else {
		status.IsClean = true
	}

	// Get number of changed files if not clean
	if !status.IsClean {
		cmd = exec.Command("git", "diff", "--name-only", "HEAD")
		cmd.Dir = path
		output, err := cmd.Output()
		if err == nil {
			status.ChangedFiles = len(strings.Split(strings.TrimSpace(string(output)), "\n"))
		}
	}

	return status, nil
}

// Merge merges a branch into the current branch
func (r *GitRepo) Merge(branch string, message string) error {
	args := []string{"merge"}
	if message != "" {
		args = append(args, "-m", message)
	}
	args = append(args, branch)

	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoRoot
	if err := cmd.Run(); err != nil {
		return types.NewGitError("merge", 
			fmt.Sprintf("failed to merge branch '%s'", branch), err)
	}

	return nil
}

// Checkout switches to a different branch
func (r *GitRepo) Checkout(branch string) error {
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = r.repoRoot
	if err := cmd.Run(); err != nil {
		return types.NewGitError("checkout", 
			fmt.Sprintf("failed to checkout branch '%s'", branch), err)
	}

	return nil
}

// Fetch fetches from remote repository
func (r *GitRepo) Fetch(remote string, refspecs ...string) error {
	args := []string{"fetch", remote}
	args = append(args, refspecs...)

	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoRoot
	if err := cmd.Run(); err != nil {
		return types.NewGitError("fetch", 
			fmt.Sprintf("failed to fetch from '%s'", remote), err)
	}

	return nil
}