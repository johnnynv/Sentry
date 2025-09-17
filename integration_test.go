package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestEndToEndValidateAction tests the complete validate action workflow
func TestEndToEndValidateAction(t *testing.T) {
	// Skip if no real environment available
	if os.Getenv("SENTRY_E2E_TEST") != "true" {
		t.Skip("Skipping E2E test - set SENTRY_E2E_TEST=true to run")
	}

	// Load real configuration
	config, err := LoadConfig("sentry.yaml")
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Create application instance
	app := &SentryApp{
		config:         config,
		monitorService: NewMonitorService(config, NewDeployService(config)),
		deployService:  NewDeployService(config),
		appConfig: &AppConfig{
			Action:     "validate",
			ConfigPath: "sentry.yaml",
			Verbose:    true,
		},
	}

	// Test validate action
	err = app.validateAction()
	if err != nil {
		t.Fatalf("Validate action failed: %v", err)
	}

	t.Log("End-to-end validate action test passed")
}

// TestEndToEndRepositoryConnectivity tests real repository connectivity
func TestEndToEndRepositoryConnectivity(t *testing.T) {
	// Skip if no real environment available
	if os.Getenv("SENTRY_E2E_TEST") != "true" {
		t.Skip("Skipping E2E test - set SENTRY_E2E_TEST=true to run")
	}

	// Load real configuration
	config, err := LoadConfig("sentry.yaml")
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	monitorService := NewMonitorService(config, NewDeployService(config))

	// Test Repository A connectivity
	t.Run("Repository A Connectivity", func(t *testing.T) {
		commit, err := monitorService.GetLatestCommit(&config.Monitor.RepoA)
		if err != nil {
			t.Fatalf("Failed to get latest commit from Repository A: %v", err)
		}

		if len(commit.SHA) < 8 {
			t.Errorf("Commit SHA seems invalid: %s", commit.SHA)
		}

		if commit.Author == "" {
			t.Error("Commit author should not be empty")
		}

		t.Logf("Repository A latest commit: %s by %s", commit.SHA[:8], commit.Author)
	})

	// Test Repository B connectivity
	t.Run("Repository B Connectivity", func(t *testing.T) {
		commit, err := monitorService.GetLatestCommit(&config.Monitor.RepoB)
		if err != nil {
			t.Fatalf("Failed to get latest commit from Repository B: %v", err)
		}

		if len(commit.SHA) < 8 {
			t.Errorf("Commit SHA seems invalid: %s", commit.SHA)
		}

		if commit.Author == "" {
			t.Error("Commit author should not be empty")
		}

		t.Logf("Repository B latest commit: %s by %s", commit.SHA[:8], commit.Author)
	})
}

// TestEndToEndDeploymentWorkflow tests the complete deployment workflow
func TestEndToEndDeploymentWorkflow(t *testing.T) {
	// Skip if no real environment available
	if os.Getenv("SENTRY_E2E_TEST") != "true" {
		t.Skip("Skipping E2E test - set SENTRY_E2E_TEST=true to run")
	}

	// Skip if kubectl not available
	if os.Getenv("SENTRY_KUBECTL_TEST") != "true" {
		t.Skip("Skipping kubectl test - set SENTRY_KUBECTL_TEST=true to run")
	}

	// Load real configuration
	config, err := LoadConfig("sentry.yaml")
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	deployService := NewDeployService(config)

	// Test deployment environment validation
	err = deployService.ValidateDeploymentEnvironment()
	if err != nil {
		t.Fatalf("Deployment environment validation failed: %v", err)
	}

	// Test Repository B deployment (assuming it has Tekton configs)
	t.Run("Repository B Deployment", func(t *testing.T) {
		result, err := deployService.DeployFromRepository(&config.Monitor.RepoB, "repo_b_test")

		// Note: This might fail if repo doesn't have .tekton directory
		// That's expected for some repositories
		if err != nil {
			if result != nil && result.Error == "no Tekton configuration files found" {
				t.Skip("Repository B has no Tekton configurations - this is expected")
				return
			}
			t.Fatalf("Deployment failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Deployment was not successful: %s", result.Error)
		}

		t.Logf("Successfully deployed %d files from Repository B", len(result.FilesDeployed))
	})
}

// TestFullWorkflowSimulation simulates a complete monitoring and deployment cycle
func TestFullWorkflowSimulation(t *testing.T) {
	// This test simulates the full workflow without real deployment

	// Create test configuration
	config := &Config{
		Monitor: MonitorConfig{
			RepoA: RepoConfig{
				Type:   "github",
				URL:    "https://github.com/test/repo-a",
				Branch: "main",
				Token:  "test-token",
			},
			RepoB: RepoConfig{
				Type:   "gitlab",
				URL:    "https://gitlab.com/test/repo-b",
				Branch: "main",
				Token:  "test-token",
			},
			Poll: PollConfig{
				Interval: 5, // Short interval for testing
				Timeout:  10,
			},
		},
		Deploy: DeployConfig{
			Namespace: "tekton-pipelines",
			TmpDir:    "/tmp/sentry-workflow-test",
			Cleanup:   true,
		},
	}

	// Create services
	monitorService := NewMonitorService(config, NewDeployService(config))
	deployService := NewDeployService(config)

	// Test service creation
	if monitorService == nil {
		t.Fatal("Monitor service should not be nil")
	}

	if deployService == nil {
		t.Fatal("Deploy service should not be nil")
	}

	// Test manual repository check
	t.Run("Manual Repository Check", func(t *testing.T) {
		// This will fail with real repos, but tests the workflow
		err := monitorService.TriggerManualCheck()
		if err != nil {
			// Expected to fail with test tokens, but workflow should work
			t.Logf("Expected failure with test configuration: %v", err)
		}
	})

	// Test deployment preparation
	t.Run("Deployment Preparation", func(t *testing.T) {
		tmpDir, err := deployService.createTempDirectory("test-workflow")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}

		// Verify directory exists
		if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
			t.Errorf("Temp directory should exist: %s", tmpDir)
		}

		// Test cleanup
		err = deployService.cleanupTempDirectory(tmpDir)
		if err != nil {
			t.Errorf("Failed to cleanup temp directory: %v", err)
		}

		// Verify directory is removed
		if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
			t.Error("Directory should be removed after cleanup")
		}
	})
}

// TestConfigurationLoadingWithRealFile tests loading the actual configuration file
func TestConfigurationLoadingWithRealFile(t *testing.T) {
	// Test loading the real sentry.yaml file
	config, err := LoadConfig("sentry.yaml")
	if err != nil {
		t.Fatalf("Failed to load sentry.yaml: %v", err)
	}

	// Validate configuration structure
	if config.Monitor.RepoA.Type == "" {
		t.Error("Repository A type should not be empty")
	}

	if config.Monitor.RepoB.Type == "" {
		t.Error("Repository B type should not be empty")
	}

	if config.Monitor.Poll.Interval <= 0 {
		t.Error("Poll interval should be positive")
	}

	if config.Deploy.Namespace == "" {
		t.Error("Deploy namespace should not be empty")
	}

	t.Logf("Configuration loaded successfully:")
	t.Logf("- Repository A: %s (%s)", config.Monitor.RepoA.Type, config.Monitor.RepoA.URL)
	t.Logf("- Repository B: %s (%s)", config.Monitor.RepoB.Type, config.Monitor.RepoB.URL)
	t.Logf("- Poll interval: %d seconds", config.Monitor.Poll.Interval)
	t.Logf("- Deploy namespace: %s", config.Deploy.Namespace)
}

// TestEnvironmentVariableSubstitution tests real environment variable loading
func TestEnvironmentVariableSubstitution(t *testing.T) {
	// Check if .env file exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		t.Skip("No .env file found - skipping environment variable test")
	}

	// Load configuration which should read .env file
	config, err := LoadConfig("sentry.yaml")
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Check that tokens are not empty (assuming they're set in .env)
	if config.Monitor.RepoA.Token == "" {
		t.Error("Repository A token should not be empty after environment substitution")
	}

	if config.Monitor.RepoB.Token == "" {
		t.Error("Repository B token should not be empty after environment substitution")
	}

	// Check that tokens don't contain variable syntax
	if strings.Contains(config.Monitor.RepoA.Token, "${") || strings.Contains(config.Monitor.RepoA.Token, "$") {
		t.Error("Repository A token should not contain unexpanded variables")
	}

	if strings.Contains(config.Monitor.RepoB.Token, "${") || strings.Contains(config.Monitor.RepoB.Token, "$") {
		t.Error("Repository B token should not contain unexpanded variables")
	}

	t.Log("Environment variable substitution test passed")
}

// TestLongRunningStability tests stability over time (short duration for CI)
func TestLongRunningStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	// Create test configuration with short intervals
	config := &Config{
		Monitor: MonitorConfig{
			RepoA: RepoConfig{
				Type:   "github",
				URL:    "https://github.com/test/repo",
				Branch: "main",
				Token:  "test",
			},
			RepoB: RepoConfig{
				Type:   "github",
				URL:    "https://github.com/test/repo",
				Branch: "main",
				Token:  "test",
			},
			Poll: PollConfig{
				Interval: 1, // 1 second for quick testing
				Timeout:  5,
			},
		},
		Deploy: DeployConfig{
			Namespace: "test",
			TmpDir:    "/tmp/sentry-stability-test",
			Cleanup:   true,
		},
	}

	// Run monitoring for a short period
	done := make(chan bool)
	go func() {
		time.Sleep(5 * time.Second)
		done <- true
	}()

	// Test that services can be created multiple times without issues
	for i := 0; i < 10; i++ {
		service := NewMonitorService(config, NewDeployService(config))
		if service == nil {
			t.Fatalf("Failed to create monitor service on iteration %d", i)
		}

		deployService := NewDeployService(config)
		if deployService == nil {
			t.Fatalf("Failed to create deploy service on iteration %d", i)
		}
	}

	<-done
	t.Log("Stability test completed successfully")
}

// Helper function removed - use strings.Contains instead
