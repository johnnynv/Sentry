package main

import (
	"os"
	"strings"
	"testing"
)

func TestExpandEnvVars(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("GITHUB_TOKEN", "github_token_123")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("GITHUB_TOKEN")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "expand ${VAR} format",
			input:    "token: ${GITHUB_TOKEN}",
			expected: "token: github_token_123",
		},
		{
			name:     "expand $VAR format",
			input:    "value: $TEST_VAR",
			expected: "value: test_value",
		},
		{
			name:     "no environment variables",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "missing variable",
			input:    "token: ${MISSING_VAR}",
			expected: "token: ",
		},
		{
			name:     "multiple variables",
			input:    "github: ${GITHUB_TOKEN}, test: $TEST_VAR",
			expected: "github: github_token_123, test: test_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("expandEnvVars() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateMonitorConfig(t *testing.T) {
	tests := []struct {
		name    string
		monitor MonitorConfig
		context string
		wantErr bool
	}{
		{
			name: "valid monitor config",
			monitor: MonitorConfig{
				RepoURL:  "https://github.com/owner/repo",
				Branches: []string{"main"},
				RepoType: "github",
				Auth: AuthConfig{
					Username: "user",
					Token:    "token",
				},
			},
			context: "test",
			wantErr: false,
		},
		{
			name: "empty repo URL",
			monitor: MonitorConfig{
				RepoURL:  "",
				Branches: []string{"main"},
				RepoType: "github",
				Auth: AuthConfig{
					Username: "user",
					Token:    "token",
				},
			},
			context: "test",
			wantErr: true,
		},
		{
			name: "no branches",
			monitor: MonitorConfig{
				RepoURL:  "https://github.com/owner/repo",
				Branches: []string{},
				RepoType: "github",
				Auth: AuthConfig{
					Username: "user",
					Token:    "token",
				},
			},
			context: "test",
			wantErr: true,
		},
		{
			name: "invalid repo type",
			monitor: MonitorConfig{
				RepoURL:  "https://github.com/owner/repo",
				Branches: []string{"main"},
				RepoType: "invalid",
				Auth: AuthConfig{
					Username: "user",
					Token:    "token",
				},
			},
			context: "test",
			wantErr: true,
		},
		{
			name: "empty token",
			monitor: MonitorConfig{
				RepoURL:  "https://github.com/owner/repo",
				Branches: []string{"main"},
				RepoType: "github",
				Auth: AuthConfig{
					Username: "user",
					Token:    "",
				},
			},
			context: "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMonitorConfig(&tt.monitor, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMonitorConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	validConfig := &Config{
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
				Name:  "test-repo",
				Group: "test-group",
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
			TmpDir:   "/tmp/test",
			Cleanup:  true,
			LogLevel: "info",
			Timeout:  300,
		},
	}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  validConfig,
			wantErr: false,
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
			wantErr: true,
		},
		{
			name: "no repositories",
			config: &Config{
				PollingInterval: 60,
				Repositories:    []RepositoryConfig{},
			},
			wantErr: true,
		},
		{
			name: "invalid group reference",
			config: &Config{
				PollingInterval: 60,
				Repositories: []RepositoryConfig{
					{
						Name:  "test-repo",
						Group: "non-existent-group",
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
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetConfigExample(t *testing.T) {
	example := GetConfigExample()
	if len(example) == 0 {
		t.Error("GetConfigExample() returned empty string")
	}

	// Check if example contains expected sections
	expectedSections := []string{"polling_interval", "groups", "repositories", "global"}
	for _, section := range expectedSections {
		if !strings.Contains(example, section) {
			t.Errorf("GetConfigExample() missing section: %s", section)
		}
	}
}

func TestIsValidK8sName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid lowercase",
			input:    "test",
			expected: true,
		},
		{
			name:     "valid with hyphens",
			input:    "test-name",
			expected: true,
		},
		{
			name:     "valid with numbers",
			input:    "test123",
			expected: true,
		},
		{
			name:     "invalid uppercase",
			input:    "Test",
			expected: false,
		},
		{
			name:     "invalid underscore",
			input:    "test_name",
			expected: false,
		},
		{
			name:     "invalid start with hyphen",
			input:    "-test",
			expected: false,
		},
		{
			name:     "invalid end with hyphen",
			input:    "test-",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidK8sName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidK8sName(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
