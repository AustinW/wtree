package github

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/awhite/wtree/pkg/types"
)

// Client handles GitHub CLI integration
type Client struct {
	cliCommand string
	timeout    time.Duration
}

var (
	// allowedCommands is a whitelist of permitted GitHub CLI commands
	allowedCommands = map[string]bool{
		"gh":           true,
		"/usr/bin/gh":  true,
		"/usr/local/bin/gh": true,
		"hub":          true, // Alternative GitHub CLI
		"/usr/bin/hub": true,
		"/usr/local/bin/hub": true,
	}

	// validCommandPattern ensures command is a simple executable name or absolute path
	validCommandPattern = regexp.MustCompile(`^(/usr/local/bin/|/usr/bin/)?[a-zA-Z0-9_-]+$`)
)

// PRInfo represents information about a GitHub PR
type PRInfo struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	HeadRef     string    `json:"headRefName"`
	BaseRef     string    `json:"baseRefName"`
	State       string    `json:"state"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsDraft     bool      `json:"isDraft"`
	Mergeable   string    `json:"mergeable"`
	HeadSha     string    `json:"headRefOid"`
	Repository  string    `json:"repository"`
}

// validateCLICommand validates the GitHub CLI command for security
func validateCLICommand(cliCommand string) error {
	// Empty command defaults to "gh", which is safe
	if cliCommand == "" {
		return nil
	}

	// Check pattern to prevent injection attacks
	if !validCommandPattern.MatchString(cliCommand) {
		log.Printf("Security violation: Invalid CLI command pattern detected: %s", cliCommand)
		return types.NewValidationError("cli-command", 
			"Invalid CLI command format. Only simple executable names and standard paths are allowed", nil)
	}

	// Check against whitelist
	if !allowedCommands[cliCommand] {
		// If it's an absolute path, check if the basename is allowed
		if filepath.IsAbs(cliCommand) {
			baseName := filepath.Base(cliCommand)
			if !allowedCommands[baseName] {
				log.Printf("Security violation: Unauthorized CLI command attempted: %s", cliCommand)
				return types.NewValidationError("cli-command", 
					"CLI command not in allowlist. Only 'gh' and 'hub' are permitted GitHub CLI tools", nil)
			}
		} else {
			log.Printf("Security violation: Unauthorized CLI command attempted: %s", cliCommand)
			return types.NewValidationError("cli-command", 
				"CLI command not in allowlist. Only 'gh' and 'hub' are permitted GitHub CLI tools", nil)
		}
	}

	return nil
}

// NewClient creates a new GitHub client
func NewClient(cliCommand string, timeout time.Duration) *Client {
	if cliCommand == "" {
		cliCommand = "gh"
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Validate the CLI command for security
	if err := validateCLICommand(cliCommand); err != nil {
		log.Printf("Security error creating GitHub client: %v", err)
		// Use safe default instead of failing completely
		cliCommand = "gh"
	}
	
	return &Client{
		cliCommand: cliCommand,
		timeout:    timeout,
	}
}

// IsAvailable checks if the GitHub CLI is available and authenticated
func (c *Client) IsAvailable() error {
	// Re-validate CLI command for additional security
	if err := validateCLICommand(c.cliCommand); err != nil {
		log.Printf("Security validation failed for CLI command: %v", err)
		return types.NewConfigError("github-cli-security", 
			"GitHub CLI command failed security validation", err)
	}

	// Check if gh command exists
	cmd := exec.Command("which", c.cliCommand)
	if err := cmd.Run(); err != nil {
		return types.NewConfigError("github-cli", "GitHub CLI not found in PATH", err)
	}

	// Check if user is authenticated
	cmd = exec.Command(c.cliCommand, "auth", "status")
	if err := cmd.Run(); err != nil {
		return types.NewConfigError("github-auth", 
			"GitHub CLI not authenticated. Run 'gh auth login' first", err)
	}

	return nil
}

// GetPR fetches information about a specific PR
func (c *Client) GetPR(prNumber int) (*PRInfo, error) {
	if prNumber <= 0 {
		return nil, types.NewValidationError("pr-number", "PR number must be positive", nil)
	}

	// Use gh pr view to get PR information in JSON format
	cmd := exec.Command(c.cliCommand, "pr", "view", strconv.Itoa(prNumber), "--json",
		"number,title,author,headRefName,baseRefName,state,url,createdAt,updatedAt,isDraft,mergeable,headRefOid")
	
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "not found") {
				return nil, types.NewValidationError("pr-not-found", 
					fmt.Sprintf("PR #%d not found in this repository", prNumber), nil)
			}
		}
		return nil, types.NewGitError("github-pr-fetch", 
			fmt.Sprintf("failed to fetch PR #%d", prNumber), err)
	}

	var prData struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Author      struct {
			Login string `json:"login"`
		} `json:"author"`
		HeadRefName string    `json:"headRefName"`
		BaseRefName string    `json:"baseRefName"`
		State       string    `json:"state"`
		URL         string    `json:"url"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
		IsDraft     bool      `json:"isDraft"`
		Mergeable   string    `json:"mergeable"`
		HeadRefOid  string    `json:"headRefOid"`
	}

	if err := json.Unmarshal(output, &prData); err != nil {
		return nil, types.NewConfigError("github-json-parse", "failed to parse GitHub response", err)
	}

	// Get repository name
	repoName, err := c.getRepositoryName()
	if err != nil {
		return nil, err
	}

	prInfo := &PRInfo{
		Number:     prData.Number,
		Title:      prData.Title,
		Author:     prData.Author.Login,
		HeadRef:    prData.HeadRefName,
		BaseRef:    prData.BaseRefName,
		State:      prData.State,
		URL:        prData.URL,
		CreatedAt:  prData.CreatedAt,
		UpdatedAt:  prData.UpdatedAt,
		IsDraft:    prData.IsDraft,
		Mergeable:  prData.Mergeable,
		HeadSha:    prData.HeadRefOid,
		Repository: repoName,
	}

	return prInfo, nil
}

// ListPRs lists all open PRs in the repository
func (c *Client) ListPRs(state string) ([]*PRInfo, error) {
	if state == "" {
		state = "open"
	}

	cmd := exec.Command(c.cliCommand, "pr", "list", "--state", state, "--json",
		"number,title,author,headRefName,baseRefName,state,url,createdAt,updatedAt,isDraft,mergeable,headRefOid")
	
	output, err := cmd.Output()
	if err != nil {
		return nil, types.NewGitError("github-pr-list", "failed to list PRs", err)
	}

	var prDataList []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Author      struct {
			Login string `json:"login"`
		} `json:"author"`
		HeadRefName string    `json:"headRefName"`
		BaseRefName string    `json:"baseRefName"`
		State       string    `json:"state"`
		URL         string    `json:"url"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
		IsDraft     bool      `json:"isDraft"`
		Mergeable   string    `json:"mergeable"`
		HeadRefOid  string    `json:"headRefOid"`
	}

	if err := json.Unmarshal(output, &prDataList); err != nil {
		return nil, types.NewConfigError("github-json-parse", "failed to parse GitHub response", err)
	}

	// Get repository name
	repoName, err := c.getRepositoryName()
	if err != nil {
		return nil, err
	}

	prInfos := make([]*PRInfo, len(prDataList))
	for i, prData := range prDataList {
		prInfos[i] = &PRInfo{
			Number:     prData.Number,
			Title:      prData.Title,
			Author:     prData.Author.Login,
			HeadRef:    prData.HeadRefName,
			BaseRef:    prData.BaseRefName,
			State:      prData.State,
			URL:        prData.URL,
			CreatedAt:  prData.CreatedAt,
			UpdatedAt:  prData.UpdatedAt,
			IsDraft:    prData.IsDraft,
			Mergeable:  prData.Mergeable,
			HeadSha:    prData.HeadRefOid,
			Repository: repoName,
		}
	}

	return prInfos, nil
}

// CheckoutPR checks out the PR branch locally
func (c *Client) CheckoutPR(prNumber int) (string, error) {
	cmd := exec.Command(c.cliCommand, "pr", "checkout", strconv.Itoa(prNumber))
	
	output, err := cmd.Output()
	if err != nil {
		return "", types.NewGitError("github-pr-checkout", 
			fmt.Sprintf("failed to checkout PR #%d", prNumber), err)
	}

	// Extract branch name from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Switched to branch") || strings.Contains(line, "Already on") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				branchName := strings.Trim(parts[len(parts)-1], "'\"")
				return branchName, nil
			}
		}
	}

	// Fallback: try to get the branch name from PR info
	prInfo, err := c.GetPR(prNumber)
	if err != nil {
		return "", fmt.Errorf("failed to get PR info after checkout: %w", err)
	}

	return prInfo.HeadRef, nil
}

// getRepositoryName gets the current repository name from GitHub
func (c *Client) getRepositoryName() (string, error) {
	cmd := exec.Command(c.cliCommand, "repo", "view", "--json", "name")
	
	output, err := cmd.Output()
	if err != nil {
		return "", types.NewGitError("github-repo-info", "failed to get repository info", err)
	}

	var repoData struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(output, &repoData); err != nil {
		return "", types.NewConfigError("github-json-parse", "failed to parse repository response", err)
	}

	return repoData.Name, nil
}

// ValidatePRState checks if PR is in a suitable state for worktree creation
func (c *Client) ValidatePRState(prInfo *PRInfo) error {
	if prInfo.State != "open" {
		return types.NewValidationError("pr-state", 
			fmt.Sprintf("PR #%d is %s, only open PRs can be checked out", prInfo.Number, prInfo.State), nil)
	}

	if prInfo.IsDraft {
		// Allow draft PRs but warn the user
		// This is just a validation function, warning should be handled by the caller
	}

	return nil
}