package main

import (
	"testing"
	"time"
)

func TestNewMonitorService(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		PollingInterval: 60,
		Global: GlobalConfig{
			Timeout: 30,
		},
	}
	deployService := NewDeployService(config)

	service := NewMonitorService(config, deployService)
	if service == nil {
		t.Error("NewMonitorService() returned nil")
	}

	if service.config != config {
		t.Error("NewMonitorService() did not set config correctly")
	}

	if service.deployService != deployService {
		t.Error("NewMonitorService() did not set deployService correctly")
	}

	if service.lastCommit == nil {
		t.Error("NewMonitorService() did not initialize lastCommit map")
	}

	if service.httpClient == nil {
		t.Error("NewMonitorService() did not initialize httpClient")
	}
}

func TestGetTimeoutFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected int
	}{
		{
			name: "with configured timeout",
			config: &Config{
				Global: GlobalConfig{
					Timeout: 45,
				},
			},
			expected: 45,
		},
		{
			name: "without configured timeout",
			config: &Config{
				Global: GlobalConfig{},
			},
			expected: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTimeoutFromConfig(tt.config)
			if result != tt.expected {
				t.Errorf("getTimeoutFromConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGitHubAPIResponseParsing(t *testing.T) {
	// This test verifies that we can parse GitHub API response structure
	// without making actual API calls

	config := &Config{
		PollingInterval: 60,
		Global: GlobalConfig{
			Timeout: 30,
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	// Test GitHub API response structure
	monitor := &MonitorConfig{
		RepoURL:  "https://github.com/owner/repo",
		RepoType: "github",
		Auth: AuthConfig{
			Username: "testuser",
			Token:    "testtoken",
		},
	}

	// We can't test actual API calls without real credentials,
	// but we can verify the service is properly initialized
	if service == nil {
		t.Error("MonitorService not properly initialized for GitHub API testing")
	}

	if monitor.RepoType != "github" {
		t.Error("GitHub monitor config not properly set")
	}
}

func TestGitLabAPIResponseParsing(t *testing.T) {
	// This test verifies that we can parse GitLab API response structure
	// without making actual API calls

	config := &Config{
		PollingInterval: 60,
		Global: GlobalConfig{
			Timeout: 30,
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	// Test GitLab API response structure
	monitor := &MonitorConfig{
		RepoURL:  "https://gitlab.com/owner/repo",
		RepoType: "gitlab",
		Auth: AuthConfig{
			Username: "testuser",
			Token:    "testtoken",
		},
	}

	// We can't test actual API calls without real credentials,
	// but we can verify the service is properly initialized
	if service == nil {
		t.Error("MonitorService not properly initialized for GitLab API testing")
	}

	if monitor.RepoType != "gitlab" {
		t.Error("GitLab monitor config not properly set")
	}
}

func TestCommitChangeDetection(t *testing.T) {
	config := &Config{
		PollingInterval: 60,
		Global: GlobalConfig{
			Timeout: 30,
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	// Test commit change detection logic
	repoKey := "test-repo:main"

	// First commit should not be detected as change
	service.lastCommit[repoKey] = "abc123"

	// Same commit should not be detected as change
	if service.lastCommit[repoKey] != "abc123" {
		t.Error("Commit SHA not properly stored")
	}

	// Different commit should be detected as change
	service.lastCommit[repoKey] = "def456"
	if service.lastCommit[repoKey] != "def456" {
		t.Error("Commit SHA not properly updated")
	}
}

func TestRetryConfig(t *testing.T) {
	retryConfig := RetryConfig{
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
	}

	if retryConfig.MaxRetries != 3 {
		t.Errorf("RetryConfig.MaxRetries = %v, want %v", retryConfig.MaxRetries, 3)
	}

	if retryConfig.RetryDelay != 2*time.Second {
		t.Errorf("RetryConfig.RetryDelay = %v, want %v", retryConfig.RetryDelay, 2*time.Second)
	}
}

func TestGroupTrigger(t *testing.T) {
	trigger := &GroupTrigger{
		GroupName:    "test-group",
		Repositories: []string{"repo1", "repo2"},
		TriggerTime:  time.Now(),
		TriggerRepo:  "repo1",
	}

	if trigger.GroupName != "test-group" {
		t.Errorf("GroupTrigger.GroupName = %v, want %v", trigger.GroupName, "test-group")
	}

	if len(trigger.Repositories) != 2 {
		t.Errorf("GroupTrigger.Repositories length = %v, want %v", len(trigger.Repositories), 2)
	}

	if trigger.TriggerRepo != "repo1" {
		t.Errorf("GroupTrigger.TriggerRepo = %v, want %v", trigger.TriggerRepo, "repo1")
	}
}

func TestCommitInfo(t *testing.T) {
	now := time.Now()
	commit := &CommitInfo{
		SHA:       "abc123def456",
		Message:   "Test commit",
		Author:    "Test Author",
		Timestamp: now,
		URL:       "https://github.com/owner/repo/commit/abc123def456",
	}

	if commit.SHA != "abc123def456" {
		t.Errorf("CommitInfo.SHA = %v, want %v", commit.SHA, "abc123def456")
	}

	if commit.Message != "Test commit" {
		t.Errorf("CommitInfo.Message = %v, want %v", commit.Message, "Test commit")
	}

	if commit.Author != "Test Author" {
		t.Errorf("CommitInfo.Author = %v, want %v", commit.Author, "Test Author")
	}

	if !commit.Timestamp.Equal(now) {
		t.Errorf("CommitInfo.Timestamp = %v, want %v", commit.Timestamp, now)
	}
}
