package worktree

// CreateOptions defines options for creating worktrees
type CreateOptions struct {
	CreateBranch bool   // Create branch if it doesn't exist
	FromBranch   string // Base branch for new branch creation
	Force        bool   // Force creation even if path exists
	OpenEditor   bool   // Open in editor after creation
	DryRun       bool   // Preview what would happen without executing
}

// DeleteOptions defines options for deleting worktrees
type DeleteOptions struct {
	DeleteBranch bool // Also delete the branch
	Force        bool // Force deletion even if dirty
	IgnoreDirty  bool // Ignore uncommitted changes
	DryRun       bool // Preview what would happen without executing
}

// ListOptions defines options for listing worktrees
type ListOptions struct {
	ShowStatus   bool   // Show git status for each worktree
	BranchFilter string // Filter by branch name
	OnlyDirty    bool   // Show only worktrees with changes
}

// MergeOptions defines options for merging branches
type MergeOptions struct {
	Message string // Custom merge message
	Force   bool   // Force merge even if working directory is dirty
}

// SwitchOptions defines options for switching worktrees
type SwitchOptions struct {
	OpenEditor bool // Open in editor after switching
}

// StatusOptions defines options for showing worktree status
type StatusOptions struct {
	CurrentOnly  bool   // Show only current worktree status
	BranchFilter string // Filter by branch name
	Verbose      bool   // Show detailed git information
}

// CleanupOptions defines options for smart worktree cleanup
type CleanupOptions struct {
	DryRun     bool   // Preview what would be cleaned up
	MergedOnly bool   // Clean only merged branches
	Auto       bool   // Auto cleanup without prompts
	OlderThan  string // Clean worktrees older than this duration
	Verbose    bool   // Show detailed information
}

// InteractiveOptions defines options for interactive mode
type InteractiveOptions struct {
	CreateMode  bool // Launch in branch creation mode
	CleanupMode bool // Launch in cleanup mode
	SwitchMode  bool // Launch in switch mode
	DryRun      bool // Preview operations without executing
}

// EditorsOptions defines options for opening multiple editors
type EditorsOptions struct {
	Editors      string // Comma-separated list of editors to open
	OpenTerminal bool   // Also open a terminal in the worktree
}
