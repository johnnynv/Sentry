package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	lastCommit    map[string]string // repoKey -> last commit SHA
	deployService *DeployService    // Deploy service for triggered deployments
}

// RetryConfig defines retry behavior for network requests
type RetryConfig struct {
	MaxRetries int
	RetryDelay time.Duration
}

// NewMonitorService creates a new monitor service instance
func NewMonitorService(config *Config, deployService *DeployService) *MonitorService {
	return &MonitorService{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Monitor.Poll.Timeout) * time.Second,
		},
		lastCommit:    make(map[string]string),
		deployService: deployService,
	}
}

// StartMonitoring starts the continuous monitoring process
func (m *MonitorService) StartMonitoring() error {
	fmt.Println("Starting repository monitoring...")

	// Initial check to get baseline
	if err := m.CheckAllRepositories(); err != nil {
		return fmt.Errorf("initial repository check failed: %w", err)
	}

	// Start polling loop
	ticker := time.NewTicker(time.Duration(m.config.Monitor.Poll.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.CheckAllRepositories(); err != nil {
				fmt.Printf("Error checking repositories: %v\n", err)
			}
		}
	}
}

// CheckAllRepositories checks both repository A and B for changes
func (m *MonitorService) CheckAllRepositories() error {
	var errors []string

	// Check repository A - if changed, deploy from repository B
	if changed, err := m.checkRepository(&m.config.Monitor.RepoA, "repo_a"); err != nil {
		errors = append(errors, fmt.Sprintf("repo_a: %v", err))
	} else if changed {
		fmt.Printf("Repository A has new commits, triggering deployment from Repository B...\n")
		if deployErr := m.triggerDeploymentFromRepoB(); deployErr != nil {
			errors = append(errors, fmt.Sprintf("repo_a triggered deployment failed: %v", deployErr))
		}
	}

	// Check repository B - if changed, also deploy from repository B
	if changed, err := m.checkRepository(&m.config.Monitor.RepoB, "repo_b"); err != nil {
		errors = append(errors, fmt.Sprintf("repo_b: %v", err))
	} else if changed {
		fmt.Printf("Repository B has new commits, triggering deployment from Repository B...\n")
		if deployErr := m.triggerDeploymentFromRepoB(); deployErr != nil {
			errors = append(errors, fmt.Sprintf("repo_b triggered deployment failed: %v", deployErr))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("repository check errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// checkRepository checks a single repository for changes
func (m *MonitorService) checkRepository(repo *RepoConfig, repoKey string) (bool, error) {
	commit, err := m.GetLatestCommit(repo)
	if err != nil {
		return false, fmt.Errorf("failed to get latest commit: %w", err)
	}

	lastSHA, exists := m.lastCommit[repoKey]
	if !exists {
		// First time checking this repository
		m.lastCommit[repoKey] = commit.SHA
		fmt.Printf("%s: Initial commit recorded: %s\n", repoKey, commit.SHA[:8])
		return false, nil
	}

	if commit.SHA != lastSHA {
		fmt.Printf("%s: New commit detected: %s -> %s\n", repoKey, lastSHA[:8], commit.SHA[:8])
		fmt.Printf("  Author: %s\n", commit.Author)
		fmt.Printf("  Message: %s\n", commit.Message)
		m.lastCommit[repoKey] = commit.SHA
		return true, nil
	}

	return false, nil
}

// GetLatestCommit retrieves the latest commit information from repository with retry
func (m *MonitorService) GetLatestCommit(repo *RepoConfig) (*CommitInfo, error) {
	retryConfig := RetryConfig{
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
	}

	var lastErr error
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("Retrying API call (attempt %d/%d) after error: %v\n", attempt, retryConfig.MaxRetries, lastErr)
			time.Sleep(retryConfig.RetryDelay)
		}

		var commit *CommitInfo
		var err error

		switch repo.Type {
		case "github":
			commit, err = m.getGitHubLatestCommit(repo)
		case "gitlab":
			commit, err = m.getGitLabLatestCommit(repo)
		default:
			return nil, fmt.Errorf("unsupported repository type: %s", repo.Type)
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
func (m *MonitorService) getGitHubLatestCommit(repo *RepoConfig) (*CommitInfo, error) {
	// Extract owner and repo from URL
	// Expected format: https://github.com/owner/repo
	parts := strings.Split(strings.TrimSuffix(repo.URL, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub URL format: %s", repo.URL)
	}
	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]

	// GitHub API endpoint for latest commit
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repoName, repo.Branch)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("token %s", repo.Token))
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
func (m *MonitorService) getGitLabLatestCommit(repo *RepoConfig) (*CommitInfo, error) {
	// Extract project path from URL
	// Expected format: https://gitlab.com/owner/project or https://gitlab-master.nvidia.com/owner/project
	url := strings.TrimSuffix(repo.URL, "/")

	// Find the base URL and project path
	var baseURL, projectPath string
	if strings.Contains(url, "gitlab.com") {
		baseURL = "https://gitlab.com"
		projectPath = strings.TrimPrefix(url, "https://gitlab.com/")
	} else if strings.Contains(url, "gitlab-master.nvidia.com") {
		baseURL = "https://gitlab-master.nvidia.com"
		projectPath = strings.TrimPrefix(url, "https://gitlab-master.nvidia.com/")
	} else {
		return nil, fmt.Errorf("unsupported GitLab URL format: %s", repo.URL)
	}

	// URL encode the project path
	projectPath = strings.ReplaceAll(projectPath, "/", "%2F")

	// GitLab API endpoint for latest commit
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits/%s", baseURL, projectPath, repo.Branch)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", repo.Token))

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

// TriggerManualCheck performs a manual check of all repositories
func (m *MonitorService) TriggerManualCheck() error {
	fmt.Println("Performing manual repository check...")
	return m.CheckAllRepositories()
}

// triggerDeploymentFromRepoB triggers deployment from repository B
func (m *MonitorService) triggerDeploymentFromRepoB() error {
	if m.deployService == nil {
		return fmt.Errorf("deploy service not initialized")
	}

	fmt.Printf("Deploying Tekton configurations from Repository B (%s)...\n", m.config.Monitor.RepoB.URL)

	// Clone repository B and deploy its .tekton configurations
	result, err := m.deployService.DeployFromRepository(&m.config.Monitor.RepoB, "repo_b")
	if err != nil {
		AppLogger.LogDeploymentFailure("repo_b", err)
		return fmt.Errorf("deployment from repository B failed: %w", err)
	}

	if !result.Success {
		AppLogger.LogDeploymentFailure("repo_b", fmt.Errorf(result.Error))
		return fmt.Errorf("deployment from repository B unsuccessful: %s", result.Error)
	}

	AppLogger.LogDeploymentSuccess("repo_b", len(result.FilesDeployed))
	fmt.Printf("Successfully deployed %d Tekton files from Repository B\n", len(result.FilesDeployed))

	// Log deployment details
	for _, file := range result.FilesDeployed {
		fmt.Printf("  - Deployed: %s\n", file)
	}

	return nil
}
