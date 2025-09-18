package main

import (
	"os"
	"testing"
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
