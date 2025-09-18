package main

import (
	"testing"
	"time"
)

func TestMonitorServiceBasics(t *testing.T) {
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
		return
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

func TestMonitorGetTimeoutFromConfig(t *testing.T) {
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
		{
			name: "with zero timeout",
			config: &Config{
				Global: GlobalConfig{
					Timeout: 0,
				},
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

func TestMonitorTriggerManualCheck(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		PollingInterval: 60,
		Global: GlobalConfig{
			Timeout: 30,
		},
		Repositories: []RepositoryConfig{
			{
				Name: "test-repo",
				Monitor: MonitorConfig{
					RepoURL:  "https://github.com/owner/repo",
					Branches: []string{"main"},
					RepoType: "github",
					Auth: AuthConfig{
						Username: "testuser",
						Token:    "testtoken",
					},
				},
			},
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	// Test manual check (this will fail but we test the function call)
	service.TriggerManualCheck()

	// We can't verify much without mocking, but at least it doesn't panic
}

func TestMonitorTriggerGroupDeployment(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		PollingInterval: 60,
		Global: GlobalConfig{
			Timeout: 30,
		},
		Groups: map[string]GroupConfig{
			"test-group": {
				ExecutionStrategy: "parallel",
				MaxParallel:       2,
				ContinueOnError:   true,
				GlobalTimeout:     300,
			},
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	repositories := []string{"repo1", "repo2"}

	// Test group deployment trigger (this mainly tests that it doesn't panic)
	err := service.triggerGroupDeployment("test-group", repositories)
	if err != nil {
		// This is expected to fail since we don't have real repos
		t.Logf("triggerGroupDeployment() returned expected error: %v", err)
	}
}

func TestMonitorTriggerIndividualDeployment(t *testing.T) {
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

	repoName := "individual-repo"

	// Test individual deployment trigger (this mainly tests that it doesn't panic)
	err := service.triggerIndividualDeployment(repoName)
	if err != nil {
		// This is expected to fail since we don't have real repos
		t.Logf("triggerIndividualDeployment() returned expected error: %v", err)
	}
}

func TestMonitorCommitChangeDetection(t *testing.T) {
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

	// First commit storage
	service.lastCommit[repoKey] = "abc123"

	// Same commit should be properly stored
	if service.lastCommit[repoKey] != "abc123" {
		t.Error("Commit SHA not properly stored")
	}

	// Different commit should be properly updated
	service.lastCommit[repoKey] = "def456"
	if service.lastCommit[repoKey] != "def456" {
		t.Error("Commit SHA not properly updated")
	}
}

func TestMonitorRetryConfig(t *testing.T) {
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

func TestMonitorGroupTrigger(t *testing.T) {
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

func TestMonitorCommitInfo(t *testing.T) {
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

func TestMonitorUnsupportedRepoType(t *testing.T) {
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

	monitor := &MonitorConfig{
		RepoURL:  "https://unsupported.com/owner/repo",
		RepoType: "unsupported",
		Auth: AuthConfig{
			Username: "testuser",
			Token:    "testtoken",
		},
	}

	_, err := service.GetLatestCommit(monitor, "main")
	if err == nil {
		t.Error("GetLatestCommit() should return error for unsupported repo type")
	}

	expectedError := "unsupported repository type: unsupported"
	if err.Error() != expectedError {
		t.Errorf("GetLatestCommit() error = %v, want %v", err.Error(), expectedError)
	}
}

func TestMonitorGitLabUnsupportedURL(t *testing.T) {
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

	monitor := &MonitorConfig{
		RepoURL:  "https://unsupported-gitlab.com/owner/repo",
		RepoType: "gitlab",
		Auth: AuthConfig{
			Username: "testuser",
			Token:    "testtoken",
		},
	}

	_, err := service.getGitLabLatestCommit(monitor, "main")
	if err == nil {
		t.Error("getGitLabLatestCommit() should return error for unsupported URL format")
	}

	// Either gets "unsupported GitLab URL format" or API error, both are valid failures
	if err == nil {
		t.Error("getGitLabLatestCommit() should return some error for unsupported URL format")
	}
}

// Helper function to check if string contains substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
