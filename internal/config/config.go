package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/awhite/wtree/pkg/types"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Manager handles configuration loading and management
type Manager struct {
	globalConfig  *types.WTreeConfig
	projectConfig *types.ProjectConfig
	mu            sync.RWMutex
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{}
}

// LoadGlobalConfig loads the global WTree configuration
func (m *Manager) LoadGlobalConfig() (*types.WTreeConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.globalConfig != nil {
		return m.globalConfig, nil
	}

	// Start with default configuration
	config := types.DefaultWTreeConfig()

	// Apply configuration from viper (which handles file, env vars, flags)
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal global config: %w", err)
	}

	// Validate configuration
	if err := m.validateGlobalConfig(config); err != nil {
		return nil, fmt.Errorf("global config validation failed: %w", err)
	}

	m.globalConfig = config
	return config, nil
}

// LoadProjectConfig loads the project-specific configuration from .wtreerc
func (m *Manager) LoadProjectConfig(repoPath string) (*types.ProjectConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	configPath := filepath.Join(repoPath, ".wtreerc")

	// Return default config if no .wtreerc exists
	if !fileExists(configPath) {
		return types.DefaultProjectConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .wtreerc: %w", err)
	}

	var config types.ProjectConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse .wtreerc: %w", err)
	}

	// Apply defaults for missing fields
	if config.Version == "" {
		config.Version = "1.0"
	}
	if config.WorktreePattern == "" {
		config.WorktreePattern = "{repo}-{branch}"
	}
	if config.Hooks == nil {
		config.Hooks = make(map[types.HookEvent][]string)
	}

	// Validate configuration
	if err := m.validateProjectConfig(&config, repoPath); err != nil {
		return nil, fmt.Errorf("project config validation failed: %w", err)
	}

	m.projectConfig = &config
	return &config, nil
}

// GetGlobalConfig returns the cached global configuration
func (m *Manager) GetGlobalConfig() *types.WTreeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.globalConfig
}

// GetProjectConfig returns the cached project configuration
func (m *Manager) GetProjectConfig() *types.ProjectConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.projectConfig
}

// validateGlobalConfig validates the global configuration
func (m *Manager) validateGlobalConfig(config *types.WTreeConfig) error {
	// Validate timeouts are positive
	if config.Hooks.Timeout <= 0 {
		return types.NewValidationError("config", "hook timeout must be positive", nil)
	}
	if config.Performance.OperationTimeout <= 0 {
		return types.NewValidationError("config", "operation timeout must be positive", nil)
	}

	// Validate max parallel is reasonable
	if config.Hooks.MaxParallel <= 0 {
		config.Hooks.MaxParallel = 1
	}
	if config.Hooks.MaxParallel > 10 {
		config.Hooks.MaxParallel = 10
	}

	return nil
}

// validateProjectConfig validates the project configuration
func (m *Manager) validateProjectConfig(config *types.ProjectConfig, repoPath string) error {
	// Validate version compatibility
	if config.Version != "1.0" {
		return types.NewValidationError("config",
			fmt.Sprintf("unsupported .wtreerc version: %s", config.Version), nil)
	}

	// Validate hook commands are not empty
	for event, hooks := range config.Hooks {
		for _, hook := range hooks {
			if len(hook) == 0 {
				return types.NewValidationError("config",
					fmt.Sprintf("empty hook command in %s", event), nil)
			}
		}
	}

	// Validate file patterns using secure path validation
	allPatterns := append(config.CopyFiles, config.LinkFiles...)
	for _, pattern := range allPatterns {
		if err := m.validateFilePattern(pattern, repoPath); err != nil {
			return types.NewValidationError("config",
				fmt.Sprintf("invalid file pattern '%s': %v", pattern, err), err)
		}
	}

	return nil
}

// ResolveEditor determines which editor to use based on configuration hierarchy
func (m *Manager) ResolveEditor(globalConfig *types.WTreeConfig, projectConfig *types.ProjectConfig) string {
	// 1. Project config override
	if projectConfig != nil && projectConfig.Editor != "" {
		return projectConfig.Editor
	}

	// 2. Global config
	if globalConfig.Editor != "" {
		return globalConfig.Editor
	}

	// 3. Default
	return "cursor"
}

// ResolveTimeout determines timeout based on configuration hierarchy
func (m *Manager) ResolveTimeout(globalConfig *types.WTreeConfig, projectConfig *types.ProjectConfig) time.Duration {
	if projectConfig != nil && projectConfig.Timeout > 0 {
		return projectConfig.Timeout
	}
	return globalConfig.Hooks.Timeout
}

// ResolveAllowFailure determines allow failure setting
func (m *Manager) ResolveAllowFailure(globalConfig *types.WTreeConfig, projectConfig *types.ProjectConfig) bool {
	if projectConfig != nil {
		return projectConfig.AllowFailure
	}
	return globalConfig.Hooks.AllowFailure
}

// validateFilePattern performs comprehensive security validation of file patterns
func (m *Manager) validateFilePattern(pattern, repoPath string) error {
	// Check for absolute paths
	if filepath.IsAbs(pattern) {
		return fmt.Errorf("file patterns cannot be absolute paths")
	}

	// Check for empty pattern
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("file pattern cannot be empty")
	}

	// Clean the pattern to resolve any . or .. components
	cleanedPattern := filepath.Clean(pattern)

	// Check if cleaned pattern tries to escape the project directory
	if strings.HasPrefix(cleanedPattern, "../") || strings.Contains(cleanedPattern, "/../") || cleanedPattern == ".." {
		return fmt.Errorf("file pattern cannot escape project directory")
	}

	// Enhanced check: scan path segments for .. components before any cleaning/joining
	// This catches cases like "normal/../file.txt" that might be missed by other checks
	pathSegments := strings.Split(pattern, string(filepath.Separator))
	for _, segment := range pathSegments {
		if segment == ".." {
			return fmt.Errorf("file pattern cannot contain directory traversal (..) components")
		}
	}

	// Check for Windows-style absolute paths
	if len(pattern) >= 3 && pattern[1] == ':' && (pattern[2] == '\\' || pattern[2] == '/') {
		return fmt.Errorf("file patterns cannot be absolute paths")
	}

	// Additional validation for patterns that might contain actual path traversal
	// We need to distinguish between legitimate dots in filenames vs path traversal
	if strings.Contains(pattern, "../") || pattern == ".." || strings.HasPrefix(pattern, "../") {
		// This contains actual path traversal syntax, do full validation
		// IMPORTANT: Use original pattern, not cleaned, to detect actual traversal attempts
		testPath := filepath.Join(repoPath, pattern)

		// Get canonical (absolute, symlink-resolved) paths
		repoAbs, err := filepath.Abs(repoPath)
		if err != nil {
			return fmt.Errorf("cannot resolve repository path: %w", err)
		}

		testAbs, err := filepath.Abs(testPath)
		if err != nil {
			return fmt.Errorf("cannot resolve pattern path: %w", err)
		}

		// Ensure the resolved path is within the project directory
		repoCanonical, err := filepath.EvalSymlinks(repoAbs)
		if err != nil {
			repoCanonical = repoAbs // Fallback if symlink resolution fails
		}

		// Check if testAbs would be within the project directory
		// Use the canonical repo path for comparison
		if !strings.HasPrefix(testAbs+string(filepath.Separator), repoCanonical+string(filepath.Separator)) &&
			testAbs != repoCanonical {
			return fmt.Errorf("file pattern would resolve outside project directory")
		}
	}

	return nil
}

// utility functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
