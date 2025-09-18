package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewDeployService(t *testing.T) {
	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test",
			Cleanup: true,
		},
	}

	service := NewDeployService(config)
	if service == nil {
		t.Error("NewDeployService() returned nil")
		return
	}

	if service.config != config {
		t.Error("NewDeployService() did not set config correctly")
	}
}

func TestCreateTempDirectory(t *testing.T) {
	config := &Config{
		Global: GlobalConfig{
			TmpDir: "/tmp/test-sentry",
		},
	}

	service := NewDeployService(config)

	tmpDir, err := service.createTempDirectory("test-repo")
	if err != nil {
		t.Errorf("createTempDirectory() error = %v", err)
		return
	}

	// Check if directory exists
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("createTempDirectory() created directory does not exist: %s", tmpDir)
	}

	// Verify directory pattern
	expectedPattern := "/tmp/test-sentry/sentry-test-repo-"
	if !strings.HasPrefix(tmpDir, expectedPattern) {
		t.Errorf("createTempDirectory() directory path = %v, should start with %v", tmpDir, expectedPattern)
	}

	// Clean up
	os.RemoveAll(tmpDir)
}

func TestCleanupTempDirectory(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir: "/tmp/test-sentry",
		},
	}

	service := NewDeployService(config)

	// Create a temporary directory
	tmpDir, err := service.createTempDirectory("test-repo")
	if err != nil {
		t.Errorf("createTempDirectory() error = %v", err)
		return
	}

	// Verify it exists
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("Temporary directory was not created: %s", tmpDir)
		return
	}

	// Clean it up
	err = service.cleanupTempDirectory(tmpDir)
	if err != nil {
		t.Errorf("cleanupTempDirectory() error = %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("cleanupTempDirectory() did not remove directory: %s", tmpDir)
	}
}

func TestCleanupNonExistentDirectory(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir: "/tmp/test-sentry",
		},
	}

	service := NewDeployService(config)

	// Try to clean up a non-existent directory
	err := service.cleanupTempDirectory("/tmp/non-existent-directory")
	if err != nil {
		t.Errorf("cleanupTempDirectory() should not error on non-existent directory, got: %v", err)
	}
}

func TestGetTempDir(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "with configured temp dir",
			config: &Config{
				Global: GlobalConfig{
					TmpDir: "/custom/temp",
				},
			},
			expected: "/custom/temp",
		},
		{
			name: "without configured temp dir",
			config: &Config{
				Global: GlobalConfig{},
			},
			expected: "/tmp/sentry",
		},
		{
			name: "empty temp dir",
			config: &Config{
				Global: GlobalConfig{
					TmpDir: "",
				},
			},
			expected: "/tmp/sentry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDeployService(tt.config)
			result := service.getTempDir()
			if result != tt.expected {
				t.Errorf("getTempDir() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestShouldCleanup(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "cleanup enabled",
			config: &Config{
				Global: GlobalConfig{
					Cleanup: true,
				},
			},
			expected: true,
		},
		{
			name: "cleanup disabled",
			config: &Config{
				Global: GlobalConfig{
					Cleanup: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDeployService(tt.config)
			result := service.shouldCleanup()
			if result != tt.expected {
				t.Errorf("shouldCleanup() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDeployIndividual(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
			Timeout: 30,
		},
		Repositories: []RepositoryConfig{
			{
				Name: "test-repo",
				Deploy: DeployConfig{
					QARepoURL:    "https://github.com/owner/qa-repo",
					QARepoBranch: "main",
					RepoType:     "github",
					ProjectName:  "test-project",
					Commands:     []string{"echo 'test command'"},
					Auth: AuthConfig{
						Username: "testuser",
						Token:    "testtoken",
					},
				},
			},
		},
	}

	service := NewDeployService(config)

	// Test with existing repository config (will fail due to invalid URL but tests the flow)
	err := service.DeployIndividual(&config.Repositories[0])
	if err == nil {
		t.Error("DeployIndividual() should fail for invalid repository URL")
	}

	expectedError := "deployment failed"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("DeployIndividual() error should contain '%v', got: %v", expectedError, err.Error())
	}
}

func TestDeployRepository(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: false, // Don't cleanup for test inspection
			Timeout: 5,
		},
		Repositories: []RepositoryConfig{
			{
				Name: "test-repo",
				Deploy: DeployConfig{
					QARepoURL:    "https://invalid-url-that-does-not-exist.com/repo",
					QARepoBranch: "main",
					RepoType:     "github",
					ProjectName:  "test-project",
					Commands:     []string{"echo 'test'"},
					Auth: AuthConfig{
						Username: "testuser",
						Token:    "testtoken",
					},
				},
			},
		},
	}

	service := NewDeployService(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with invalid repository URL (should fail)
	result := service.deployRepository("test-repo", ctx)
	if result.Success {
		t.Error("deployRepository() should fail for invalid repository URL")
	}

	if result.Error == "" {
		t.Error("deployRepository() should set error message for failure")
	}

	if result.RepoName != "test-repo" {
		t.Errorf("deployRepository() RepoName = %v, want %v", result.RepoName, "test-repo")
	}

	// Cleanup if directory was created
	if result.ClonePath != "" {
		os.RemoveAll(result.ClonePath)
	}
}

func TestDeployRepositoryNotFound(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
			Timeout: 5,
		},
		Repositories: []RepositoryConfig{}, // Empty repositories
	}

	service := NewDeployService(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with non-existent repository
	result := service.deployRepository("non-existent-repo", ctx)
	if result.Success {
		t.Error("deployRepository() should fail for non-existent repository")
	}

	expectedError := "repository configuration not found"
	if !strings.Contains(result.Error, expectedError) {
		t.Errorf("deployRepository() error should contain '%v', got: %v", expectedError, result.Error)
	}
}

func TestDeployGroup(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
			Timeout: 5,
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

	service := NewDeployService(config)

	groupConfig := config.Groups["test-group"]
	repoNames := []string{"repo1", "repo2"}

	// Test group deployment (should fail due to invalid repos but test the flow)
	err := service.DeployGroup("test-group", repoNames, &groupConfig)

	// Should return error due to repository not found or deployment failures
	if err == nil {
		t.Error("DeployGroup() should fail due to repository configuration not found")
	}

	expectedError := "deployment failures"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("DeployGroup() error should contain '%v', got: %v", expectedError, err.Error())
	}
}

func TestDeployGroupSequential(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
			Timeout: 5,
		},
		Groups: map[string]GroupConfig{
			"sequential-group": {
				ExecutionStrategy: "sequential",
				MaxParallel:       1,
				ContinueOnError:   false,
				GlobalTimeout:     300,
			},
		},
	}

	service := NewDeployService(config)

	groupConfig := config.Groups["sequential-group"]
	repoNames := []string{"seq-repo1"}

	// Test sequential group deployment
	err := service.DeployGroup("sequential-group", repoNames, &groupConfig)

	// Should fail due to repository not found
	if err == nil {
		t.Error("DeployGroup() should fail due to repository configuration not found")
	}
}

func TestDeployGroupErrorHandling(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
			Timeout: 5,
		},
		Groups: map[string]GroupConfig{
			"error-group": {
				ExecutionStrategy: "sequential",
				MaxParallel:       1,
				ContinueOnError:   false, // Stop on first error
				GlobalTimeout:     300,
			},
		},
	}

	service := NewDeployService(config)

	groupConfig := config.Groups["error-group"]
	repoNames := []string{"error-repo1"}

	// Test sequential deployment with stop-on-error
	err := service.DeployGroup("error-group", repoNames, &groupConfig)

	// Should fail due to repository not found
	if err == nil {
		t.Error("DeployGroup() should fail due to repository configuration not found")
	}
}

func TestDeployResult(t *testing.T) {
	result := &DeployResult{
		RepoName:    "test-repo",
		ClonePath:   "/tmp/test",
		CommandsRun: []string{"echo test"},
		Success:     true,
		Duration:    "1.5s",
	}

	if result.RepoName != "test-repo" {
		t.Errorf("DeployResult.RepoName = %v, want %v", result.RepoName, "test-repo")
	}

	if result.ClonePath != "/tmp/test" {
		t.Errorf("DeployResult.ClonePath = %v, want %v", result.ClonePath, "/tmp/test")
	}

	if len(result.CommandsRun) != 1 || result.CommandsRun[0] != "echo test" {
		t.Errorf("DeployResult.CommandsRun = %v, want %v", result.CommandsRun, []string{"echo test"})
	}

	if !result.Success {
		t.Errorf("DeployResult.Success = %v, want %v", result.Success, true)
	}

	if result.Duration != "1.5s" {
		t.Errorf("DeployResult.Duration = %v, want %v", result.Duration, "1.5s")
	}
}

func TestGroupDeployResult(t *testing.T) {
	result := &GroupDeployResult{
		GroupName: "test-group",
		Results: map[string]*DeployResult{
			"repo1": {
				RepoName: "repo1",
				Success:  true,
			},
		},
		Success:   true,
		TotalTime: "2.5s",
		Strategy:  "parallel",
	}

	if result.GroupName != "test-group" {
		t.Errorf("GroupDeployResult.GroupName = %v, want %v", result.GroupName, "test-group")
	}

	if len(result.Results) != 1 {
		t.Errorf("GroupDeployResult.Results length = %v, want %v", len(result.Results), 1)
	}

	if !result.Success {
		t.Errorf("GroupDeployResult.Success = %v, want %v", result.Success, true)
	}

	if result.Strategy != "parallel" {
		t.Errorf("GroupDeployResult.Strategy = %v, want %v", result.Strategy, "parallel")
	}
}

func TestDeployIndividualValidation(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
		},
		Repositories: []RepositoryConfig{
			{
				Name: "valid-repo",
				Deploy: DeployConfig{
					QARepoURL:    "https://github.com/owner/qa-repo",
					QARepoBranch: "main",
					RepoType:     "github",
					ProjectName:  "", // Empty project name should cause validation error
					Commands:     []string{"echo 'test'"},
					Auth: AuthConfig{
						Username: "testuser",
						Token:    "testtoken",
					},
				},
			},
		},
	}

	service := NewDeployService(config)

	// Test with repository that has empty project name
	err := service.DeployIndividual(&config.Repositories[0])

	// Should fail due to validation issues or cloning issues
	if err == nil {
		t.Error("DeployIndividual() should fail for repository with empty project name or invalid URL")
	}
}

// Test additional edge cases
func TestDeployWithRealCommands(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
			Timeout: 10,
		},
		Repositories: []RepositoryConfig{
			{
				Name: "echo-repo",
				Deploy: DeployConfig{
					QARepoURL:    "https://invalid-for-clone.com/repo.git",
					QARepoBranch: "main",
					RepoType:     "github",
					ProjectName:  "echo-project",
					Commands:     []string{"echo 'Hello World'"},
					Auth: AuthConfig{
						Username: "testuser",
						Token:    "testtoken",
					},
				},
			},
		},
	}

	service := NewDeployService(config)

	// Test with echo command (will fail at clone stage but tests command setup)
	err := service.DeployIndividual(&config.Repositories[0])
	if err == nil {
		t.Error("DeployIndividual() should fail due to invalid clone URL")
	}

	// Error should be related to cloning, not command execution
	if !strings.Contains(err.Error(), "deployment failed") {
		t.Errorf("DeployIndividual() should fail with deployment error, got: %v", err.Error())
	}
}

func TestDeployServiceTimeout(t *testing.T) {
	// Initialize logger for test
	InitializeLogger(false)

	config := &Config{
		Global: GlobalConfig{
			TmpDir:  "/tmp/test-sentry",
			Cleanup: true,
			Timeout: 1, // Very short timeout
		},
		Repositories: []RepositoryConfig{
			{
				Name: "timeout-repo",
				Deploy: DeployConfig{
					QARepoURL:    "https://github.com/owner/qa-repo",
					QARepoBranch: "main",
					RepoType:     "github",
					ProjectName:  "timeout-project",
					Commands:     []string{"sleep 5"}, // Command that takes longer than timeout
					Auth: AuthConfig{
						Username: "testuser",
						Token:    "testtoken",
					},
				},
			},
		},
	}

	service := NewDeployService(config)

	// Test with timeout (will fail due to invalid URL before reaching command timeout)
	err := service.DeployIndividual(&config.Repositories[0])
	if err == nil {
		t.Error("DeployIndividual() should fail due to timeout or invalid URL")
	}
}
