// Package main implements Sentry - Tekton Pipeline Auto-Deployer
//
// Sentry monitors Git repositories for changes and automatically deploys
// Tekton configurations to Kubernetes clusters. It supports both GitHub
// and GitLab repositories with automatic change detection and deployment.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the complete Sentry configuration
type Config struct {
	Monitor MonitorConfig `yaml:"monitor"`
	Deploy  DeployConfig  `yaml:"deploy"`
}

// MonitorConfig monitoring configuration
type MonitorConfig struct {
	RepoA RepoConfig `yaml:"repo_a"`
	RepoB RepoConfig `yaml:"repo_b"`
	Poll  PollConfig `yaml:"poll"`
}

// RepoConfig repository configuration
type RepoConfig struct {
	Type   string `yaml:"type"`   // "github" or "gitlab"
	URL    string `yaml:"url"`    // Repository URL
	Branch string `yaml:"branch"` // Branch to monitor
	Token  string `yaml:"token"`  // Access token (supports environment variables)
}

// PollConfig polling configuration
type PollConfig struct {
	Interval int `yaml:"interval"` // Polling interval in seconds
	Timeout  int `yaml:"timeout"`  // Request timeout in seconds
}

// DeployConfig deployment configuration
type DeployConfig struct {
	Namespace string `yaml:"namespace"` // Kubernetes namespace
	TmpDir    string `yaml:"tmp_dir"`   // Temporary directory
	Cleanup   bool   `yaml:"cleanup"`   // Whether to auto cleanup
}

// LoadConfig loads configuration from YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Load .env file (if exists)
	if err := godotenv.Load(); err != nil {
		// .env file not existing is normal, don't error
		fmt.Printf("Warning: .env file not found: %v\n", err)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Replace environment variables
	configContent := expandEnvVars(string(data))

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// expandEnvVars expands environment variables in configuration
// Supports formats: ${VAR_NAME} and $VAR_NAME
func expandEnvVars(content string) string {
	// Match ${VAR_NAME} format
	re1 := regexp.MustCompile(`\$\{([^}]+)\}`)
	content = re1.ReplaceAllStringFunc(content, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		return os.Getenv(varName)
	})

	// Match $VAR_NAME format (variable name contains only letters, numbers, underscores)
	re2 := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	content = re2.ReplaceAllStringFunc(content, func(match string) string {
		varName := match[1:] // Remove $
		return os.Getenv(varName)
	})

	return content
}

// validateConfig validates configuration validity
func validateConfig(config *Config) error {
	// Validate repository A configuration
	if err := validateRepoConfig(&config.Monitor.RepoA, "repo_a"); err != nil {
		return err
	}

	// Validate repository B configuration
	if err := validateRepoConfig(&config.Monitor.RepoB, "repo_b"); err != nil {
		return err
	}

	// Validate polling configuration
	if config.Monitor.Poll.Interval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	if config.Monitor.Poll.Timeout <= 0 {
		return fmt.Errorf("poll timeout must be positive")
	}

	// Validate deployment configuration
	if strings.TrimSpace(config.Deploy.Namespace) == "" {
		return fmt.Errorf("deploy namespace cannot be empty")
	}
	if strings.TrimSpace(config.Deploy.TmpDir) == "" {
		return fmt.Errorf("deploy tmp_dir cannot be empty")
	}

	return nil
}

// validateRepoConfig validates single repository configuration
func validateRepoConfig(repo *RepoConfig, name string) error {
	if repo.Type != "github" && repo.Type != "gitlab" {
		return fmt.Errorf("%s: type must be 'github' or 'gitlab', got: %s", name, repo.Type)
	}

	if strings.TrimSpace(repo.URL) == "" {
		return fmt.Errorf("%s: URL cannot be empty", name)
	}

	if strings.TrimSpace(repo.Branch) == "" {
		return fmt.Errorf("%s: branch cannot be empty", name)
	}

	if strings.TrimSpace(repo.Token) == "" {
		return fmt.Errorf("%s: token cannot be empty", name)
	}

	return nil
}

// GetConfigExample returns configuration file example
func GetConfigExample() string {
	return `# Sentry configuration file
monitor:
  # Repository A configuration (GitHub example)
  repo_a:
    type: "github"
    url: "https://github.com/owner/repo-a"
    branch: "main"
    token: "${GITHUB_TOKEN}"  # Environment variable
  
  # Repository B configuration (GitLab example)
  repo_b:
    type: "gitlab" 
    url: "https://gitlab.com/owner/repo-b"
    branch: "main"
    token: "${GITLAB_TOKEN}"  # Environment variable
  
  # Polling configuration
  poll:
    interval: 30  # Poll interval in seconds
    timeout: 10   # Request timeout in seconds

# Deployment configuration
deploy:
  namespace: "tekton-pipelines"  # Kubernetes namespace
  tmp_dir: "/tmp/sentry"         # Temporary directory
  cleanup: true                  # Auto cleanup temporary files
`
}
