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
	PollingInterval int                    `yaml:"polling_interval"`
	Groups          map[string]GroupConfig `yaml:"groups,omitempty"`
	Repositories    []RepositoryConfig     `yaml:"repositories"`
	Global          GlobalConfig           `yaml:"global,omitempty"`
}

// GroupConfig defines execution strategy for a group of repositories
type GroupConfig struct {
	ExecutionStrategy string `yaml:"execution_strategy"` // "parallel" or "sequential"
	MaxParallel       int    `yaml:"max_parallel"`       // Maximum parallel executions
	ContinueOnError   bool   `yaml:"continue_on_error"`  // Continue if one project fails
	GlobalTimeout     int    `yaml:"global_timeout"`     // Global timeout in seconds
}

// RepositoryConfig defines a single repository configuration
type RepositoryConfig struct {
	Name       string        `yaml:"name"`
	Group      string        `yaml:"group,omitempty"` // Optional group name
	Monitor    MonitorConfig `yaml:"monitor"`
	Deploy     DeployConfig  `yaml:"deploy"`
	WebhookURL string        `yaml:"webhook_url,omitempty"`
}

// MonitorConfig defines repository monitoring configuration
type MonitorConfig struct {
	RepoURL  string     `yaml:"repo_url"`
	Branches []string   `yaml:"branches"` // Supports regex patterns
	RepoType string     `yaml:"repo_type"`
	Auth     AuthConfig `yaml:"auth"`
}

// DeployConfig defines deployment configuration
type DeployConfig struct {
	QARepoURL    string     `yaml:"qa_repo_url"`
	QARepoBranch string     `yaml:"qa_repo_branch"`
	RepoType     string     `yaml:"repo_type"`
	Auth         AuthConfig `yaml:"auth"`
	ProjectName  string     `yaml:"project_name"`
	Commands     []string   `yaml:"commands"`
}

// AuthConfig defines authentication configuration
type AuthConfig struct {
	Username string `yaml:"username"`
	Token    string `yaml:"token"`
}

// GlobalConfig defines global settings
type GlobalConfig struct {
	TmpDir   string `yaml:"tmp_dir"`
	Cleanup  bool   `yaml:"cleanup"`
	LogLevel string `yaml:"log_level"`
	Timeout  int    `yaml:"timeout"`
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
	// Validate basic configuration
	if config.PollingInterval <= 0 {
		return fmt.Errorf("polling_interval must be positive")
	}
	if config.PollingInterval < 60 {
		return fmt.Errorf("polling_interval must be at least 60 seconds")
	}

	// Validate repositories
	if len(config.Repositories) == 0 {
		return fmt.Errorf("at least one repository must be configured")
	}

	repoNames := make(map[string]bool)
	for i, repo := range config.Repositories {
		// Check for duplicate names
		if repoNames[repo.Name] {
			return fmt.Errorf("duplicate repository name: %s", repo.Name)
		}
		repoNames[repo.Name] = true

		// Validate individual repository
		if err := validateRepositoryConfig(&repo, fmt.Sprintf("repositories[%d]", i)); err != nil {
			return err
		}

		// Validate group reference
		if repo.Group != "" {
			if config.Groups == nil {
				return fmt.Errorf("repository %s references group '%s' but no groups are defined", repo.Name, repo.Group)
			}
			if _, exists := config.Groups[repo.Group]; !exists {
				return fmt.Errorf("repository %s references undefined group '%s'", repo.Name, repo.Group)
			}
		}
	}

	// Validate groups
	for groupName, group := range config.Groups {
		if err := validateGroupConfig(&group, groupName); err != nil {
			return err
		}
	}

	return nil
}

// validateRepositoryConfig validates single repository configuration
func validateRepositoryConfig(repo *RepositoryConfig, context string) error {
	if strings.TrimSpace(repo.Name) == "" {
		return fmt.Errorf("%s: name cannot be empty", context)
	}

	// Validate monitor configuration
	if err := validateMonitorConfig(&repo.Monitor, fmt.Sprintf("%s.monitor", context)); err != nil {
		return err
	}

	// Validate deploy configuration
	if err := validateDeployConfig(&repo.Deploy, fmt.Sprintf("%s.deploy", context)); err != nil {
		return err
	}

	return nil
}

// validateMonitorConfig validates monitor configuration
func validateMonitorConfig(monitor *MonitorConfig, context string) error {
	if strings.TrimSpace(monitor.RepoURL) == "" {
		return fmt.Errorf("%s: repo_url cannot be empty", context)
	}

	if len(monitor.Branches) == 0 {
		return fmt.Errorf("%s: at least one branch must be specified", context)
	}

	if monitor.RepoType != "github" && monitor.RepoType != "gitlab" && monitor.RepoType != "gitea" {
		return fmt.Errorf("%s: repo_type must be 'github', 'gitlab', or 'gitea', got: %s", context, monitor.RepoType)
	}

	return validateAuthConfig(&monitor.Auth, fmt.Sprintf("%s.auth", context))
}

// validateDeployConfig validates deploy configuration
func validateDeployConfig(deploy *DeployConfig, context string) error {
	if strings.TrimSpace(deploy.QARepoURL) == "" {
		return fmt.Errorf("%s: qa_repo_url cannot be empty", context)
	}

	if strings.TrimSpace(deploy.QARepoBranch) == "" {
		return fmt.Errorf("%s: qa_repo_branch cannot be empty", context)
	}

	if deploy.RepoType != "github" && deploy.RepoType != "gitlab" && deploy.RepoType != "gitea" {
		return fmt.Errorf("%s: repo_type must be 'github', 'gitlab', or 'gitea', got: %s", context, deploy.RepoType)
	}

	if strings.TrimSpace(deploy.ProjectName) == "" {
		return fmt.Errorf("%s: project_name cannot be empty", context)
	}

	// Validate k8s naming convention for project_name
	if !isValidK8sName(deploy.ProjectName) {
		return fmt.Errorf("%s: project_name '%s' must follow Kubernetes naming conventions (lowercase letters, numbers, and hyphens only)", context, deploy.ProjectName)
	}

	if len(deploy.Commands) == 0 {
		return fmt.Errorf("%s: at least one command must be specified", context)
	}

	return validateAuthConfig(&deploy.Auth, fmt.Sprintf("%s.auth", context))
}

// validateAuthConfig validates authentication configuration
func validateAuthConfig(auth *AuthConfig, context string) error {
	if strings.TrimSpace(auth.Token) == "" {
		return fmt.Errorf("%s: token cannot be empty", context)
	}
	return nil
}

// validateGroupConfig validates group configuration
func validateGroupConfig(group *GroupConfig, groupName string) error {
	if group.ExecutionStrategy != "parallel" && group.ExecutionStrategy != "sequential" {
		return fmt.Errorf("group '%s': execution_strategy must be 'parallel' or 'sequential', got: %s", groupName, group.ExecutionStrategy)
	}

	if group.MaxParallel <= 0 {
		return fmt.Errorf("group '%s': max_parallel must be positive", groupName)
	}

	if group.GlobalTimeout <= 0 {
		return fmt.Errorf("group '%s': global_timeout must be positive", groupName)
	}

	return nil
}

// isValidK8sName checks if a name follows Kubernetes naming conventions
func isValidK8sName(name string) bool {
	// Kubernetes names must be lowercase alphanumeric characters or '-'
	// Must start and end with alphanumeric character
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	matched, _ := regexp.MatchString(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`, name)
	return matched
}

// GetConfigExample returns configuration file example
func GetConfigExample() string {
	return `# Sentry configuration file
polling_interval: 60  # Poll interval in seconds (minimum 60)

# Global group configurations
groups:
  ai-blueprints:
    execution_strategy: "parallel"  # parallel | sequential
    max_parallel: 3
    continue_on_error: true
    global_timeout: 900  # 15 minutes

# Repository configurations
repositories:
  - name: "rag-project"
    group: "ai-blueprints"  # Optional group assignment
    monitor:
      repo_url: "https://github.com/NVIDIA-AI-Blueprints/rag"
      branches: ["main", "dev.*"]  # Supports regex patterns
      repo_type: "github"
      auth:
        username: "${GITHUB_USERNAME}"
        token: "${GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab-master.nvidia.com/cloud-service-qa/Blueprint/blueprint-github-test"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      auth:
        username: "${GITLAB_USERNAME}"
        token: "${GITLAB_TOKEN}"
      project_name: "rag"  # Must follow k8s naming conventions
      commands:
        - "cd .tekton/rag"
        - "kubectl apply -f . --namespace=tekton-pipelines"
        - "kubectl wait --for=condition=Ready pipeline/rag-build --timeout=60s"
    webhook_url: ""

  - name: "chatbot-project"
    group: "ai-blueprints"  # Same group for batch processing
    monitor:
      repo_url: "https://github.com/NVIDIA-AI-Blueprints/chatbot"
      branches: ["main"]
      repo_type: "github"
      auth:
        username: "${GITHUB_USERNAME}"
        token: "${GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab-master.nvidia.com/cloud-service-qa/Blueprint/blueprint-github-test"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      auth:
        username: "${GITLAB_USERNAME}"
        token: "${GITLAB_TOKEN}"
      project_name: "chatbot"
      commands:
        - "cd .tekton/chatbot"
        - "kubectl apply -f . --namespace=tekton-pipelines"
    webhook_url: ""

  # Independent project (no group)
  - name: "standalone-service"
    monitor:
      repo_url: "https://github.com/company/standalone"
      branches: ["main"]
      repo_type: "github"
      auth:
        username: "${GITHUB_USERNAME}"
        token: "${GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab-internal.com/qa/standalone"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      auth:
        username: "${GITLAB_USERNAME}"
        token: "${GITLAB_TOKEN}"
      project_name: "standalone"
      commands:
        - "cd .tekton/standalone"
        - "kubectl apply -f . --namespace=tekton-pipelines"
    webhook_url: ""

# Global settings (optional)
global:
  tmp_dir: "/tmp/sentry"
  cleanup: true
  log_level: "info"
  timeout: 300
`
}
