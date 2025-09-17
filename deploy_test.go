package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDeployService(t *testing.T) {
	config := &Config{
		Deploy: DeployConfig{
			Namespace: "test-namespace",
			TmpDir:    "/tmp/test",
			Cleanup:   true,
		},
	}

	service := NewDeployService(config)

	if service.config != config {
		t.Error("NewDeployService() should set config correctly")
	}
}

func TestCreateTempDirectory(t *testing.T) {
	config := &Config{
		Deploy: DeployConfig{
			TmpDir: "/tmp/sentry-test",
		},
	}
	service := NewDeployService(config)

	// Test creating temp directory
	tmpDir, err := service.createTempDirectory("test-repo")
	if err != nil {
		t.Fatalf("createTempDirectory() error = %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("Temp directory should exist: %s", tmpDir)
	}

	// Verify path format
	expectedPath := filepath.Join("/tmp/sentry-test", "sentry-test-repo")
	if tmpDir != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, tmpDir)
	}

	// Cleanup
	os.RemoveAll("/tmp/sentry-test")
}

func TestCleanupTempDirectory(t *testing.T) {
	config := &Config{
		Deploy: DeployConfig{
			TmpDir: "/tmp/sentry-test-cleanup",
		},
	}
	service := NewDeployService(config)

	// Create test directory
	testDir := "/tmp/sentry-test-cleanup/test"
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test cleanup
	err = service.cleanupTempDirectory("/tmp/sentry-test-cleanup")
	if err != nil {
		t.Fatalf("cleanupTempDirectory() error = %v", err)
	}

	// Verify directory is removed
	if _, err := os.Stat("/tmp/sentry-test-cleanup"); !os.IsNotExist(err) {
		t.Error("Directory should be removed after cleanup")
	}
}

func TestScanYAMLFiles(t *testing.T) {
	config := &Config{}
	service := NewDeployService(config)

	// Create test directory structure
	testDir := "/tmp/sentry-test-yaml"
	tektonDir := filepath.Join(testDir, ".tekton")
	err := os.MkdirAll(tektonDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create test YAML files
	testFiles := map[string]string{
		"pipeline.yaml": `apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: test-pipeline`,
		"task.yml": `apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: test-task`,
		"not-tekton.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`,
		"readme.txt": "This is not a YAML file",
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tektonDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Test scanning YAML files
	yamlFiles, err := service.scanYAMLFiles(tektonDir)
	if err != nil {
		t.Fatalf("scanYAMLFiles() error = %v", err)
	}

	// Should find 2 Tekton YAML files (pipeline.yaml and task.yml)
	expectedCount := 2
	if len(yamlFiles) != expectedCount {
		t.Errorf("Expected %d YAML files, got %d", expectedCount, len(yamlFiles))
	}

	// Verify file paths
	expectedFiles := []string{
		filepath.Join(tektonDir, "pipeline.yaml"),
		filepath.Join(tektonDir, "task.yml"),
	}

	for _, expectedFile := range expectedFiles {
		found := false
		for _, actualFile := range yamlFiles {
			if actualFile == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file not found: %s", expectedFile)
		}
	}
}

func TestIsTektonResource(t *testing.T) {
	config := &Config{}
	service := NewDeployService(config)

	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "tekton pipeline",
			content: `apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: test-pipeline`,
			expected: true,
		},
		{
			name: "tekton task",
			content: `apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: test-task`,
			expected: true,
		},
		{
			name: "tekton trigger binding",
			content: `apiVersion: triggers.tekton.dev/v1alpha1
kind: TriggerBinding
metadata:
  name: test-binding`,
			expected: true,
		},
		{
			name: "non-tekton resource",
			content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`,
			expected: false,
		},
		{
			name: "deployment resource",
			content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment`,
			expected: false,
		},
	}

	// Create temporary directory for test files
	testDir := "/tmp/sentry-test-tekton"
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(testDir, tc.name+".yaml")
			err := os.WriteFile(filePath, []byte(tc.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test Tekton resource detection
			result := service.isTektonResource(filePath)
			if result != tc.expected {
				t.Errorf("isTektonResource() for %s = %v, want %v", tc.name, result, tc.expected)
			}

			// Cleanup test file
			os.Remove(filePath)
		})
	}
}

func TestScanTektonFiles(t *testing.T) {
	config := &Config{}
	service := NewDeployService(config)

	// Create test directory structure
	testDir := "/tmp/sentry-test-scan"
	tektonDir1 := filepath.Join(testDir, "project1", ".tekton")
	tektonDir2 := filepath.Join(testDir, "project2", ".tekton")
	normalDir := filepath.Join(testDir, "normal")

	dirs := []string{tektonDir1, tektonDir2, normalDir}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
	defer os.RemoveAll(testDir)

	// Create test files
	testFiles := map[string]string{
		filepath.Join(tektonDir1, "pipeline.yaml"): `apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: pipeline1`,
		filepath.Join(tektonDir2, "task.yml"): `apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task2`,
		filepath.Join(normalDir, "config.yaml"): `apiVersion: v1
kind: ConfigMap
metadata:
  name: config`,
	}

	for filePath, content := range testFiles {
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Test scanning Tekton files
	tektonFiles, err := service.scanTektonFiles(testDir)
	if err != nil {
		t.Fatalf("scanTektonFiles() error = %v", err)
	}

	// Should find 2 Tekton files
	expectedCount := 2
	if len(tektonFiles) != expectedCount {
		t.Errorf("Expected %d Tekton files, got %d", expectedCount, len(tektonFiles))
	}

	// Verify that only Tekton files from .tekton directories are found
	for _, filePath := range tektonFiles {
		if !strings.Contains(filePath, ".tekton") {
			t.Errorf("Non-.tekton file found: %s", filePath)
		}
	}
}

func TestDeployResult(t *testing.T) {
	result := &DeployResult{
		RepoKey:       "test-repo",
		ClonePath:     "/tmp/test-path",
		FilesDeployed: []string{"file1.yaml", "file2.yaml"},
		Success:       true,
	}

	if result.RepoKey != "test-repo" {
		t.Errorf("Expected RepoKey 'test-repo', got '%s'", result.RepoKey)
	}

	if len(result.FilesDeployed) != 2 {
		t.Errorf("Expected 2 deployed files, got %d", len(result.FilesDeployed))
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
}
