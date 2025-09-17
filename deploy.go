package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DeployService handles Tekton pipeline deployment
type DeployService struct {
	config *Config
}

// DeployResult represents the result of a deployment operation
type DeployResult struct {
	RepoKey       string   `json:"repo_key"`
	ClonePath     string   `json:"clone_path"`
	FilesDeployed []string `json:"files_deployed"`
	Success       bool     `json:"success"`
	Error         string   `json:"error,omitempty"`
}

// NewDeployService creates a new deploy service instance
func NewDeployService(config *Config) *DeployService {
	return &DeployService{
		config: config,
	}
}

// DeployFromRepository clones repository and deploys Tekton configurations
func (d *DeployService) DeployFromRepository(repo *RepoConfig, repoKey string) (*DeployResult, error) {
	result := &DeployResult{
		RepoKey:       repoKey,
		FilesDeployed: []string{},
		Success:       false,
	}

	// Create temporary directory for cloning
	tmpDir, err := d.createTempDirectory(repoKey)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create temp directory: %v", err)
		return result, err
	}
	result.ClonePath = tmpDir

	// Ensure cleanup happens regardless of success/failure
	defer func() {
		if d.config.Deploy.Cleanup {
			if cleanupErr := d.cleanupTempDirectory(tmpDir); cleanupErr != nil {
				fmt.Printf("Warning: failed to cleanup temp directory %s: %v\n", tmpDir, cleanupErr)
			}
		}
	}()

	// Clone repository with retry mechanism
	if err := d.cloneRepositoryWithRetry(repo, tmpDir); err != nil {
		result.Error = fmt.Sprintf("failed to clone repository: %v", err)
		return result, err
	}

	// Scan for .tekton directories and YAML files
	tektonFiles, err := d.scanTektonFiles(tmpDir)
	if err != nil {
		result.Error = fmt.Sprintf("failed to scan Tekton files: %v", err)
		return result, err
	}

	if len(tektonFiles) == 0 {
		result.Error = "no Tekton configuration files found"
		return result, fmt.Errorf("no Tekton configuration files found in repository")
	}

	// Deploy each Tekton file with rollback on failure
	deployedFiles := []string{}
	for _, filePath := range tektonFiles {
		if err := d.deployTektonFileWithRetry(filePath); err != nil {
			// Rollback previously deployed files
			d.rollbackDeployedFiles(deployedFiles)
			result.Error = fmt.Sprintf("failed to deploy %s: %v", filePath, err)
			return result, err
		}
		deployedFiles = append(deployedFiles, filePath)
		result.FilesDeployed = append(result.FilesDeployed, filePath)
	}

	result.Success = true
	fmt.Printf("Successfully deployed %d Tekton files from %s\n", len(result.FilesDeployed), repoKey)
	return result, nil
}

// createTempDirectory creates a temporary directory for repository cloning
func (d *DeployService) createTempDirectory(repoKey string) (string, error) {
	baseDir := d.config.Deploy.TmpDir
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create base temp directory: %w", err)
	}

	tmpDir := filepath.Join(baseDir, fmt.Sprintf("sentry-%s", repoKey))

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

// cloneRepository clones the Git repository to specified directory
func (d *DeployService) cloneRepository(repo *RepoConfig, destDir string) error {
	// Prepare Git clone command
	var cmd *exec.Cmd

	if repo.Type == "github" {
		// For GitHub, use HTTPS with token authentication
		authURL := strings.Replace(repo.URL, "https://", fmt.Sprintf("https://%s@", repo.Token), 1)
		cmd = exec.Command("git", "clone", "--branch", repo.Branch, "--depth", "1", authURL, destDir)
	} else if repo.Type == "gitlab" {
		// For GitLab, use HTTPS with token authentication
		authURL := strings.Replace(repo.URL, "https://", fmt.Sprintf("https://oauth2:%s@", repo.Token), 1)
		cmd = exec.Command("git", "clone", "--branch", repo.Branch, "--depth", "1", authURL, destDir)
	} else {
		return fmt.Errorf("unsupported repository type: %s", repo.Type)
	}

	// Set environment to avoid interactive prompts
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=echo",
	)

	// Execute clone command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	fmt.Printf("Successfully cloned repository to %s\n", destDir)
	return nil
}

// scanTektonFiles recursively scans for .tekton directories and YAML files
func (d *DeployService) scanTektonFiles(rootDir string) ([]string, error) {
	var tektonFiles []string

	err := filepath.WalkDir(rootDir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check if this is a .tekton directory
		if info.IsDir() && info.Name() == ".tekton" {
			// Scan YAML files in .tekton directory
			yamlFiles, scanErr := d.scanYAMLFiles(path)
			if scanErr != nil {
				return fmt.Errorf("failed to scan YAML files in %s: %w", path, scanErr)
			}
			tektonFiles = append(tektonFiles, yamlFiles...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory tree: %w", err)
	}

	return tektonFiles, nil
}

// scanYAMLFiles scans for YAML files in specified directory
func (d *DeployService) scanYAMLFiles(dir string) ([]string, error) {
	var yamlFiles []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		ext := strings.ToLower(filepath.Ext(fileName))

		// Check for YAML file extensions
		if ext == ".yaml" || ext == ".yml" {
			fullPath := filepath.Join(dir, fileName)

			// Validate that this is a Tekton resource
			if d.isTektonResource(fullPath) {
				yamlFiles = append(yamlFiles, fullPath)
			}
		}
	}

	return yamlFiles, nil
}

// isTektonResource checks if a YAML file contains Tekton resources
func (d *DeployService) isTektonResource(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Warning: failed to read file %s: %v\n", filePath, err)
		return false
	}

	contentStr := string(content)

	// Check for Tekton API versions and kinds
	tektonIndicators := []string{
		"apiVersion: tekton.dev/",
		"kind: Pipeline",
		"kind: PipelineRun",
		"kind: Task",
		"kind: TaskRun",
		"kind: TriggerBinding",
		"kind: TriggerTemplate",
		"kind: EventListener",
	}

	for _, indicator := range tektonIndicators {
		if strings.Contains(contentStr, indicator) {
			return true
		}
	}

	return false
}

// deployTektonFile deploys a single Tekton YAML file using kubectl
func (d *DeployService) deployTektonFile(filePath string) error {
	// Prepare kubectl apply command
	cmd := exec.Command("kubectl", "apply", "-f", filePath, "-n", d.config.Deploy.Namespace)

	// Execute deployment command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed for %s: %w\nOutput: %s", filePath, err, string(output))
	}

	fmt.Printf("Successfully deployed: %s\n", filePath)
	fmt.Printf("kubectl output: %s\n", string(output))
	return nil
}

// cleanupTempDirectory removes the temporary directory
func (d *DeployService) cleanupTempDirectory(tmpDir string) error {
	if err := os.RemoveAll(tmpDir); err != nil {
		return fmt.Errorf("failed to remove temp directory %s: %w", tmpDir, err)
	}

	fmt.Printf("Cleaned up temporary directory: %s\n", tmpDir)
	return nil
}

// ValidateDeploymentEnvironment checks if deployment environment is ready
func (d *DeployService) ValidateDeploymentEnvironment() error {
	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl command not found in PATH: %w", err)
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git command not found in PATH: %w", err)
	}

	// Check kubectl connectivity
	cmd := exec.Command("kubectl", "cluster-info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl cluster connectivity check failed: %w", err)
	}

	// Check if target namespace exists
	cmd = exec.Command("kubectl", "get", "namespace", d.config.Deploy.Namespace)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("target namespace '%s' does not exist or is not accessible: %w", d.config.Deploy.Namespace, err)
	}

	fmt.Println("Deployment environment validation passed")
	return nil
}

// cloneRepositoryWithRetry clones repository with retry mechanism
func (d *DeployService) cloneRepositoryWithRetry(repo *RepoConfig, destDir string) error {
	maxRetries := 3
	retryDelay := 2 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("Retrying git clone (attempt %d/%d) after error: %v\n", attempt, maxRetries, lastErr)
			time.Sleep(retryDelay)
		}

		err := d.cloneRepository(repo, destDir)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("failed to clone after %d retries: %w", maxRetries, lastErr)
}

// deployTektonFileWithRetry deploys Tekton file with retry mechanism
func (d *DeployService) deployTektonFileWithRetry(filePath string) error {
	maxRetries := 2
	retryDelay := 1 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("Retrying kubectl apply (attempt %d/%d) for %s after error: %v\n", attempt, maxRetries, filePath, lastErr)
			time.Sleep(retryDelay)
		}

		err := d.deployTektonFile(filePath)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("failed to deploy %s after %d retries: %w", filePath, maxRetries, lastErr)
}

// rollbackDeployedFiles attempts to delete previously deployed resources
func (d *DeployService) rollbackDeployedFiles(deployedFiles []string) {
	if len(deployedFiles) == 0 {
		return
	}

	fmt.Printf("Rolling back %d deployed files due to deployment failure...\n", len(deployedFiles))

	for _, filePath := range deployedFiles {
		if err := d.deleteTektonFile(filePath); err != nil {
			fmt.Printf("Warning: failed to rollback %s: %v\n", filePath, err)
		} else {
			fmt.Printf("Successfully rolled back: %s\n", filePath)
		}
	}
}

// deleteTektonFile removes a deployed Tekton resource using kubectl delete
func (d *DeployService) deleteTektonFile(filePath string) error {
	// Prepare kubectl delete command
	cmd := exec.Command("kubectl", "delete", "-f", filePath, "-n", d.config.Deploy.Namespace, "--ignore-not-found=true")

	// Execute delete command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete failed for %s: %w\nOutput: %s", filePath, err, string(output))
	}

	return nil
}
