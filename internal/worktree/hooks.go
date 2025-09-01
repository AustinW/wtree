package worktree

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/awhite/wtree/pkg/types"
)

// HookExecutor handles the execution of project-defined hooks
type HookExecutor struct {
	config  *types.ProjectConfig
	timeout time.Duration
	verbose bool
}

// NewHookExecutor creates a new hook executor
func NewHookExecutor(config *types.ProjectConfig, timeout time.Duration, verbose bool) *HookExecutor {
	return &HookExecutor{
		config:  config,
		timeout: timeout,
		verbose: verbose,
	}
}

// ExecuteHooks runs all hooks for the specified event
func (he *HookExecutor) ExecuteHooks(event types.HookEvent, ctx types.HookContext) error {
	hooks := he.config.Hooks[event]
	if len(hooks) == 0 {
		return nil // No hooks defined for this event
	}

	fmt.Printf("Running %s hooks...\n", event)

	for i, hookCmd := range hooks {
		if err := he.executeHook(hookCmd, ctx, i+1, len(hooks)); err != nil {
			return fmt.Errorf("hook failed: %s: %w", hookCmd, err)
		}
	}

	return nil
}

// executeHook runs a single hook command
func (he *HookExecutor) executeHook(cmd string, ctx types.HookContext, current, total int) error {
	// Show progress
	fmt.Printf("  [%d/%d] Running: %s\n", current, total, cmd)

	// Expand command with context variables
	expandedCmd := he.expandCommand(cmd, ctx)

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(context.Background(), he.timeout)
	defer cancel()

	// Prepare command execution
	command := exec.CommandContext(execCtx, "sh", "-c", expandedCmd)
	command.Dir = ctx.WorktreePath
	command.Env = he.buildEnvironment(ctx)

	// Execute command and capture output
	output, err := command.CombinedOutput()
	
	if err != nil {
		fmt.Printf("    ✗ Hook failed: %s\n", string(output))
		return err
	}

	if he.verbose && len(output) > 0 {
		fmt.Printf("    ✓ Output: %s\n", string(output))
	} else {
		fmt.Printf("    ✓ Completed\n")
	}

	return nil
}

// expandCommand replaces placeholders in hook commands with actual values
func (he *HookExecutor) expandCommand(cmd string, ctx types.HookContext) string {
	replacements := map[string]string{
		"{repo}":          filepath.Base(ctx.RepoPath),
		"{branch}":        ctx.Branch,
		"{target_branch}": ctx.TargetBranch,
		"{worktree_path}": ctx.WorktreePath,
		"{repo_path}":     ctx.RepoPath,
	}

	expanded := cmd
	for placeholder, value := range replacements {
		expanded = strings.ReplaceAll(expanded, placeholder, value)
	}

	return expanded
}

// buildEnvironment creates the environment for hook execution
func (he *HookExecutor) buildEnvironment(ctx types.HookContext) []string {
	// Start with current environment
	env := os.Environ()

	// Add WTree-specific environment variables
	wtreeEnv := map[string]string{
		"WTREE_EVENT":        string(ctx.Event),
		"WTREE_BRANCH":       ctx.Branch,
		"WTREE_REPO_PATH":    ctx.RepoPath,
		"WTREE_WORKTREE_PATH": ctx.WorktreePath,
		"WTREE_TARGET_BRANCH": ctx.TargetBranch,
	}

	// Add WTree environment variables to env slice
	for key, value := range wtreeEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add any custom environment variables from context
	for key, value := range ctx.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

// ValidateHooks checks if all hook commands are valid
func (he *HookExecutor) ValidateHooks() error {
	for event, hooks := range he.config.Hooks {
		for _, hook := range hooks {
			if strings.TrimSpace(hook) == "" {
				return types.NewValidationError("hook-validation",
					fmt.Sprintf("empty hook command in %s", event), nil)
			}
			
			// Basic command validation - check for dangerous patterns
			if err := he.validateHookCommand(hook); err != nil {
				return types.NewValidationError("hook-validation",
					fmt.Sprintf("dangerous hook command in %s: %s", event, hook), err)
			}
		}
	}

	return nil
}

// validateHookCommand performs comprehensive security checks on hook commands
func (he *HookExecutor) validateHookCommand(cmd string) error {
	// Log the command being validated for security auditing
	log.Printf("Hook validation: Checking command: %s", cmd)

	// Normalize and clean the command for analysis
	normalizedCmd := he.normalizeCommand(cmd)
	
	// Check for dangerous patterns with comprehensive detection
	if err := he.checkDangerousPatterns(normalizedCmd); err != nil {
		log.Printf("Security violation: %v in command: %s", err, cmd)
		return err
	}

	// Check for command injection techniques
	if err := he.checkInjectionPatterns(normalizedCmd); err != nil {
		log.Printf("Security violation: %v in command: %s", err, cmd)
		return err
	}

	// Check for shell escape sequences and obfuscation
	if err := he.checkObfuscationPatterns(cmd); err != nil {
		log.Printf("Security violation: %v in command: %s", err, cmd)
		return err
	}

	return nil
}

// normalizeCommand removes comments, extra spaces, and normalizes case for analysis
func (he *HookExecutor) normalizeCommand(cmd string) string {
	// Remove shell comments (everything after unescaped #)
	var normalized strings.Builder
	inQuotes := false
	var quoteChar rune
	escaped := false

	for _, r := range cmd {
		if escaped {
			normalized.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			normalized.WriteRune(r)
			continue
		}

		if inQuotes {
			normalized.WriteRune(r)
			if r == quoteChar {
				inQuotes = false
			}
		} else {
			if r == '"' || r == '\'' || r == '`' {
				inQuotes = true
				quoteChar = r
				normalized.WriteRune(r)
			} else if r == '#' {
				// Stop at unescaped comment
				break
			} else {
				normalized.WriteRune(r)
			}
		}
	}

	// Normalize whitespace and convert to lowercase for pattern matching
	result := strings.TrimSpace(normalized.String())
	return strings.ToLower(regexp.MustCompile(`\s+`).ReplaceAllString(result, " "))
}

// checkDangerousPatterns checks for obviously dangerous command patterns
func (he *HookExecutor) checkDangerousPatterns(normalizedCmd string) error {
	dangerousPatterns := []struct {
		pattern     *regexp.Regexp
		description string
	}{
		// Match rm commands targeting root or home - simplified patterns
		{regexp.MustCompile(`\brm\s+[^;|&]*-[a-z]*r[a-z]*\s+[^;|&]*(/|~)`), "recursive delete of root or home filesystem"},
		{regexp.MustCompile(`\brm\s+[^;|&]*-[a-z]*f[a-z]*\s+[^;|&]*(/|~)`), "force delete of root or home filesystem"},
		{regexp.MustCompile(`\brm\s+[^;|&]*(/|~)\s+[^;|&]*-[a-z]*[rf][a-z]*`), "recursive delete of root or home filesystem"},
		{regexp.MustCompile(`\brm\s+[^;|&]*-[a-z]*[rf]+[a-z]*[^;|&]*\*`), "recursive delete with wildcards"},
		{regexp.MustCompile(`:\(\)\s*\{\s*:\|\:&\s*\}`), "fork bomb pattern"},
		{regexp.MustCompile(`\bdd\s+if=/dev/(zero|random|urandom)`), "dangerous dd operations"},
		{regexp.MustCompile(`\bchmod\s+777\s+/`), "dangerous permission changes on root"},
		{regexp.MustCompile(`\b(mkfs|format)(\.|[\s]+)`), "filesystem formatting commands"},
		{regexp.MustCompile(`\bmount\s.*--bind.*/(proc|sys|dev)`), "dangerous mount operations"},
		{regexp.MustCompile(`\biptables\s+-f\b`), "firewall rule flushing"},
		{regexp.MustCompile(`\b(shutdown|halt|reboot|init\s+0)\b`), "system shutdown commands"},
	}

	for _, dp := range dangerousPatterns {
		if dp.pattern.MatchString(normalizedCmd) {
			return fmt.Errorf("dangerous command pattern detected: %s", dp.description)
		}
	}

	return nil
}

// checkInjectionPatterns checks for command injection techniques
func (he *HookExecutor) checkInjectionPatterns(normalizedCmd string) error {
	injectionPatterns := []struct {
		pattern     *regexp.Regexp
		description string
	}{
		{regexp.MustCompile(`[;&|]+\s*(rm|del|format|mkfs)`), "command chaining with dangerous commands"},
		{regexp.MustCompile(`rm\$\{ifs\}`), "IFS variable exploitation with rm"},
		{regexp.MustCompile(`\$\{ifs\}`), "IFS variable exploitation"},
		{regexp.MustCompile(`\$\([^)]*rm[^)]*\)`), "command substitution with rm"},
		{regexp.MustCompile("`[^`]*rm[^`]*`"), "backtick command substitution with rm"},
		{regexp.MustCompile(`(curl|wget).*\|\s*sh`), "remote script execution"},
		{regexp.MustCompile(`[;&|]+.*curl.*\|\s*sh`), "chained remote script execution"},
		{regexp.MustCompile(`[;&|]+.*wget.*\|\s*sh`), "chained remote script execution via wget"},
		{regexp.MustCompile(`>>\s*/etc/(passwd|shadow|hosts)`), "system file modification"},
		{regexp.MustCompile(`/dev/tcp/`), "network connections via /dev/tcp"},
		{regexp.MustCompile(`nc\s+.*-e`), "netcat with command execution"},
	}

	for _, ip := range injectionPatterns {
		if ip.pattern.MatchString(normalizedCmd) {
			return fmt.Errorf("command injection pattern detected: %s", ip.description)
		}
	}

	return nil
}

// checkObfuscationPatterns checks for shell escape sequences and obfuscation
func (he *HookExecutor) checkObfuscationPatterns(cmd string) error {
	// Check for hex encoded commands
	if strings.Contains(cmd, "\\x") && len(regexp.MustCompile(`\\x[0-9a-fA-F]{2}`).FindAllString(cmd, -1)) > 5 {
		return fmt.Errorf("suspicious hex encoding detected")
	}

	// Check for excessive variable expansions
	if strings.Count(cmd, "${") > 10 {
		return fmt.Errorf("excessive variable expansion detected")
	}

	// Check for non-printable characters (excluding common whitespace)
	for _, r := range cmd {
		if !unicode.IsPrint(r) && r != '\t' && r != '\n' && r != '\r' {
			return fmt.Errorf("non-printable character detected: potential obfuscation")
		}
	}

	// Check for suspiciously long commands (likely obfuscated)
	if len(cmd) >= 1000 {
		return fmt.Errorf("command too long: potential obfuscation attempt")
	}

	// Check for excessive quote nesting (shell escape attempt)
	quoteDepth := 0
	maxDepth := 0
	for _, r := range cmd {
		if r == '"' || r == '\'' {
			quoteDepth++
			if quoteDepth > maxDepth {
				maxDepth = quoteDepth
			}
		}
	}
	if maxDepth > 6 {
		return fmt.Errorf("excessive quote nesting detected: potential shell escape")
	}

	return nil
}

// HookRunner provides a higher-level interface for running hooks with error handling
type HookRunner struct {
	executor     *HookExecutor
	allowFailure bool
}

// NewHookRunner creates a new hook runner
func NewHookRunner(config *types.ProjectConfig, timeout time.Duration, verbose, allowFailure bool) *HookRunner {
	return &HookRunner{
		executor:     NewHookExecutor(config, timeout, verbose),
		allowFailure: allowFailure,
	}
}

// RunHooks executes hooks with error handling based on configuration
func (hr *HookRunner) RunHooks(event types.HookEvent, ctx types.HookContext) error {
	err := hr.executor.ExecuteHooks(event, ctx)
	if err != nil && hr.allowFailure {
		fmt.Printf("⚠ Hook %s failed but continuing due to allow_failure: %v\n", event, err)
		return nil
	}
	return err
}

// Validate validates the hook configuration
func (hr *HookRunner) Validate() error {
	return hr.executor.ValidateHooks()
}