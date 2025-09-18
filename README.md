# Sentry - Tekton Pipeline Auto-Deployer

[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/your-org/sentry)
[![Go](https://img.shields.io/badge/go-1.21-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Sentry is an automated deployment tool that monitors Git repositories for changes and automatically deploys Tekton pipeline configurations to Kubernetes clusters. It supports both GitHub and GitLab repositories with intelligent change detection and robust error handling.

## Features

- **Multi-Platform Support**: Works with GitHub and GitLab repositories
- **Automatic Detection**: Monitors repository changes and triggers deployments
- **Tekton Integration**: Scans for `.tekton` directories and deploys pipeline configurations
- **Robust Error Handling**: Includes retry mechanisms and rollback capabilities
- **Secure**: Uses environment variables for sensitive information
- **Kubernetes Native**: Designed to run in Kubernetes with proper RBAC
- **Cross-Platform**: Supports Linux, macOS, and Windows

## Quick Start

### Prerequisites

- Go 1.21 or later
- Kubernetes cluster with Tekton Pipelines installed
- kubectl configured for your cluster
- Git access tokens for your repositories

### Installation

#### From Source

```bash
git clone https://github.com/your-org/sentry.git
cd sentry
make build
./build/sentry -version
```

#### Using Docker

```bash
docker pull localhost:5000/sentry:1.0.0
docker run --rm -v $(pwd)/sentry.yaml:/app/sentry.yaml \
  -e GITHUB_TOKEN=your_token \
  -e GITLAB_TOKEN=your_token \
  localhost:5000/sentry:1.0.0 -action=validate
```

#### Using Kubernetes (Raw Manifests)

```bash
# Update tokens in k8s/02-secret.yaml
kubectl apply -f k8s/
```

#### Using Helm (Recommended)

```bash
# Quick install with inline values
helm install sentry ./helm/sentry \
  --set secrets.githubToken="your_github_token" \
  --set secrets.gitlabToken="your_gitlab_token"

# Install development environment
helm install sentry-dev ./helm/sentry -f ./helm/sentry/values-dev.yaml

# Install production environment
helm install sentry-prod ./helm/sentry -f ./helm/sentry/values-production.yaml
```

### Configuration

Create a `sentry.yaml` configuration file:

```yaml
polling_interval: 60

groups:
  my-projects:
    execution_strategy: "parallel"
    max_parallel: 3
    continue_on_error: true
    global_timeout: 900

repositories:
  - name: "my-project"
    group: "my-projects"
    monitor:
      repo_url: "https://github.com/owner/repo"
      branches: ["main"]
      repo_type: "github"
      auth:
        username: "${GITHUB_USERNAME}"
        token: "${GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab.com/qa/repo"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      auth:
        username: "${GITLAB_USERNAME}"
        token: "${GITLAB_TOKEN}"
      project_name: "my-project"
      commands:
        - "cd .tekton/my-project"
        - "kubectl apply -f . --namespace=tekton-pipelines"

global:
  tmp_dir: "/tmp/sentry"
  cleanup: true
  log_level: "info"
  timeout: 300
```

Set environment variables:

```bash
export GITHUB_TOKEN="your_github_token"
export GITLAB_TOKEN="your_gitlab_token"
```

### Usage

#### Validate Configuration

```bash
sentry -action=validate
```

#### Manual Deployment Trigger

```bash
sentry -action=trigger
```

#### Continuous Monitoring

```bash
sentry -action=watch -verbose
```

## Documentation

- [Architecture Design](docs/zh/architecture.md)
- [Implementation Plan](docs/zh/implementation.md)
- [Deployment Guide](#deployment-guide)

## Build and Development

### Building

```bash
# Build for current platform
make build

# Cross-compile for all platforms
make cross-compile

# Build Docker image
make docker

# Run tests
make test

# Run end-to-end tests
make test-e2e
```

### Development

```bash
# Install dependencies
make deps

# Run linting
make lint

# Clean build artifacts
make clean
```

## Deployment Guide

### Kubernetes Deployment

#### Option 1: Helm Deployment (Recommended)

1. **Quick Start**:

```bash
helm install sentry ./helm/sentry \
  --create-namespace \
  --namespace sentry-system \
  --set secrets.githubToken="your_github_token" \
  --set secrets.gitlabToken="your_gitlab_token"
```

2. **Custom Configuration**:

Create a `my-values.yaml`:

```yaml
config:
  pollingInterval: 60
  repositories:
    - name: "my-project"
      monitor:
        repo_url: "https://github.com/your-org/your-repo"
        branches: ["main"]
        repo_type: "github"
        auth:
          username: "${GITHUB_USERNAME}"
          token: "${GITHUB_TOKEN}"
      deploy:
        qa_repo_url: "https://gitlab.com/qa/repo"
        qa_repo_branch: "main"
        repo_type: "gitlab"
        auth:
          username: "${GITLAB_USERNAME}"
          token: "${GITLAB_TOKEN}"
        project_name: "my-project"
        commands:
          - "cd .tekton/my-project"
          - "kubectl apply -f ."

secrets:
  githubToken: "your_github_token"
  gitlabToken: "your_gitlab_token"
```

Deploy with custom values:

```bash
helm install sentry ./helm/sentry -f my-values.yaml
```

3. **Environment-Specific Deployments**:

```bash
# Development
helm install sentry-dev ./helm/sentry -f ./helm/sentry/values-dev.yaml

# Production
helm install sentry-prod ./helm/sentry -f ./helm/sentry/values-production.yaml
```

4. **Verify Deployment**:

```bash
kubectl get pods -n sentry-system
kubectl logs -f deployment/sentry -n sentry-system
```

#### Option 2: Raw Manifests

1. **Update Secrets**: Edit `k8s/02-secret.yaml` with your actual tokens (base64 encoded):

```bash
echo -n "your_github_token" | base64
echo -n "your_gitlab_token" | base64
```

2. **Update Configuration**: Modify `k8s/03-configmap.yaml` with your repository URLs.

3. **Deploy**:

```bash
kubectl apply -f k8s/
```

4. **Verify**:

```bash
kubectl get pods -n sentry-system
kubectl logs -f deployment/sentry -n sentry-system
```

### Security Considerations

- Use dedicated service accounts with minimal required permissions
- Store sensitive tokens in Kubernetes Secrets
- Enable read-only root filesystem in containers
- Run containers as non-root user

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For questions and support:

- Create an issue on GitHub
- Check the documentation in the `docs/` directory
- Review the implementation plan for technical details

## Changelog

### v1.0.0 (2025-09-17)

- Initial release
- GitHub and GitLab support
- Automatic Tekton deployment
- Kubernetes integration
- Comprehensive error handling
- Full test coverage