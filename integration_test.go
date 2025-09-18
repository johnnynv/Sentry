package main

import (
	"os"
	"testing"
)

// TestBasicConfigurationLoading tests basic configuration loading functionality
func TestBasicConfigurationLoading(t *testing.T) {
	// Create a temporary config file
	configContent := `
polling_interval: 60
repositories:
  - name: "test-repo"
    monitor:
      repo_url: "https://github.com/test/repo"
      branches: ["main"]
      repo_type: "github"
      auth:
        username: "testuser"
        token: "testtoken"
    deploy:
      qa_repo_url: "https://gitlab.com/qa/repo"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      auth:
        username: "qauser"
        token: "qatoken"
      project_name: "test-project"
      commands:
        - "echo test"
global:
  tmp_dir: "/tmp/sentry-test"
  cleanup: true
`

	tmpFile, err := os.CreateTemp("", "sentry-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}
	tmpFile.Close()

	// Load the configuration
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify basic configuration values
	if config.PollingInterval != 60 {
		t.Errorf("Expected polling interval 60, got %d", config.PollingInterval)
	}

	if len(config.Repositories) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(config.Repositories))
	}

	repo := config.Repositories[0]
	if repo.Name != "test-repo" {
		t.Errorf("Expected repo name 'test-repo', got %s", repo.Name)
	}

	if repo.Monitor.RepoURL != "https://github.com/test/repo" {
		t.Errorf("Expected monitor repo URL 'https://github.com/test/repo', got %s", repo.Monitor.RepoURL)
	}
}

// TestServiceInitialization tests that services can be initialized with valid config
func TestServiceInitialization(t *testing.T) {
	config := &Config{
		PollingInterval: 60,
		Repositories: []RepositoryConfig{
			{
				Name: "test-repo",
				Monitor: MonitorConfig{
					RepoURL:  "https://github.com/test/repo",
					Branches: []string{"main"},
					RepoType: "github",
					Auth: AuthConfig{
						Username: "user",
						Token:    "token",
					},
				},
				Deploy: DeployConfig{
					QARepoURL:    "https://gitlab.com/qa/repo",
					QARepoBranch: "main",
					RepoType:     "gitlab",
					Auth: AuthConfig{
						Username: "user",
						Token:    "token",
					},
					ProjectName: "test",
					Commands:    []string{"echo test"},
				},
			},
		},
		Global: GlobalConfig{
			TmpDir:  "/tmp/sentry-test",
			Cleanup: true,
		},
	}

	// Test that services can be created
	deployService := NewDeployService(config)
	if deployService == nil {
		t.Error("Failed to create DeployService")
	}

	monitorService := NewMonitorService(config, deployService)
	if monitorService == nil {
		t.Error("Failed to create MonitorService")
	}

	// Test that SentryApp can be created
	appConfig := &AppConfig{
		Action:     "validate",
		ConfigPath: "test.yaml",
		Verbose:    false,
	}

	app := &SentryApp{
		config:         config,
		monitorService: monitorService,
		deployService:  deployService,
		appConfig:      appConfig,
	}

	if app.config != config {
		t.Error("SentryApp config not set correctly")
	}
}

// TestEnvironmentVariableExpansion tests environment variable expansion
func TestEnvironmentVariableExpansion(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_GITHUB_TOKEN", "github_token_123")
	os.Setenv("TEST_GITLAB_TOKEN", "gitlab_token_456")
	defer os.Unsetenv("TEST_GITHUB_TOKEN")
	defer os.Unsetenv("TEST_GITLAB_TOKEN")

	configContent := `
polling_interval: 60
repositories:
  - name: "test-repo"
    monitor:
      repo_url: "https://github.com/test/repo"
      branches: ["main"]
      repo_type: "github"
      auth:
        username: "testuser"
        token: "${TEST_GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab.com/qa/repo"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      auth:
        username: "qauser"
        token: "${TEST_GITLAB_TOKEN}"
      project_name: "test-project"
      commands:
        - "echo test"
global:
  tmp_dir: "/tmp/sentry-test"
  cleanup: true
`

	tmpFile, err := os.CreateTemp("", "sentry-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}
	tmpFile.Close()

	// Load the configuration
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variables were expanded
	repo := config.Repositories[0]
	if repo.Monitor.Auth.Token != "github_token_123" {
		t.Errorf("Expected github token 'github_token_123', got %s", repo.Monitor.Auth.Token)
	}

	if repo.Deploy.Auth.Token != "gitlab_token_456" {
		t.Errorf("Expected gitlab token 'gitlab_token_456', got %s", repo.Deploy.Auth.Token)
	}
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				PollingInterval: 60,
				Repositories: []RepositoryConfig{
					{
						Name: "test-repo",
						Monitor: MonitorConfig{
							RepoURL:  "https://github.com/test/repo",
							Branches: []string{"main"},
							RepoType: "github",
							Auth: AuthConfig{
								Username: "user",
								Token:    "token",
							},
						},
						Deploy: DeployConfig{
							QARepoURL:    "https://gitlab.com/qa/repo",
							QARepoBranch: "main",
							RepoType:     "gitlab",
							Auth: AuthConfig{
								Username: "user",
								Token:    "token",
							},
							ProjectName: "test",
							Commands:    []string{"echo test"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid polling interval",
			config: &Config{
				PollingInterval: 30, // Too low
				Repositories: []RepositoryConfig{
					{
						Name: "test-repo",
						Monitor: MonitorConfig{
							RepoURL:  "https://github.com/test/repo",
							Branches: []string{"main"},
							RepoType: "github",
							Auth: AuthConfig{
								Username: "user",
								Token:    "token",
							},
						},
						Deploy: DeployConfig{
							QARepoURL:    "https://gitlab.com/qa/repo",
							QARepoBranch: "main",
							RepoType:     "gitlab",
							Auth: AuthConfig{
								Username: "user",
								Token:    "token",
							},
							ProjectName: "test",
							Commands:    []string{"echo test"},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "no repositories",
			config: &Config{
				PollingInterval: 60,
				Repositories:    []RepositoryConfig{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.expectError {
				t.Errorf("validateConfig() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

// TestGroupConfiguration tests group configuration functionality
func TestGroupConfiguration(t *testing.T) {
	config := &Config{
		PollingInterval: 60,
		Groups: map[string]GroupConfig{
			"test-group": {
				ExecutionStrategy: "parallel",
				MaxParallel:       3,
				ContinueOnError:   true,
				GlobalTimeout:     900,
			},
		},
		Repositories: []RepositoryConfig{
			{
				Name:  "repo1",
				Group: "test-group",
				Monitor: MonitorConfig{
					RepoURL:  "https://github.com/test/repo1",
					Branches: []string{"main"},
					RepoType: "github",
					Auth: AuthConfig{
						Username: "user",
						Token:    "token",
					},
				},
				Deploy: DeployConfig{
					QARepoURL:    "https://gitlab.com/qa/repo",
					QARepoBranch: "main",
					RepoType:     "gitlab",
					Auth: AuthConfig{
						Username: "user",
						Token:    "token",
					},
					ProjectName: "project1",
					Commands:    []string{"echo 'deploy repo1'"},
				},
			},
			{
				Name:  "repo2",
				Group: "test-group",
				Monitor: MonitorConfig{
					RepoURL:  "https://github.com/test/repo2",
					Branches: []string{"main"},
					RepoType: "github",
					Auth: AuthConfig{
						Username: "user",
						Token:    "token",
					},
				},
				Deploy: DeployConfig{
					QARepoURL:    "https://gitlab.com/qa/repo",
					QARepoBranch: "main",
					RepoType:     "gitlab",
					Auth: AuthConfig{
						Username: "user",
						Token:    "token",
					},
					ProjectName: "project2",
					Commands:    []string{"echo 'deploy repo2'"},
				},
			},
		},
		Global: GlobalConfig{
			TmpDir:  "/tmp/sentry-test",
			Cleanup: true,
		},
	}

	// Validate config with groups
	err := validateConfig(config)
	if err != nil {
		t.Errorf("Group config validation failed: %v", err)
	}

	// Test group configuration
	group, exists := config.Groups["test-group"]
	if !exists {
		t.Error("Test group not found")
	}

	if group.ExecutionStrategy != "parallel" {
		t.Errorf("Expected parallel strategy, got %s", group.ExecutionStrategy)
	}

	if group.MaxParallel != 3 {
		t.Errorf("Expected max parallel 3, got %d", group.MaxParallel)
	}

	// Test that both repositories belong to the group
	groupRepos := 0
	for _, repo := range config.Repositories {
		if repo.Group == "test-group" {
			groupRepos++
		}
	}

	if groupRepos != 2 {
		t.Errorf("Expected 2 repositories in group, got %d", groupRepos)
	}
}

// Integration tests completed - duplicate structure tests are in individual test files
