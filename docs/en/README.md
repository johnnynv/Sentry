# Sentry English Documentation

Welcome to Sentry - Tekton Pipeline Auto-Deployer! This directory contains comprehensive English documentation.

## 📚 Documentation Index

### Core Documentation

- **[Deployment Guide](deployment.md)** - Detailed deployment instructions, primarily using Helm Chart
- **[Architecture Design](architecture.md)** - System architecture and design principles  
- **[Implementation Plan](implementation.md)** - Project implementation and development roadmap

### Quick Reference

| Document | Description | Recommended Users |
|----------|-------------|-------------------|
| [Deployment Guide](deployment.md) | 🚀 Helm Chart deployment, YAML manifests, Docker deployment | DevOps Engineers, SRE |
| [Architecture Design](architecture.md) | 🏗️ System architecture, component relationships, technology choices | Developers, Architects |
| [Implementation Plan](implementation.md) | 📋 Development plan, phase breakdown, timeline | Project Managers, Development Teams |

## 🚀 Quick Start

If you're new to Sentry, we recommend reading in this order:

1. **Understand the System** - Read [Architecture Design](architecture.md) to understand how Sentry works
2. **Deploy the System** - Follow [Deployment Guide](deployment.md) to deploy Sentry in your environment
3. **Configure and Use** - Configure monitored repositories and deployment strategies according to your needs

## 📖 Key Features

Sentry provides the following core capabilities:

- ✅ **Multi-Platform Support** - Supports GitHub, GitLab, Gitea and other Git platforms
- ✅ **Intelligent Monitoring** - Automatically detects code repository changes and triggers deployments
- ✅ **Group-Level Deployment** - Supports parallel and sequential batch deployment strategies
- ✅ **Flexible Configuration** - Supports various deployment commands and custom scripts
- ✅ **Security & Reliability** - Complete RBAC permission control and error recovery mechanisms
- ✅ **Cloud-Native** - Optimized design for Kubernetes and Tekton Pipelines

## 🎯 Deployment Methods Comparison

| Deployment Method | Use Case | Complexity | Recommendation |
|-------------------|----------|------------|----------------|
| **Helm Chart** | Production environments, multi-environment management | Medium | ⭐⭐⭐⭐⭐ |
| **Raw YAML** | Simple environments, custom requirements | Simple | ⭐⭐⭐ |
| **Docker** | Local testing, development debugging | Simple | ⭐⭐ |

## 🔧 System Requirements

### Base Environment
- Kubernetes 1.20+
- Tekton Pipelines
- kubectl access permissions

### Optional Components
- Helm 3.0+ (recommended)
- Docker (local development)
- Git client

## 📝 Configuration Examples

### Minimal Configuration
```yaml
polling_interval: 60
repositories:
  - name: "my-project"
    monitor:
      repo_url: "https://github.com/org/repo"
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
```

### Advanced Configuration (Group-Level Deployment)
```yaml
polling_interval: 60
groups:
  ai-projects:
    execution_strategy: "parallel"
    max_parallel: 3
    continue_on_error: true
    global_timeout: 900
repositories:
  - name: "rag-service"
    group: "ai-projects"
    # ... detailed configuration in deployment guide
```

## 🆘 Getting Help

When encountering issues, follow this resolution path:

1. **Check Logs** - Use `kubectl logs` to view detailed error information
2. **Verify Configuration** - Validate YAML configuration file format and content  
3. **Permission Verification** - Confirm token permissions and RBAC configuration
4. **Reference Documentation** - Check troubleshooting sections in relevant chapters
5. **Community Support** - Submit issues in the project repository

## 🔄 Documentation Updates

This documentation is updated synchronously with project versions. Current documentation corresponds to:

- **Sentry Version**: v1.0.0
- **Documentation Version**: v1.0.0
- **Last Updated**: 2025-09-18

---

**Note**: For Chinese version documentation, please refer to the `docs/zh/` directory.