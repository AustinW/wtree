package cmd

import (
	"fmt"
	"os"

	"github.com/awhite/wtree/internal/config"
	"github.com/awhite/wtree/internal/git"
	"github.com/awhite/wtree/internal/ui"
	"github.com/awhite/wtree/internal/worktree"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	dryRun  bool
	force   bool
)

// rootCmd represents the base command when called without any subcommands
var (
	version = "dev"
	commit  = "none"    // Set at build time
	date    = "unknown" // Set at build time
)

// Use blank identifier to indicate these are intentionally unused for now
var _ = commit
var _ = date

var rootCmd = &cobra.Command{
	Use:     "wtree",
	Short:   "Generic git worktree manager",
	Version: version,
	Long: `WTree is a generic git worktree management tool that works with any project type.

It manages git worktrees while allowing projects to define their own setup behavior
via .wtreerc configuration files.

Examples:
  wtree create feature-branch      # Create worktree for existing branch
  wtree create -b new-feature      # Create new branch and worktree
  wtree list                       # List all worktrees
  wtree delete feature-branch      # Delete worktree
  wtree switch main                # Switch to main worktree
  wtree merge feature-branch       # Merge branch into current`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/wtree/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would happen without executing")
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "skip confirmations and force operations")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".wtree" (without extension).
		configDir := home + "/.config/wtree"
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("WTREE")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

// setupManager creates and initializes the worktree manager
func setupManager() (*worktree.Manager, error) {
	// Initialize git repository
	repo, err := git.NewRepository("")
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}

	// Initialize configuration manager
	configMgr := config.NewManager()

	// Initialize UI manager
	colors := !viper.GetBool("no_color")
	uiMgr := ui.NewManager(colors, verbose)

	// Create worktree manager
	manager := worktree.NewManager(repo, configMgr, uiMgr)

	// Initialize manager (loads configs)
	if err := manager.Initialize(); err != nil {
		return nil, err
	}

	return manager, nil
}
