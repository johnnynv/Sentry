# Sentry - Tekton Pipeline Auto-Deployer
# Build configuration and automation

# Application metadata
APP_NAME := sentry
VERSION := 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Build settings
GO_VERSION := 1.21
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED := 0

# Directories
BUILD_DIR := build
DIST_DIR := dist
DOCKER_DIR := docker

# Docker settings
DOCKER_REGISTRY ?= localhost:5000
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(APP_NAME)
DOCKER_TAG := $(VERSION)

# Go build flags
LDFLAGS := -ldflags "-s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.BuildTime=$(BUILD_TIME)' \
	-X 'main.GitCommit=$(GIT_COMMIT)' \
	-X 'main.GitBranch=$(GIT_BRANCH)'"

.PHONY: help build clean test lint docker k8s-deploy k8s-clean helm-lint helm-install helm-uninstall install deps cross-compile

# Default target
all: clean deps test lint build

# Help information
help:
	@echo "Sentry v$(VERSION) - Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  build          Build the application binary"
	@echo "  clean          Clean build artifacts"
	@echo "  test           Run all tests"
	@echo "  test-e2e       Run end-to-end tests"
	@echo "  lint           Run code linting"
	@echo "  deps           Download and verify dependencies"
	@echo "  cross-compile  Build for multiple platforms"
	@echo "  docker         Build Docker image"
	@echo "  docker-push    Push Docker image to registry"
	@echo "  k8s-deploy     Deploy to Kubernetes"
	@echo "  k8s-clean      Clean Kubernetes deployment"
	@echo "  install        Install binary to system"
	@echo "  release        Create release package"
	@echo ""
	@echo "Environment variables:"
	@echo "  GOOS           Target OS (default: linux)"
	@echo "  GOARCH         Target architecture (default: amd64)"
	@echo "  DOCKER_REGISTRY Docker registry (default: localhost:5000)"

# Build the application
build: deps
	@echo "Building $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) .
	@echo "Binary built: $(BUILD_DIR)/$(APP_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DIST_DIR)
	@rm -f $(APP_NAME)
	@go clean
	@echo "Clean completed"

# Download and verify dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@go mod tidy

# Run tests
test: deps
	@echo "Running tests..."
	@go test -v ./...

# Run end-to-end tests
test-e2e: deps
	@echo "Running end-to-end tests..."
	@SENTRY_E2E_TEST=true go test -v -run TestEndToEnd ./...

# Run linting
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, running go vet..."; \
		go vet ./...; \
	fi

# Cross-compile for multiple platforms
cross-compile: deps
	@echo "Cross-compiling for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	
	# Linux AMD64
	@echo "Building for Linux AMD64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 .
	
	# Linux ARM64
	@echo "Building for Linux ARM64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 .
	
	# macOS AMD64
	@echo "Building for macOS AMD64..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 .
	
	# macOS ARM64
	@echo "Building for macOS ARM64..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 .
	
	# Windows AMD64
	@echo "Building for Windows AMD64..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe .
	
	@echo "Cross-compilation completed. Binaries in $(DIST_DIR)/"

# Build Docker image
docker: build
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Push Docker image
docker-push: docker
	@echo "Pushing Docker image..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@docker push $(DOCKER_IMAGE):latest
	@echo "Docker image pushed: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Deploy to Kubernetes
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	@kubectl apply -f k8s/
	@echo "Deployment completed"

# Clean Kubernetes deployment
k8s-clean:
	@echo "Cleaning Kubernetes deployment..."
	@kubectl delete -f k8s/ --ignore-not-found=true
	@echo "Cleanup completed"

# Lint Helm chart
helm-lint:
	@echo "Linting Helm chart..."
	@helm lint helm/sentry
	@echo "Helm lint completed"

# Install via Helm (development)
helm-install:
	@echo "Installing Sentry via Helm..."
	@helm upgrade --install sentry helm/sentry \
		--create-namespace \
		--namespace sentry-system \
		--set secrets.githubToken="$(GITHUB_TOKEN)" \
		--set secrets.gitlabToken="$(GITLAB_TOKEN)"
	@echo "Helm installation completed"

# Install via Helm with custom values
helm-install-dev:
	@echo "Installing Sentry via Helm (development)..."
	@helm upgrade --install sentry-dev helm/sentry \
		--create-namespace \
		--namespace sentry-dev \
		-f helm/sentry/values-dev.yaml
	@echo "Development Helm installation completed"

# Install via Helm with production values
helm-install-prod:
	@echo "Installing Sentry via Helm (production)..."
	@helm upgrade --install sentry-prod helm/sentry \
		--create-namespace \
		--namespace sentry-system \
		-f helm/sentry/values-production.yaml
	@echo "Production Helm installation completed"

# Uninstall Helm release
helm-uninstall:
	@echo "Uninstalling Helm release..."
	@helm uninstall sentry --namespace sentry-system || true
	@kubectl delete namespace sentry-system --ignore-not-found=true
	@echo "Helm uninstall completed"

# Install binary to system
install: build
	@echo "Installing $(APP_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(APP_NAME) /usr/local/bin/
	@sudo chmod +x /usr/local/bin/$(APP_NAME)
	@echo "Installation completed"

# Create release package
release: clean cross-compile
	@echo "Creating release package..."
	@mkdir -p $(DIST_DIR)/release
	@cp README.md $(DIST_DIR)/release/
	@cp sentry.yaml $(DIST_DIR)/release/
	@cp env.example $(DIST_DIR)/release/
	@cp -r docs $(DIST_DIR)/release/
	@cp -r k8s $(DIST_DIR)/release/ 2>/dev/null || true
	@cd $(DIST_DIR) && tar -czf $(APP_NAME)-$(VERSION).tar.gz release/ $(APP_NAME)-*
	@echo "Release package created: $(DIST_DIR)/$(APP_NAME)-$(VERSION).tar.gz"

# Show build information
info:
	@echo "Build Information:"
	@echo "  Application: $(APP_NAME)"
	@echo "  Version: $(VERSION)"
	@echo "  Build Time: $(BUILD_TIME)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@echo "  Git Branch: $(GIT_BRANCH)"
	@echo "  Go Version: $(shell go version)"
	@echo "  Target OS: $(GOOS)"
	@echo "  Target Arch: $(GOARCH)"
