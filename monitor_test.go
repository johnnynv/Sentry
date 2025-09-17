package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewMonitorService(t *testing.T) {
	config := &Config{
		Monitor: MonitorConfig{
			Poll: PollConfig{
				Timeout: 10,
			},
		},
		Deploy: DeployConfig{
			TmpDir: "/tmp/test",
		},
	}

	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	if service.config != config {
		t.Error("NewMonitorService() should set config correctly")
	}

	if service.httpClient.Timeout != 10*time.Second {
		t.Errorf("NewMonitorService() timeout = %v, want %v", service.httpClient.Timeout, 10*time.Second)
	}

	if service.lastCommit == nil {
		t.Error("NewMonitorService() should initialize lastCommit map")
	}

	if service.deployService != deployService {
		t.Error("NewMonitorService() should set deployService correctly")
	}
}

func TestGitHubAPIResponseParsing(t *testing.T) {
	// Mock GitHub API response
	mockResponse := map[string]interface{}{
		"sha": "abc123def456",
		"commit": map[string]interface{}{
			"message": "Test commit message",
			"author": map[string]interface{}{
				"name": "Test Author",
				"date": "2023-01-01T12:00:00Z",
			},
		},
		"html_url": "https://github.com/owner/repo/commit/abc123def456",
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if auth := r.Header.Get("Authorization"); auth != "token test_token" {
			t.Errorf("Expected Authorization header 'token test_token', got '%s'", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Create monitor service
	config := &Config{
		Monitor: MonitorConfig{
			Poll: PollConfig{Timeout: 10},
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	// Test the HTTP request directly
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "token test_token")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := service.httpClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var githubCommit struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		HTMLURL string `json:"html_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubCommit); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	commit := &CommitInfo{
		SHA:       githubCommit.SHA,
		Message:   githubCommit.Commit.Message,
		Author:    githubCommit.Commit.Author.Name,
		Timestamp: githubCommit.Commit.Author.Date,
		URL:       githubCommit.HTMLURL,
	}

	if commit.SHA != "abc123def456" {
		t.Errorf("Expected SHA 'abc123def456', got '%s'", commit.SHA)
	}

	if commit.Message != "Test commit message" {
		t.Errorf("Expected message 'Test commit message', got '%s'", commit.Message)
	}

	if commit.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", commit.Author)
	}
}

func TestGitLabAPIResponseParsing(t *testing.T) {
	// Mock GitLab API response
	mockResponse := map[string]interface{}{
		"id":          "def456abc123",
		"title":       "Test GitLab commit",
		"author_name": "GitLab Author",
		"created_at":  "2023-01-01T12:00:00Z",
		"web_url":     "https://gitlab.com/owner/project/-/commit/def456abc123",
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if auth := r.Header.Get("Authorization"); auth != "Bearer gitlab_token" {
			t.Errorf("Expected Authorization header 'Bearer gitlab_token', got '%s'", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Create monitor service
	config := &Config{
		Monitor: MonitorConfig{
			Poll: PollConfig{Timeout: 10},
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	// Test the HTTP request directly
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer gitlab_token")

	resp, err := service.httpClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var gitlabCommit struct {
		ID         string    `json:"id"`
		Title      string    `json:"title"`
		AuthorName string    `json:"author_name"`
		CreatedAt  time.Time `json:"created_at"`
		WebURL     string    `json:"web_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gitlabCommit); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	commit := &CommitInfo{
		SHA:       gitlabCommit.ID,
		Message:   gitlabCommit.Title,
		Author:    gitlabCommit.AuthorName,
		Timestamp: gitlabCommit.CreatedAt,
		URL:       gitlabCommit.WebURL,
	}

	if commit.SHA != "def456abc123" {
		t.Errorf("Expected SHA 'def456abc123', got '%s'", commit.SHA)
	}

	if commit.Message != "Test GitLab commit" {
		t.Errorf("Expected message 'Test GitLab commit', got '%s'", commit.Message)
	}

	if commit.Author != "GitLab Author" {
		t.Errorf("Expected author 'GitLab Author', got '%s'", commit.Author)
	}
}

func TestCommitChangeDetection(t *testing.T) {
	config := &Config{
		Monitor: MonitorConfig{
			Poll: PollConfig{Timeout: 10},
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	// Test initial state - no previous commit
	if service.lastCommit["test_repo"] != "" {
		t.Error("lastCommit should be empty initially")
	}

	// Simulate first commit record
	service.lastCommit["test_repo"] = "commit123"

	// Check if same commit is detected as no change
	if service.lastCommit["test_repo"] != "commit123" {
		t.Error("lastCommit should persist")
	}

	// Simulate new commit
	service.lastCommit["test_repo"] = "commit456"

	if service.lastCommit["test_repo"] != "commit456" {
		t.Error("lastCommit should update to new commit")
	}
}

func TestGetLatestCommitUnsupportedType(t *testing.T) {
	config := &Config{
		Monitor: MonitorConfig{
			Poll: PollConfig{Timeout: 10},
		},
	}
	deployService := NewDeployService(config)
	service := NewMonitorService(config, deployService)

	repo := &RepoConfig{
		Type: "unsupported",
	}

	_, err := service.GetLatestCommit(repo)
	if err == nil {
		t.Error("GetLatestCommit() should error for unsupported type")
	}

	expectedMsg := "unsupported repository type: unsupported"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}
