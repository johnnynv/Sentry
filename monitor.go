package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CommitInfo represents commit information from Git APIs
type CommitInfo struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
}

// MonitorService handles repository monitoring
type MonitorService struct {
	config        *Config
	httpClient    *http.Client
	lastCommit    map[string]string // repoName -> last commit SHA
	deployService *DeployService    // Deploy service for triggered deployments
	mu            sync.RWMutex      // Protects lastCommit map
}

// RetryConfig defines retry behavior for network requests
type RetryConfig struct {
	MaxRetries int
	RetryDelay time.Duration
}

// GroupTrigger represents a triggered group deployment
type GroupTrigger struct {
	GroupName    string
	Repositories []string
	TriggerTime  time.Time
	TriggerRepo  string // Which repo triggered this group
}

// NewMonitorService creates a new monitor service instance
func NewMonitorService(config *Config, deployService *DeployService) *MonitorService {
	return &MonitorService{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(getTimeoutFromConfig(config)) * time.Second,
		},
		lastCommit:    make(map[string]string),
		deployService: deployService,
	}
}

// getTimeoutFromConfig gets timeout from global config or uses default
func getTimeoutFromConfig(config *Config) int {
	if config.Global.Timeout > 0 {
		return config.Global.Timeout
	}
	return 30 // Default 30 seconds
}

// StartMonitoring starts the continuous monitoring process
func (m *MonitorService) StartMonitoring() error {
	AppLogger.InfoS("Starting repository monitoring", "polling_interval", m.config.PollingInterval)

	// Initial check to get baseline
	if err := m.CheckAllRepositories(); err != nil {
		return fmt.Errorf("initial repository check failed: %w", err)
	}

	// Start polling loop
	ticker := time.NewTicker(time.Duration(m.config.PollingInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.CheckAllRepositories(); err != nil {
				AppLogger.ErrorS("Error checking repositories", "error", err)
			}
		}
	}
}

// CheckAllRepositories checks all configured repositories for changes
func (m *MonitorService) CheckAllRepositories() error {
	var errors []string
	triggeredGroups := make(map[string]*GroupTrigger)
	triggeredIndividual := make([]string, 0)

	// Check all repositories for changes
	for _, repo := range m.config.Repositories {
		changed, err := m.checkRepository(&repo)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", repo.Name, err))
			continue
		}

		if changed {
			AppLogger.InfoS("Repository change detected", "repo", repo.Name, "group", repo.Group)

			if repo.Group != "" {
				// This repo belongs to a group
				if _, exists := triggeredGroups[repo.Group]; !exists {
					triggeredGroups[repo.Group] = &GroupTrigger{
						GroupName:    repo.Group,
						Repositories: make([]string, 0),
						TriggerTime:  time.Now(),
						TriggerRepo:  repo.Name,
					}
				}
				// Add all repositories in this group to the trigger list
				for _, r := range m.config.Repositories {
					if r.Group == repo.Group {
						triggeredGroups[repo.Group].Repositories = append(triggeredGroups[repo.Group].Repositories, r.Name)
					}
				}
			} else {
				// Individual repository (no group)
				triggeredIndividual = append(triggeredIndividual, repo.Name)
			}
		}
	}

	// Process group triggers
	for groupName, trigger := range triggeredGroups {
		AppLogger.InfoS("Triggering group deployment",
			"group", groupName,
			"triggered_by", trigger.TriggerRepo,
			"repositories", trigger.Repositories)

		if err := m.triggerGroupDeployment(groupName, trigger.Repositories); err != nil {
			errors = append(errors, fmt.Sprintf("group %s deployment failed: %v", groupName, err))
		}
	}

	// Process individual triggers
	for _, repoName := range triggeredIndividual {
		AppLogger.InfoS("Triggering individual deployment", "repo", repoName)
		if err := m.triggerIndividualDeployment(repoName); err != nil {
			errors = append(errors, fmt.Sprintf("individual %s deployment failed: %v", repoName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("repository check errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// checkRepository checks a single repository for changes
func (m *MonitorService) checkRepository(repo *RepositoryConfig) (bool, error) {
	// Check all configured branches
	for _, branch := range repo.Monitor.Branches {
		changed, err := m.checkRepositoryBranch(repo, branch)
		if err != nil {
			return false, err
		}
		if changed {
			return true, nil // Any branch change triggers deployment
		}
	}
	return false, nil
}

// checkRepositoryBranch checks a specific branch of a repository
func (m *MonitorService) checkRepositoryBranch(repo *RepositoryConfig, branch string) (bool, error) {
	// Create a temporary repo config for this specific branch
	branchRepo := &MonitorConfig{
		RepoURL:  repo.Monitor.RepoURL,
		RepoType: repo.Monitor.RepoType,
		Auth:     repo.Monitor.Auth,
	}

	commit, err := m.GetLatestCommit(branchRepo, branch)
	if err != nil {
		return false, fmt.Errorf("failed to get latest commit for branch %s: %w", branch, err)
	}

	cacheKey := fmt.Sprintf("%s:%s", repo.Name, branch)

	m.mu.Lock()
	lastSHA, exists := m.lastCommit[cacheKey]
	if !exists {
		// First time checking this repository/branch
		m.lastCommit[cacheKey] = commit.SHA
		m.mu.Unlock()
		AppLogger.InfoS("Initial commit recorded",
			"repo", repo.Name,
			"branch", branch,
			"sha", commit.SHA[:8])
		return false, nil
	}
	m.mu.Unlock()

	if commit.SHA != lastSHA {
		AppLogger.InfoS("New commit detected",
			"repo", repo.Name,
			"branch", branch,
			"old_sha", lastSHA[:8],
			"new_sha", commit.SHA[:8],
			"author", commit.Author,
			"message", commit.Message)

		m.mu.Lock()
		m.lastCommit[cacheKey] = commit.SHA
		m.mu.Unlock()
		return true, nil
	}

	return false, nil
}

// GetLatestCommit retrieves the latest commit information from repository with retry
func (m *MonitorService) GetLatestCommit(monitor *MonitorConfig, branch string) (*CommitInfo, error) {
	retryConfig := RetryConfig{
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
	}

	var lastErr error
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			AppLogger.WarnS("Retrying API call",
				"attempt", attempt,
				"max_retries", retryConfig.MaxRetries,
				"error", lastErr)
			time.Sleep(retryConfig.RetryDelay)
		}

		var commit *CommitInfo
		var err error

		switch monitor.RepoType {
		case "github":
			commit, err = m.getGitHubLatestCommit(monitor, branch)
		case "gitlab":
			commit, err = m.getGitLabLatestCommit(monitor, branch)
		case "gitea":
			commit, err = m.getGiteaLatestCommit(monitor, branch)
		default:
			return nil, fmt.Errorf("unsupported repository type: %s", monitor.RepoType)
		}

		if err == nil {
			return commit, nil
		}

		lastErr = err

		// Don't retry for authentication or client errors (4xx)
		if strings.Contains(err.Error(), "status 4") {
			break
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", retryConfig.MaxRetries, lastErr)
}

// getGitHubLatestCommit gets latest commit from GitHub API
func (m *MonitorService) getGitHubLatestCommit(monitor *MonitorConfig, branch string) (*CommitInfo, error) {
	// Extract owner and repo from URL
	parts := strings.Split(strings.TrimSuffix(monitor.RepoURL, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub URL format: %s", monitor.RepoURL)
	}
	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]

	// GitHub API endpoint for latest commit
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repoName, branch)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("token %s", monitor.Auth.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Limit response body size to prevent memory issues
	limitedReader := io.LimitReader(resp.Body, 1024*1024) // 1MB limit
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var githubCommit struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		HTMLURL string `json:"html_url"`
	}

	if err := json.Unmarshal(body, &githubCommit); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	return &CommitInfo{
		SHA:       githubCommit.SHA,
		Message:   githubCommit.Commit.Message,
		Author:    githubCommit.Commit.Author.Name,
		Timestamp: githubCommit.Commit.Author.Date,
		URL:       githubCommit.HTMLURL,
	}, nil
}

// getGitLabLatestCommit gets latest commit from GitLab API
func (m *MonitorService) getGitLabLatestCommit(monitor *MonitorConfig, branch string) (*CommitInfo, error) {
	url := strings.TrimSuffix(monitor.RepoURL, "/")

	// Find the base URL and project path
	var baseURL, projectPath string
	if strings.Contains(url, "gitlab.com") {
		baseURL = "https://gitlab.com"
		projectPath = strings.TrimPrefix(url, "https://gitlab.com/")
	} else if strings.Contains(url, "gitlab-master.nvidia.com") {
		baseURL = "https://gitlab-master.nvidia.com"
		projectPath = strings.TrimPrefix(url, "https://gitlab-master.nvidia.com/")
	} else {
		return nil, fmt.Errorf("unsupported GitLab URL format: %s", monitor.RepoURL)
	}

	// URL encode the project path
	projectPath = strings.ReplaceAll(projectPath, "/", "%2F")

	// GitLab API endpoint for latest commit
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits/%s", baseURL, projectPath, branch)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", monitor.Auth.Token))

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Limit response body size to prevent memory issues
	limitedReader := io.LimitReader(resp.Body, 1024*1024) // 1MB limit
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var gitlabCommit struct {
		ID         string    `json:"id"`
		Title      string    `json:"title"`
		AuthorName string    `json:"author_name"`
		CreatedAt  time.Time `json:"created_at"`
		WebURL     string    `json:"web_url"`
	}

	if err := json.Unmarshal(body, &gitlabCommit); err != nil {
		return nil, fmt.Errorf("failed to parse GitLab response: %w", err)
	}

	return &CommitInfo{
		SHA:       gitlabCommit.ID,
		Message:   gitlabCommit.Title,
		Author:    gitlabCommit.AuthorName,
		Timestamp: gitlabCommit.CreatedAt,
		URL:       gitlabCommit.WebURL,
	}, nil
}

// getGiteaLatestCommit gets latest commit from Gitea API
func (m *MonitorService) getGiteaLatestCommit(monitor *MonitorConfig, branch string) (*CommitInfo, error) {
	// Extract base URL, owner and repo from URL
	url := strings.TrimSuffix(monitor.RepoURL, "/")
	parts := strings.Split(url, "/")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid Gitea URL format: %s", monitor.RepoURL)
	}

	baseURL := strings.Join(parts[:3], "/") // https://gitea.example.com
	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]

	// Gitea API endpoint for latest commit
	apiURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/commits/%s", baseURL, owner, repoName, branch)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("token %s", monitor.Auth.Token))

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gitea API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Limit response body size to prevent memory issues
	limitedReader := io.LimitReader(resp.Body, 1024*1024) // 1MB limit
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var giteaCommit struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		HTMLURL string `json:"html_url"`
	}

	if err := json.Unmarshal(body, &giteaCommit); err != nil {
		return nil, fmt.Errorf("failed to parse Gitea response: %w", err)
	}

	return &CommitInfo{
		SHA:       giteaCommit.SHA,
		Message:   giteaCommit.Commit.Message,
		Author:    giteaCommit.Commit.Author.Name,
		Timestamp: giteaCommit.Commit.Author.Date,
		URL:       giteaCommit.HTMLURL,
	}, nil
}

// TriggerManualCheck performs a manual check of all repositories
func (m *MonitorService) TriggerManualCheck() error {
	AppLogger.Info("Performing manual repository check")
	return m.CheckAllRepositories()
}

// triggerGroupDeployment triggers deployment for a group of repositories
func (m *MonitorService) triggerGroupDeployment(groupName string, repositories []string) error {
	if m.deployService == nil {
		return fmt.Errorf("deploy service not initialized")
	}

	groupConfig, exists := m.config.Groups[groupName]
	if !exists {
		return fmt.Errorf("group configuration not found: %s", groupName)
	}

	AppLogger.InfoS("Starting group deployment",
		"group", groupName,
		"strategy", groupConfig.ExecutionStrategy,
		"repositories", repositories)

	return m.deployService.DeployGroup(groupName, repositories, &groupConfig)
}

// triggerIndividualDeployment triggers deployment for an individual repository
func (m *MonitorService) triggerIndividualDeployment(repoName string) error {
	if m.deployService == nil {
		return fmt.Errorf("deploy service not initialized")
	}

	// Find the repository config
	var repoConfig *RepositoryConfig
	for _, repo := range m.config.Repositories {
		if repo.Name == repoName {
			repoConfig = &repo
			break
		}
	}

	if repoConfig == nil {
		return fmt.Errorf("repository configuration not found: %s", repoName)
	}

	AppLogger.InfoS("Starting individual deployment", "repo", repoName)

	return m.deployService.DeployIndividual(repoConfig)
}
