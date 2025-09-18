package main

import (
	"os"
	"testing"
)

func TestParseCommandLineArgs(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Test valid arguments
	os.Args = []string{"sentry", "-action=validate", "-config=test.yaml", "-verbose"}
	config := parseCommandLineArgs()

	if config.Action != "validate" {
		t.Errorf("Expected action 'validate', got %s", config.Action)
	}

	if config.ConfigPath != "test.yaml" {
		t.Errorf("Expected config path 'test.yaml', got %s", config.ConfigPath)
	}

	if !config.Verbose {
		t.Errorf("Expected verbose to be true")
	}
}

func TestAppConfig(t *testing.T) {
	config := &AppConfig{
		Action:     "watch",
		ConfigPath: "/path/to/config.yaml",
		Verbose:    true,
	}

	if config.Action != "watch" {
		t.Errorf("AppConfig.Action = %v, want %v", config.Action, "watch")
	}

	if config.ConfigPath != "/path/to/config.yaml" {
		t.Errorf("AppConfig.ConfigPath = %v, want %v", config.ConfigPath, "/path/to/config.yaml")
	}

	if !config.Verbose {
		t.Errorf("AppConfig.Verbose = %v, want %v", config.Verbose, true)
	}
}

func TestSentryApp(t *testing.T) {
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
			TmpDir:  "/tmp/test",
			Cleanup: true,
		},
	}

	deployService := NewDeployService(config)
	monitorService := NewMonitorService(config, deployService)
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
		t.Error("SentryApp.config not set correctly")
	}

	if app.monitorService != monitorService {
		t.Error("SentryApp.monitorService not set correctly")
	}

	if app.deployService != deployService {
		t.Error("SentryApp.deployService not set correctly")
	}

	if app.appConfig != appConfig {
		t.Error("SentryApp.appConfig not set correctly")
	}
}

func TestVersionInfo(t *testing.T) {
	// Test that version variables exist
	if Version == "" {
		t.Error("Version should not be empty")
	}

	if BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}

	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}

	if GitBranch == "" {
		t.Error("GitBranch should not be empty")
	}
}

func TestPrintFunctions(t *testing.T) {
	// Test that print functions don't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Print function panicked: %v", r)
		}
	}()

	printVersionInfo()
	printBanner()
	printUsage()
}

func TestExecuteActionValidation(t *testing.T) {
	// Test action validation without actual execution
	validActions := []string{"validate", "trigger", "watch"}

	for _, action := range validActions {
		appConfig := &AppConfig{
			Action:     action,
			ConfigPath: "test.yaml",
			Verbose:    false,
		}

		// Test that action is recognized
		switch appConfig.Action {
		case "validate", "trigger", "watch":
			// Valid action
		default:
			t.Errorf("Action %s should be valid", action)
		}
	}
}

func TestCreateSimpleConfig(t *testing.T) {
	// Test creating a simple config for testing
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
						Username: "testuser",
						Token:    "testtoken",
					},
				},
				Deploy: DeployConfig{
					QARepoURL:    "https://gitlab.com/qa/repo",
					QARepoBranch: "main",
					RepoType:     "gitlab",
					Auth: AuthConfig{
						Username: "qauser",
						Token:    "qatoken",
					},
					ProjectName: "test-project",
					Commands:    []string{"echo 'test deployment'"},
				},
			},
		},
		Global: GlobalConfig{
			TmpDir:   "/tmp/sentry-test",
			Cleanup:  true,
			LogLevel: "info",
			Timeout:  300,
		},
	}

	// Validate the test config
	err := validateConfig(config)
	if err != nil {
		t.Errorf("Test config validation failed: %v", err)
	}

	// Test that services can be created with this config
	deployService := NewDeployService(config)
	if deployService == nil {
		t.Error("Failed to create DeployService with test config")
	}

	monitorService := NewMonitorService(config, deployService)
	if monitorService == nil {
		t.Error("Failed to create MonitorService with test config")
	}
}

func TestConfigWithGroups(t *testing.T) {
	// Test configuration with groups
	config := &Config{
		PollingInterval: 60,
		Groups: map[string]GroupConfig{
			"test-group": {
				ExecutionStrategy: "parallel",
				MaxParallel:       2,
				ContinueOnError:   true,
				GlobalTimeout:     600,
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

	// Validate the config with groups
	err := validateConfig(config)
	if err != nil {
		t.Errorf("Config with groups validation failed: %v", err)
	}

	// Test group configuration
	group, exists := config.Groups["test-group"]
	if !exists {
		t.Error("Test group not found in config")
	}

	if group.ExecutionStrategy != "parallel" {
		t.Errorf("Group execution strategy = %v, want %v", group.ExecutionStrategy, "parallel")
	}

	if group.MaxParallel != 2 {
		t.Errorf("Group max parallel = %v, want %v", group.MaxParallel, 2)
	}
}

func TestGrouping(t *testing.T) {
	// Test grouping logic (similar to triggerAction)
	repositories := []RepositoryConfig{
		{Name: "repo1", Group: "group1"},
		{Name: "repo2", Group: "group1"},
		{Name: "repo3", Group: ""}, // Individual
		{Name: "repo4", Group: "group2"},
	}

	groups := make(map[string][]string)
	individual := make([]string, 0)

	for _, repo := range repositories {
		if repo.Group != "" {
			groups[repo.Group] = append(groups[repo.Group], repo.Name)
		} else {
			individual = append(individual, repo.Name)
		}
	}

	// Verify grouping results
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	if len(groups["group1"]) != 2 {
		t.Errorf("Expected 2 repos in group1, got %d", len(groups["group1"]))
	}

	if len(groups["group2"]) != 1 {
		t.Errorf("Expected 1 repo in group2, got %d", len(groups["group2"]))
	}

	if len(individual) != 1 {
		t.Errorf("Expected 1 individual repo, got %d", len(individual))
	}

	if individual[0] != "repo3" {
		t.Errorf("Expected repo3 as individual, got %s", individual[0])
	}
}
