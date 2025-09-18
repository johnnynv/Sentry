package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DeployService handles Tekton pipeline deployment
type DeployService struct {
	config *Config
}

// DeployResult represents the result of a deployment operation
type DeployResult struct {
	RepoName    string   `json:"repo_name"`
	ClonePath   string   `json:"clone_path"`
	CommandsRun []string `json:"commands_run"`
	Success     bool     `json:"success"`
	Error       string   `json:"error,omitempty"`
	Duration    string   `json:"duration"`
}

// GroupDeployResult represents the result of a group deployment
type GroupDeployResult struct {
	GroupName string                   `json:"group_name"`
	Results   map[string]*DeployResult `json:"results"`
	Success   bool                     `json:"success"`
	TotalTime string                   `json:"total_time"`
	Strategy  string                   `json:"strategy"`
}

// NewDeployService creates a new deploy service instance
func NewDeployService(config *Config) *DeployService {
	return &DeployService{
		config: config,
	}
}

// DeployGroup deploys a group of repositories with specified strategy
func (d *DeployService) DeployGroup(groupName string, repoNames []string, groupConfig *GroupConfig) error {
	startTime := time.Now()

	AppLogger.InfoS("Starting group deployment",
		"group", groupName,
		"strategy", groupConfig.ExecutionStrategy,
		"repositories", repoNames,
		"max_parallel", groupConfig.MaxParallel)

	groupResult := &GroupDeployResult{
		GroupName: groupName,
		Results:   make(map[string]*DeployResult),
		Strategy:  groupConfig.ExecutionStrategy,
	}

	var err error
	if groupConfig.ExecutionStrategy == "parallel" {
		err = d.deployGroupParallel(repoNames, groupConfig, groupResult)
	} else {
		err = d.deployGroupSequential(repoNames, groupConfig, groupResult)
	}

	groupResult.TotalTime = time.Since(startTime).String()
	groupResult.Success = err == nil

	// Log overall result
	if groupResult.Success {
		AppLogger.LogGroupDeploymentSuccess(groupName, len(repoNames), groupResult.TotalTime)
	} else {
		AppLogger.LogGroupDeploymentFailure(groupName, err)
	}

	return err
}

// deployGroupParallel deploys repositories in parallel
func (d *DeployService) deployGroupParallel(repoNames []string, groupConfig *GroupConfig, result *GroupDeployResult) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(groupConfig.GlobalTimeout)*time.Second)
	defer cancel()

	// Create semaphore to limit concurrent deployments
	semaphore := make(chan struct{}, groupConfig.MaxParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	for _, repoName := range repoNames {
		wg.Add(1)
		go func(rn string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				mu.Lock()
				if firstError == nil {
					firstError = fmt.Errorf("deployment timeout reached")
				}
				result.Results[rn] = &DeployResult{
					RepoName: rn,
					Success:  false,
					Error:    "timeout",
				}
				mu.Unlock()
				return
			}

			// Deploy the repository
			repoResult := d.deployRepository(rn, ctx)

			mu.Lock()
			result.Results[rn] = repoResult
			if !repoResult.Success && firstError == nil && !groupConfig.ContinueOnError {
				firstError = fmt.Errorf("deployment failed for %s: %s", rn, repoResult.Error)
			}
			mu.Unlock()
		}(repoName)
	}

	wg.Wait()

	// Check if we should fail fast
	if !groupConfig.ContinueOnError && firstError != nil {
		return firstError
	}

	// Check if any deployments failed
	var failures []string
	for repoName, res := range result.Results {
		if !res.Success {
			failures = append(failures, fmt.Sprintf("%s: %s", repoName, res.Error))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("deployment failures: %s", strings.Join(failures, "; "))
	}

	return nil
}

// deployGroupSequential deploys repositories sequentially
func (d *DeployService) deployGroupSequential(repoNames []string, groupConfig *GroupConfig, result *GroupDeployResult) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(groupConfig.GlobalTimeout)*time.Second)
	defer cancel()

	for _, repoName := range repoNames {
		repoResult := d.deployRepository(repoName, ctx)
		result.Results[repoName] = repoResult

		if !repoResult.Success {
			if !groupConfig.ContinueOnError {
				return fmt.Errorf("deployment failed for %s: %s", repoName, repoResult.Error)
			}
			AppLogger.WarnS("Repository deployment failed but continuing",
				"repo", repoName,
				"error", repoResult.Error)
		}

		// Check for context timeout
		select {
		case <-ctx.Done():
			return fmt.Errorf("deployment timeout reached")
		default:
		}
	}

	return nil
}

// DeployIndividual deploys a single repository
func (d *DeployService) DeployIndividual(repoConfig *RepositoryConfig) error {
	ctx := context.Background()
	result := d.deployRepository(repoConfig.Name, ctx)

	if result.Success {
		AppLogger.LogDeploymentSuccess(repoConfig.Name, len(result.CommandsRun))
		return nil
	} else {
		AppLogger.LogDeploymentFailure(repoConfig.Name, fmt.Errorf(result.Error))
		return fmt.Errorf("deployment failed: %s", result.Error)
	}
}

// deployRepository performs the actual deployment for a single repository
func (d *DeployService) deployRepository(repoName string, ctx context.Context) *DeployResult {
	startTime := time.Now()
	result := &DeployResult{
		RepoName:    repoName,
		CommandsRun: []string{},
		Success:     false,
	}

	// Find repository configuration
	var repoConfig *RepositoryConfig
	for _, repo := range d.config.Repositories {
		if repo.Name == repoName {
			repoConfig = &repo
			break
		}
	}

	if repoConfig == nil {
		result.Error = fmt.Sprintf("repository configuration not found: %s", repoName)
		result.Duration = time.Since(startTime).String()
		return result
	}

	AppLogger.InfoS("Starting repository deployment",
		"repo", repoName,
		"qa_repo", repoConfig.Deploy.QARepoURL,
		"project", repoConfig.Deploy.ProjectName)

	// Create temporary directory for cloning
	tmpDir, err := d.createTempDirectory(repoName)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create temp directory: %v", err)
		result.Duration = time.Since(startTime).String()
		return result
	}
	result.ClonePath = tmpDir

	// Ensure cleanup happens
	defer func() {
		if d.shouldCleanup() {
			if cleanupErr := d.cleanupTempDirectory(tmpDir); cleanupErr != nil {
				AppLogger.WarnS("Failed to cleanup temp directory",
					"path", tmpDir,
					"error", cleanupErr)
			}
		}
	}()

	// Clone QA repository
	if err := d.cloneQARepository(repoConfig, tmpDir, ctx); err != nil {
		result.Error = fmt.Sprintf("failed to clone QA repository: %v", err)
		result.Duration = time.Since(startTime).String()
		return result
	}

	// Execute deployment commands
	if err := d.executeDeploymentCommands(repoConfig, tmpDir, result, ctx); err != nil {
		result.Error = fmt.Sprintf("failed to execute commands: %v", err)
		result.Duration = time.Since(startTime).String()
		return result
	}

	result.Success = true
	result.Duration = time.Since(startTime).String()

	AppLogger.InfoS("Repository deployment completed",
		"repo", repoName,
		"duration", result.Duration,
		"commands_executed", len(result.CommandsRun))

	return result
}

// createTempDirectory creates a temporary directory for repository cloning
func (d *DeployService) createTempDirectory(repoName string) (string, error) {
	baseDir := d.getTempDir()
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create base temp directory: %w", err)
	}

	tmpDir := filepath.Join(baseDir, fmt.Sprintf("sentry-%s-%d", repoName, time.Now().Unix()))

	// Remove existing directory if it exists
	if _, err := os.Stat(tmpDir); err == nil {
		if err := os.RemoveAll(tmpDir); err != nil {
			return "", fmt.Errorf("failed to remove existing temp directory: %w", err)
		}
	}

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	return tmpDir, nil
}

// cloneQARepository clones the QA repository
func (d *DeployService) cloneQARepository(repoConfig *RepositoryConfig, destDir string, ctx context.Context) error {
	AppLogger.InfoS("Cloning QA repository",
		"repo", repoConfig.Deploy.QARepoURL,
		"branch", repoConfig.Deploy.QARepoBranch,
		"dest", destDir)

	var cmd *exec.Cmd
	auth := repoConfig.Deploy.Auth

	switch repoConfig.Deploy.RepoType {
	case "github":
		// For GitHub, use HTTPS with token authentication
		cloneURL := strings.Replace(repoConfig.Deploy.QARepoURL, "https://", fmt.Sprintf("https://%s:%s@", auth.Username, auth.Token), 1)
		cmd = exec.CommandContext(ctx, "git", "clone", "--branch", repoConfig.Deploy.QARepoBranch, "--single-branch", cloneURL, destDir)

	case "gitlab":
		// For GitLab, use HTTPS with token authentication
		cloneURL := strings.Replace(repoConfig.Deploy.QARepoURL, "https://", fmt.Sprintf("https://%s:%s@", auth.Username, auth.Token), 1)
		cmd = exec.CommandContext(ctx, "git", "clone", "--branch", repoConfig.Deploy.QARepoBranch, "--single-branch", cloneURL, destDir)

	case "gitea":
		// For Gitea, use HTTPS with token authentication
		cloneURL := strings.Replace(repoConfig.Deploy.QARepoURL, "https://", fmt.Sprintf("https://%s:%s@", auth.Username, auth.Token), 1)
		cmd = exec.CommandContext(ctx, "git", "clone", "--branch", repoConfig.Deploy.QARepoBranch, "--single-branch", cloneURL, destDir)

	default:
		return fmt.Errorf("unsupported repository type: %s", repoConfig.Deploy.RepoType)
	}

	// Set environment variables to avoid interactive prompts
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	AppLogger.InfoS("QA repository cloned successfully", "repo", repoConfig.Name)
	return nil
}

// executeDeploymentCommands executes the configured deployment commands
func (d *DeployService) executeDeploymentCommands(repoConfig *RepositoryConfig, workDir string, result *DeployResult, ctx context.Context) error {
	AppLogger.InfoS("Executing deployment commands",
		"repo", repoConfig.Name,
		"commands", repoConfig.Deploy.Commands)

	for i, cmdStr := range repoConfig.Deploy.Commands {
		AppLogger.InfoS("Executing command",
			"repo", repoConfig.Name,
			"step", i+1,
			"command", cmdStr)

		// Execute command with timeout
		cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		cmd := exec.CommandContext(cmdCtx, "/bin/sh", "-c", cmdStr)
		cmd.Dir = workDir

		// Set environment variables
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("SENTRY_REPO=%s", repoConfig.Name),
			fmt.Sprintf("SENTRY_PROJECT=%s", repoConfig.Deploy.ProjectName))

		output, err := cmd.CombinedOutput()
		cancel()

		result.CommandsRun = append(result.CommandsRun, cmdStr)

		if err != nil {
			AppLogger.ErrorS("Command execution failed",
				"repo", repoConfig.Name,
				"step", i+1,
				"command", cmdStr,
				"error", err,
				"output", string(output))
			return fmt.Errorf("command failed (step %d): %s, error: %w, output: %s", i+1, cmdStr, err, string(output))
		}

		AppLogger.InfoS("Command executed successfully",
			"repo", repoConfig.Name,
			"step", i+1,
			"output_size", len(output))
	}

	return nil
}

// cleanupTempDirectory removes the temporary directory
func (d *DeployService) cleanupTempDirectory(tmpDir string) error {
	if tmpDir == "" || tmpDir == "/" {
		return fmt.Errorf("invalid temp directory path: %s", tmpDir)
	}

	AppLogger.InfoS("Cleaning up temporary directory", "path", tmpDir)
	return os.RemoveAll(tmpDir)
}

// getTempDir returns the configured temp directory or default
func (d *DeployService) getTempDir() string {
	if d.config.Global.TmpDir != "" {
		return d.config.Global.TmpDir
	}
	return "/tmp/sentry"
}

// shouldCleanup returns whether to cleanup temp directories
func (d *DeployService) shouldCleanup() bool {
	return d.config.Global.Cleanup
}
