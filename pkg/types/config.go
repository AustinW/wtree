package types

import "time"

// WTreeConfig represents the global WTree tool configuration
type WTreeConfig struct {
	// Editor preferences
	Editor string `yaml:"editor" mapstructure:"editor"`

	// UI settings
	UI UIConfig `yaml:"ui" mapstructure:"ui"`

	// GitHub settings
	GitHub GitHubConfig `yaml:"github" mapstructure:"github"`

	// Hook execution settings
	Hooks HookConfig `yaml:"hooks" mapstructure:"hooks"`

	// Path settings
	Paths PathConfig `yaml:"paths" mapstructure:"paths"`

	// Performance settings
	Performance PerformanceConfig `yaml:"performance" mapstructure:"performance"`
}

// UIConfig represents UI/output configuration
type UIConfig struct {
	Colors             bool `yaml:"colors" mapstructure:"colors"`
	ProgressBars       bool `yaml:"progress_bars" mapstructure:"progress_bars"`
	Verbose            bool `yaml:"verbose" mapstructure:"verbose"`
	ConfirmDestructive bool `yaml:"confirm_destructive" mapstructure:"confirm_destructive"`
}

// GitHubConfig represents GitHub integration configuration
type GitHubConfig struct {
	CLICommand   string        `yaml:"cli_command" mapstructure:"cli_command"`
	CacheTimeout time.Duration `yaml:"cache_timeout" mapstructure:"cache_timeout"`
}

// HookConfig represents hook execution configuration
type HookConfig struct {
	Timeout      time.Duration `yaml:"timeout" mapstructure:"timeout"`
	AllowFailure bool          `yaml:"allow_failure" mapstructure:"allow_failure"`
	MaxParallel  int           `yaml:"max_parallel" mapstructure:"max_parallel"`
}

// PathConfig represents path configuration
type PathConfig struct {
	WorktreeParent string `yaml:"worktree_parent" mapstructure:"worktree_parent"`
}

// PerformanceConfig represents performance settings
type PerformanceConfig struct {
	MaxConcurrentOps int           `yaml:"max_concurrent_operations" mapstructure:"max_concurrent_operations"`
	OperationTimeout time.Duration `yaml:"operation_timeout" mapstructure:"operation_timeout"`
}

// DefaultWTreeConfig returns the default configuration
func DefaultWTreeConfig() *WTreeConfig {
	return &WTreeConfig{
		Editor: "cursor",
		UI: UIConfig{
			Colors:             true,
			ProgressBars:       true,
			Verbose:            false,
			ConfirmDestructive: true,
		},
		GitHub: GitHubConfig{
			CLICommand:   "gh",
			CacheTimeout: 5 * time.Minute,
		},
		Hooks: HookConfig{
			Timeout:      5 * time.Minute,
			AllowFailure: false,
			MaxParallel:  3,
		},
		Paths: PathConfig{
			WorktreeParent: "", // Auto-detect
		},
		Performance: PerformanceConfig{
			MaxConcurrentOps: 3,
			OperationTimeout: 10 * time.Minute,
		},
	}
}

// HookEvent represents the different hook events
type HookEvent string

const (
	HookPreCreate  HookEvent = "pre_create"
	HookPostCreate HookEvent = "post_create"
	HookPreDelete  HookEvent = "pre_delete"
	HookPostDelete HookEvent = "post_delete"
	HookPreMerge   HookEvent = "pre_merge"
	HookPostMerge  HookEvent = "post_merge"
)

// ProjectConfig represents project-specific configuration from .wtreerc
type ProjectConfig struct {
	Version string `yaml:"version" mapstructure:"version"`

	// Hook definitions (project-specific commands)
	Hooks map[HookEvent][]string `yaml:"hooks" mapstructure:"hooks"`

	// File operations
	CopyFiles   []string `yaml:"copy_files" mapstructure:"copy_files"`
	LinkFiles   []string `yaml:"link_files" mapstructure:"link_files"`
	IgnoreFiles []string `yaml:"ignore_files" mapstructure:"ignore_files"`

	// Naming and behavior overrides
	WorktreePattern string `yaml:"worktree_pattern" mapstructure:"worktree_pattern"`
	Editor          string `yaml:"editor" mapstructure:"editor"`

	// Execution settings (overrides global)
	Timeout      time.Duration `yaml:"timeout" mapstructure:"timeout"`
	AllowFailure bool          `yaml:"allow_failure" mapstructure:"allow_failure"`
	Verbose      bool          `yaml:"verbose" mapstructure:"verbose"`
}

// DefaultProjectConfig returns the default project configuration
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Version:         "1.0",
		Hooks:           make(map[HookEvent][]string),
		WorktreePattern: "{repo}-{branch}",
		CopyFiles:       []string{},
		LinkFiles:       []string{},
		IgnoreFiles:     []string{},
	}
}

// HookContext provides context information to hooks
type HookContext struct {
	Event        HookEvent
	WorktreePath string
	RepoPath     string
	Branch       string
	TargetBranch string
	Environment  map[string]string
}

// WorktreeInfo represents information about a worktree
type WorktreeInfo struct {
	Path       string
	Branch     string
	IsMainRepo bool
	IsClean    bool
	Ahead      int
	Behind     int
}

// WorktreeStatus represents the status of a worktree for display
type WorktreeStatus struct {
	Branch       string
	Path         string
	Type         string // "branch", "pr"
	Status       string // "clean", "dirty", "ahead", "behind"
	ChangedFiles int
	Ahead        int
	Behind       int
	PRNumber     int
	LastCommit   time.Time
}
