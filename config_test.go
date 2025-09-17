package main

import (
	"os"
	"testing"
)

func TestExpandEnvVars(t *testing.T) {
	// 设置测试环境变量
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
				t.Errorf("expandEnvVars() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateRepoConfig(t *testing.T) {
	tests := []struct {
		name      string
		repo      RepoConfig
		repoName  string
		wantError bool
	}{
		{
			name: "valid github config",
			repo: RepoConfig{
				Type:   "github",
				URL:    "https://github.com/owner/repo",
				Branch: "main",
				Token:  "token123",
			},
			repoName:  "test_repo",
			wantError: false,
		},
		{
			name: "valid gitlab config",
			repo: RepoConfig{
				Type:   "gitlab",
				URL:    "https://gitlab.com/owner/repo",
				Branch: "develop",
				Token:  "token456",
			},
			repoName:  "test_repo",
			wantError: false,
		},
		{
			name: "invalid type",
			repo: RepoConfig{
				Type:   "bitbucket",
				URL:    "https://bitbucket.org/owner/repo",
				Branch: "main",
				Token:  "token123",
			},
			repoName:  "test_repo",
			wantError: true,
		},
		{
			name: "empty URL",
			repo: RepoConfig{
				Type:   "github",
				URL:    "",
				Branch: "main",
				Token:  "token123",
			},
			repoName:  "test_repo",
			wantError: true,
		},
		{
			name: "empty branch",
			repo: RepoConfig{
				Type:   "github",
				URL:    "https://github.com/owner/repo",
				Branch: "",
				Token:  "token123",
			},
			repoName:  "test_repo",
			wantError: true,
		},
		{
			name: "empty token",
			repo: RepoConfig{
				Type:   "github",
				URL:    "https://github.com/owner/repo",
				Branch: "main",
				Token:  "",
			},
			repoName:  "test_repo",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepoConfig(&tt.repo, tt.repoName)
			if (err != nil) != tt.wantError {
				t.Errorf("validateRepoConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	validConfig := &Config{
		Monitor: MonitorConfig{
			RepoA: RepoConfig{
				Type:   "github",
				URL:    "https://github.com/owner/repo-a",
				Branch: "main",
				Token:  "token123",
			},
			RepoB: RepoConfig{
				Type:   "gitlab",
				URL:    "https://gitlab.com/owner/repo-b",
				Branch: "main",
				Token:  "token456",
			},
			Poll: PollConfig{
				Interval: 30,
				Timeout:  10,
			},
		},
		Deploy: DeployConfig{
			Namespace: "tekton-pipelines",
			TmpDir:    "/tmp/sentry",
			Cleanup:   true,
		},
	}

	t.Run("valid config", func(t *testing.T) {
		err := validateConfig(validConfig)
		if err != nil {
			t.Errorf("validateConfig() with valid config should not error, got: %v", err)
		}
	})

	t.Run("invalid poll interval", func(t *testing.T) {
		config := *validConfig
		config.Monitor.Poll.Interval = 0
		err := validateConfig(&config)
		if err == nil {
			t.Error("validateConfig() with zero interval should error")
		}
	})

	t.Run("invalid poll timeout", func(t *testing.T) {
		config := *validConfig
		config.Monitor.Poll.Timeout = -1
		err := validateConfig(&config)
		if err == nil {
			t.Error("validateConfig() with negative timeout should error")
		}
	})

	t.Run("empty namespace", func(t *testing.T) {
		config := *validConfig
		config.Deploy.Namespace = ""
		err := validateConfig(&config)
		if err == nil {
			t.Error("validateConfig() with empty namespace should error")
		}
	})

	t.Run("empty tmp_dir", func(t *testing.T) {
		config := *validConfig
		config.Deploy.TmpDir = ""
		err := validateConfig(&config)
		if err == nil {
			t.Error("validateConfig() with empty tmp_dir should error")
		}
	})
}

func TestGetConfigExample(t *testing.T) {
	example := GetConfigExample()
	if len(example) == 0 {
		t.Error("GetConfigExample() should return non-empty string")
	}
	
	// 检查示例是否包含关键配置项
	expectedItems := []string{
		"monitor:",
		"repo_a:",
		"repo_b:",
		"deploy:",
		"namespace:",
		"tmp_dir:",
	}
	
	for _, item := range expectedItems {
		if !contains(example, item) {
			t.Errorf("GetConfigExample() should contain %q", item)
		}
	}
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || len(s) > len(substr) && 
		    (s[:len(substr)] == substr || 
		     s[len(s)-len(substr):] == substr || 
		     indexOf(s, substr) >= 0))
}

// indexOf 查找子字符串的位置
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
