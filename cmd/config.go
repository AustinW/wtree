package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/awhite/wtree/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage WTree configuration",
	Long: `Manage WTree configuration files.

This command helps you create and manage both global configuration
($HOME/.config/wtree/config.yaml) and project-specific configuration
(.wtreerc) files.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize project configuration",
	Long: `Initialize a .wtreerc file in the current repository.

Creates a sample .wtreerc configuration file with common hooks and
file patterns that you can customize for your project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we're in a git repository
		if _, err := os.Stat(".git"); os.IsNotExist(err) {
			return fmt.Errorf("not in a git repository")
		}

		// Check if .wtreerc already exists
		if _, err := os.Stat(".wtreerc"); err == nil {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				return fmt.Errorf(".wtreerc already exists, use --force to overwrite")
			}
		}

		// Create sample configuration
		config := types.ProjectConfig{
			Version:         "1.0",
			WorktreePattern: "{repo}-{branch}",
			CopyFiles:       []string{".env.example"},
			LinkFiles:       []string{"node_modules", "vendor"},
			IgnoreFiles:     []string{"*.log", "*.tmp"},
			Hooks: map[types.HookEvent][]string{
				types.HookPostCreate: {
					"echo 'Worktree created: {worktree_path}'",
					"echo 'Branch: {branch}'",
				},
				types.HookPreDelete: {
					"echo 'Cleaning up worktree: {branch}'",
				},
			},
		}

		// Write to file
		data, err := yaml.Marshal(&config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := os.WriteFile(".wtreerc", data, 0644); err != nil {
			return fmt.Errorf("failed to write .wtreerc: %w", err)
		}

		fmt.Println("Created .wtreerc configuration file")
		fmt.Println("Edit this file to customize worktree behavior for your project")
		return nil
	},
}

var configGlobalCmd = &cobra.Command{
	Use:   "global",
	Short: "Initialize global configuration",
	Long: `Initialize global WTree configuration.

Creates the global configuration directory and file at
$HOME/.config/wtree/config.yaml with default settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		configDir := filepath.Join(home, ".config", "wtree")
		configFile := filepath.Join(configDir, "config.yaml")

		// Check if config already exists
		if _, err := os.Stat(configFile); err == nil {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				return fmt.Errorf("global config already exists at %s, use --force to overwrite", configFile)
			}
		}

		// Create config directory
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Create default global configuration
		config := types.DefaultWTreeConfig()

		// Write to file
		data, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := os.WriteFile(configFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		fmt.Printf("Created global configuration at: %s\n", configFile)
		fmt.Println("Edit this file to customize global WTree settings")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configGlobalCmd)

	configInitCmd.Flags().Bool("force", false, "overwrite existing .wtreerc file")
	configGlobalCmd.Flags().Bool("force", false, "overwrite existing global config file")
}