# GitHub Integration Architecture

## Integration Strategy

### Primary Approach: GitHub CLI Integration
We'll primarily use the GitHub CLI (`gh`) for GitHub operations rather than direct API calls because:
- User's existing authentication is automatically available
- Handles authentication, token refresh, and API rate limiting
- Consistent with user's existing workflow
- Simpler error handling and permissions

### Fallback: Direct API Integration
For advanced features or when `gh` is unavailable, we'll implement direct GitHub API integration as a fallback.

## 1. GitHub Client Architecture

### 1.1 Client Interface
```go
type GitHubClient interface {
    // Authentication
    IsAuthenticated() bool
    GetAuthenticatedUser() (*User, error)
    
    // PR Operations
    GetPR(number int) (*PRInfo, error)
    ListPRs(state PRState) ([]*PRInfo, error)
    
    // Repository Operations
    GetRepository() (*Repository, error)
    
    // Validation
    ValidateAccess() error
}

type CLIClient struct {
    command    string  // "gh"
    timeout    time.Duration
    repository string  // owner/repo format
    cache      *sync.Map
}

type APIClient struct {
    client     *github.Client
    repository string
    cache      *PRCache
}
```

### 1.2 Client Factory
```go
func NewGitHubClient(ctx context.Context) (GitHubClient, error) {
    // Try CLI client first
    if cliClient, err := NewCLIClient(); err == nil {
        if cliClient.IsAuthenticated() {
            return cliClient, nil
        }
    }
    
    // Fallback to API client
    return NewAPIClient(ctx)
}

func NewCLIClient() (*CLIClient, error) {
    // Verify gh command exists
    if _, err := exec.LookPath("gh"); err != nil {
        return nil, fmt.Errorf("github cli not found: %w", err)
    }
    
    // Get repository info
    repo, err := getCurrentRepository()
    if err != nil {
        return nil, fmt.Errorf("failed to get repository info: %w", err)
    }
    
    return &CLIClient{
        command:    "gh",
        timeout:    30 * time.Second,
        repository: repo,
        cache:      &sync.Map{},
    }, nil
}
```

## 2. PR Information Management

### 2.1 PR Data Structure
```go
type PRInfo struct {
    Number      int       `json:"number"`
    Title       string    `json:"title"`
    Body        string    `json:"body"`
    State       PRState   `json:"state"`
    
    // Branch information
    HeadRef     string    `json:"headRefName"`
    HeadSHA     string    `json:"headRefOid"`
    BaseRef     string    `json:"baseRefName"`
    BaseSHA     string    `json:"baseRefOid"`
    
    // Author information
    Author      *User     `json:"author"`
    
    // Repository information
    HeadRepo    *Repository `json:"headRepository"`
    BaseRepo    *Repository `json:"baseRepository"`
    
    // Status information
    Mergeable   *bool     `json:"mergeable"`
    Merged      bool      `json:"merged"`
    Draft       bool      `json:"isDraft"`
    
    // Timestamps
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
    MergedAt    *time.Time `json:"mergedAt"`
    
    // URLs
    URL         string    `json:"url"`
    HTMLURL     string    `json:"htmlUrl"`
}

type PRState string
const (
    PRStateOpen   PRState = "open"
    PRStateClosed PRState = "closed"
    PRStateMerged PRState = "merged"
)

type User struct {
    Login     string `json:"login"`
    Name      string `json:"name"`
    Email     string `json:"email"`
    AvatarURL string `json:"avatarUrl"`
}
```

### 2.2 PR Fetching Implementation
```go
func (c *CLIClient) GetPR(number int) (*PRInfo, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("pr-%d", number)
    if cached, ok := c.cache.Load(cacheKey); ok {
        if pr, ok := cached.(*PRInfo); ok {
            // Check if cache is still fresh (5 minutes)
            if time.Since(pr.fetchedAt) < 5*time.Minute {
                return pr, nil
            }
        }
    }
    
    // Fetch fresh data
    cmd := exec.Command(c.command, "pr", "view", strconv.Itoa(number), 
        "--json", "number,title,body,state,headRefName,headRefOid,baseRefName,baseSHA,author,headRepository,mergeable,merged,isDraft,createdAt,updatedAt,mergedAt,url")
    
    cmd.Dir = getCurrentDir()
    
    ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
    defer cancel()
    cmd = cmd.WithContext(ctx)
    
    output, err := cmd.Output()
    if err != nil {
        return nil, c.parseGHError(err)
    }
    
    var prInfo PRInfo
    if err := json.Unmarshal(output, &prInfo); err != nil {
        return nil, fmt.Errorf("failed to parse PR data: %w", err)
    }
    
    // Validate required fields
    if err := c.validatePRInfo(&prInfo); err != nil {
        return nil, err
    }
    
    // Cache the result
    prInfo.fetchedAt = time.Now()
    c.cache.Store(cacheKey, &prInfo)
    
    return &prInfo, nil
}

func (c *CLIClient) validatePRInfo(pr *PRInfo) error {
    if pr.Number == 0 {
        return fmt.Errorf("invalid PR: missing number")
    }
    if pr.HeadRef == "" {
        return fmt.Errorf("invalid PR: missing head branch")
    }
    if pr.BaseRef == "" {
        return fmt.Errorf("invalid PR: missing base branch")
    }
    return nil
}
```

### 2.3 Error Handling for GitHub Operations
```go
func (c *CLIClient) parseGHError(err error) error {
    if exitErr, ok := err.(*exec.ExitError); ok {
        stderr := string(exitErr.Stderr)
        
        switch {
        case strings.Contains(stderr, "not found"):
            return NewPRNotFoundError("PR not found or access denied")
        case strings.Contains(stderr, "authentication"):
            return NewAuthenticationError("GitHub authentication required")
        case strings.Contains(stderr, "rate limit"):
            return NewRateLimitError("GitHub API rate limit exceeded")
        case strings.Contains(stderr, "network"):
            return NewNetworkError("Network error accessing GitHub")
        default:
            return NewGitHubError(fmt.Sprintf("GitHub CLI error: %s", stderr))
        }
    }
    
    return NewGitHubError(fmt.Sprintf("GitHub operation failed: %v", err))
}

type GitHubError struct {
    Type    GitHubErrorType
    Message string
    Cause   error
}

type GitHubErrorType int
const (
    GitHubErrorGeneral GitHubErrorType = iota
    GitHubErrorAuth
    GitHubErrorNotFound
    GitHubErrorRateLimit
    GitHubErrorNetwork
)

func (e *GitHubError) SuggestedActions() []string {
    switch e.Type {
    case GitHubErrorAuth:
        return []string{
            "Run 'gh auth login' to authenticate with GitHub",
            "Check 'gh auth status' to verify authentication",
            "Ensure you have access to this repository",
        }
    case GitHubErrorNotFound:
        return []string{
            "Verify the PR number is correct",
            "Check if the PR exists in this repository",
            "Ensure you have read access to the repository",
        }
    case GitHubErrorRateLimit:
        return []string{
            "Wait a few minutes for rate limit to reset",
            "Use authenticated requests (gh auth login)",
        }
    case GitHubErrorNetwork:
        return []string{
            "Check your internet connection",
            "Verify GitHub is accessible",
            "Try again in a few moments",
        }
    default:
        return []string{
            "Check GitHub CLI installation: gh --version",
            "Verify repository access: gh repo view",
        }
    }
}
```

## 3. PR Worktree Workflow

### 3.1 PR Worktree Creation
```go
type PRWorktreeManager struct {
    client     GitHubClient
    repo       GitRepo
    pathMgr    *PathManager
    config     *Config
}

func (pm *PRWorktreeManager) CreatePRWorktree(prNumber int) (*PRWorktreeInfo, error) {
    // 1. Fetch PR information
    prInfo, err := pm.client.GetPR(prNumber)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch PR info: %w", err)
    }
    
    // 2. Validate PR state
    if err := pm.validatePRForWorktree(prInfo); err != nil {
        return nil, err
    }
    
    // 3. Create local tracking branch
    localBranch := fmt.Sprintf("pr-%d-%s", prInfo.Number, prInfo.HeadRef)
    if err := pm.createTrackingBranch(prInfo, localBranch); err != nil {
        return nil, fmt.Errorf("failed to create tracking branch: %w", err)
    }
    
    // 4. Create worktree
    wtPath := pm.pathMgr.PRWorktreePath(prNumber)
    if err := pm.repo.CreateWorktree(wtPath, localBranch); err != nil {
        // Cleanup tracking branch on failure
        pm.repo.DeleteBranch(localBranch, true)
        return nil, fmt.Errorf("failed to create worktree: %w", err)
    }
    
    // 5. Setup PR-specific environment
    wtInfo := &PRWorktreeInfo{
        PRInfo:       prInfo,
        WorktreePath: wtPath,
        LocalBranch:  localBranch,
        CreatedAt:    time.Now(),
    }
    
    if err := pm.setupPREnvironment(wtInfo); err != nil {
        // Log warning but don't fail
        pm.config.UI.Warning("PR environment setup failed: %v", err)
    }
    
    return wtInfo, nil
}

func (pm *PRWorktreeManager) validatePRForWorktree(pr *PRInfo) error {
    if pr.State == PRStateClosed {
        return NewValidationError("Cannot create worktree for closed PR")
    }
    
    if pr.Merged {
        return NewValidationError("Cannot create worktree for merged PR")
    }
    
    if pr.HeadRepo == nil {
        return NewValidationError("PR head repository not accessible")
    }
    
    return nil
}

func (pm *PRWorktreeManager) createTrackingBranch(pr *PRInfo, localBranch string) error {
    // Delete existing branch if it exists
    if pm.repo.BranchExists(localBranch) {
        if err := pm.repo.DeleteBranch(localBranch, true); err != nil {
            return fmt.Errorf("failed to delete existing branch: %w", err)
        }
    }
    
    // Fetch PR branch
    fetchRef := fmt.Sprintf("pull/%d/head:%s", pr.Number, localBranch)
    if err := pm.repo.Fetch("origin", fetchRef); err != nil {
        return fmt.Errorf("failed to fetch PR: %w", err)
    }
    
    return nil
}
```

### 3.2 PR Metadata Management
```go
type PRWorktreeInfo struct {
    *PRInfo
    WorktreePath string    `yaml:"worktree_path"`
    LocalBranch  string    `yaml:"local_branch"`
    CreatedAt    time.Time `yaml:"created_at"`
    UpdatedAt    time.Time `yaml:"updated_at"`
}

func (pm *PRWorktreeManager) setupPREnvironment(wtInfo *PRWorktreeInfo) error {
    // 1. Standard environment setup (inherits from main)
    envSetup := &EnvironmentSetup{
        WorktreePath: wtInfo.WorktreePath,
        RepoPath:     pm.pathMgr.RepoRoot,
        Config:       pm.config,
    }
    
    if err := envSetup.Setup(); err != nil {
        return fmt.Errorf("environment setup failed: %w", err)
    }
    
    // 2. Create PR-specific metadata file
    metadataPath := filepath.Join(wtInfo.WorktreePath, ".wtree-pr-info")
    if err := pm.writePRMetadata(wtInfo, metadataPath); err != nil {
        return fmt.Errorf("metadata write failed: %w", err)
    }
    
    // 3. Create PR information file for easy reference
    infoPath := filepath.Join(wtInfo.WorktreePath, "PR-INFO.md")
    if err := pm.writePRInfoFile(wtInfo, infoPath); err != nil {
        // Non-fatal - just log warning
        pm.config.UI.Warning("Failed to create PR info file: %v", err)
    }
    
    return nil
}

func (pm *PRWorktreeManager) writePRMetadata(wtInfo *PRWorktreeInfo, path string) error {
    data, err := yaml.Marshal(wtInfo)
    if err != nil {
        return fmt.Errorf("failed to marshal PR metadata: %w", err)
    }
    
    return os.WriteFile(path, data, 0644)
}

func (pm *PRWorktreeManager) writePRInfoFile(wtInfo *PRWorktreeInfo, path string) error {
    content := fmt.Sprintf(`# PR #%d: %s

**Author:** %s  
**Status:** %s  
**Branch:** %s → %s  
**URL:** %s  

## Description
%s

---
*This file was generated by wtree for PR #%d*
`, 
        wtInfo.Number, wtInfo.Title,
        wtInfo.Author.Login,
        wtInfo.State,
        wtInfo.HeadRef, wtInfo.BaseRef,
        wtInfo.HTMLURL,
        wtInfo.Body,
        wtInfo.Number,
    )
    
    return os.WriteFile(path, []byte(content), 0644)
}
```

## 4. PR Cleanup Operations

### 4.1 PR Worktree Discovery
```go
func (pm *PRWorktreeManager) FindPRWorktrees() ([]*PRWorktreeInfo, error) {
    worktrees, err := pm.repo.ListWorktrees()
    if err != nil {
        return nil, fmt.Errorf("failed to list worktrees: %w", err)
    }
    
    var prWorktrees []*PRWorktreeInfo
    for _, wt := range worktrees {
        if wt.IsMainRepo {
            continue
        }
        
        // Check if this is a PR worktree
        metadataPath := filepath.Join(wt.Path, ".wtree-pr-info")
        if !fileExists(metadataPath) {
            continue
        }
        
        // Load PR metadata
        prInfo, err := pm.loadPRMetadata(metadataPath)
        if err != nil {
            // Log warning but continue
            pm.config.UI.Warning("Failed to load PR metadata for %s: %v", wt.Path, err)
            continue
        }
        
        prWorktrees = append(prWorktrees, prInfo)
    }
    
    return prWorktrees, nil
}

func (pm *PRWorktreeManager) loadPRMetadata(path string) (*PRWorktreeInfo, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read metadata: %w", err)
    }
    
    var prInfo PRWorktreeInfo
    if err := yaml.Unmarshal(data, &prInfo); err != nil {
        return nil, fmt.Errorf("failed to parse metadata: %w", err)
    }
    
    return &prInfo, nil
}
```

### 4.2 Intelligent PR Cleanup
```go
func (pm *PRWorktreeManager) CleanupPRWorktrees(options CleanupOptions) error {
    prWorktrees, err := pm.FindPRWorktrees()
    if err != nil {
        return fmt.Errorf("failed to find PR worktrees: %w", err)
    }
    
    // Filter based on cleanup options
    toCleanup := pm.filterWorktreesForCleanup(prWorktrees, options)
    
    if len(toCleanup) == 0 {
        pm.config.UI.Info("No PR worktrees to cleanup")
        return nil
    }
    
    // Display cleanup plan
    if err := pm.displayCleanupPlan(toCleanup); err != nil {
        return err
    }
    
    // Confirm cleanup
    if !options.Force {
        if err := pm.config.UI.Confirm("Continue with cleanup?"); err != nil {
            return err
        }
    }
    
    // Execute cleanup
    return pm.executeCleanup(toCleanup)
}

type CleanupOptions struct {
    PRNumbers   []int
    CleanAll    bool
    MergedOnly  bool
    OlderThan   time.Duration
    Force       bool
}

func (pm *PRWorktreeManager) filterWorktreesForCleanup(worktrees []*PRWorktreeInfo, options CleanupOptions) []*PRWorktreeInfo {
    var filtered []*PRWorktreeInfo
    
    for _, wt := range worktrees {
        // Filter by PR numbers if specified
        if len(options.PRNumbers) > 0 {
            found := false
            for _, num := range options.PRNumbers {
                if wt.Number == num {
                    found = true
                    break
                }
            }
            if !found {
                continue
            }
        }
        
        // Filter by merged status
        if options.MergedOnly && !wt.Merged {
            continue
        }
        
        // Filter by age
        if options.OlderThan > 0 && time.Since(wt.CreatedAt) < options.OlderThan {
            continue
        }
        
        filtered = append(filtered, wt)
    }
    
    return filtered
}
```

## 5. Authentication and Security

### 5.1 Authentication Validation
```go
func (c *CLIClient) ValidateAuthentication() error {
    cmd := exec.Command(c.command, "auth", "status")
    if err := cmd.Run(); err != nil {
        return NewAuthenticationError("GitHub CLI not authenticated")
    }
    
    // Verify repository access
    cmd = exec.Command(c.command, "repo", "view", c.repository)
    if err := cmd.Run(); err != nil {
        return NewAuthenticationError("No access to repository")
    }
    
    return nil
}
```

### 5.2 Rate Limiting and Caching
```go
type PRCache struct {
    cache     *sync.Map
    ttl       time.Duration
    lastFetch map[string]time.Time
    mu        sync.RWMutex
}

func (pc *PRCache) Get(key string) (*PRInfo, bool) {
    pc.mu.RLock()
    defer pc.mu.RUnlock()
    
    if lastFetch, ok := pc.lastFetch[key]; ok {
        if time.Since(lastFetch) > pc.ttl {
            return nil, false // Expired
        }
    }
    
    if value, ok := pc.cache.Load(key); ok {
        if pr, ok := value.(*PRInfo); ok {
            return pr, true
        }
    }
    
    return nil, false
}

func (pc *PRCache) Set(key string, pr *PRInfo) {
    pc.mu.Lock()
    defer pc.mu.Unlock()
    
    pc.cache.Store(key, pr)
    pc.lastFetch[key] = time.Now()
}
```

This GitHub integration architecture provides robust, secure, and efficient access to GitHub PR information while gracefully handling authentication, rate limiting, and error scenarios.