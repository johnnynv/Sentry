# Sentry Deployment Guide

This document provides comprehensive instructions for deploying Sentry - the Tekton Pipeline Auto-Deployer in Kubernetes clusters. We recommend using Helm Chart for deployment, with alternative deployment methods as backup options.

## 📋 Prerequisites

### Required Components

- **Kubernetes Cluster**: 1.20+
- **Helm**: 3.0+
- **Tekton Pipelines**: Installed in the cluster
- **kubectl**: Configured with cluster access permissions

### Access Credentials

- **GitHub Token**: Personal Access Token with repository read permissions
- **GitLab Token**: Access Token with API and repository read permissions

### Environment Verification

```bash
# Check Kubernetes connectivity
kubectl cluster-info

# Check Helm version
helm version

# Check Tekton Pipelines
kubectl get pods -n tekton-pipelines

# Check namespace permissions
kubectl auth can-i create deployments --namespace=sentry-system
```

## 🚀 Method 1: Helm Chart Deployment (Recommended)

### 1. Quick Start

#### Basic Deployment

```bash
# Clone the project (if not already done)
git clone <your-repo-url>
cd Sentry

# Install with default configuration
helm install sentry ./helm/sentry \
  --create-namespace \
  --namespace sentry-system \
  --set secrets.githubToken="your_github_token_here" \
  --set secrets.gitlabToken="your_gitlab_token_here"
```

#### Verify Deployment

```bash
# Check Pod status
kubectl get pods -n sentry-system

# View logs
kubectl logs -f deployment/sentry -n sentry-system

# Check service status
kubectl get all -n sentry-system
```

### 2. Custom Configuration Deployment

#### Create Custom Values File

Create `my-values.yaml`:

```yaml
# Image configuration
image:
  repository: your-registry/sentry
  tag: "1.0.0"
  pullPolicy: IfNotPresent

# Resource configuration
resources:
  requests:
    memory: "256Mi"
    cpu: "200m"
  limits:
    memory: "512Mi"
    cpu: "500m"

# Application configuration
config:
  pollingInterval: 60
  
  # Global group configurations
  groups:
    ai-projects:
      execution_strategy: "parallel"
      max_parallel: 3
      continue_on_error: true
      global_timeout: 900
    
    critical-services:
      execution_strategy: "sequential"
      max_parallel: 1
      continue_on_error: false
      global_timeout: 1200

  # Repository configurations
  repositories:
    - name: "rag-pipeline"
      group: "ai-projects"
      monitor:
        repo_url: "https://github.com/company/rag-service"
        branches: ["main", "develop"]
        repo_type: "github"
        auth:
          username: "${GITHUB_USERNAME}"
          token: "${GITHUB_TOKEN}"
      deploy:
        qa_repo_url: "https://gitlab.company.com/qa/pipelines"
        qa_repo_branch: "main"
        repo_type: "gitlab"
        auth:
          username: "${GITLAB_USERNAME}"
          token: "${GITLAB_TOKEN}"
        project_name: "rag"
        commands:
          - "cd .tekton/rag"
          - "kubectl apply -f . --namespace=tekton-pipelines"
          - "./scripts/verify-deployment.sh"
      webhook_url: ""

    - name: "chatbot-pipeline"
      group: "ai-projects"
      monitor:
        repo_url: "https://github.com/company/chatbot-service"
        branches: ["main"]
        repo_type: "github"
        auth:
          username: "${GITHUB_USERNAME}"
          token: "${GITHUB_TOKEN}"
      deploy:
        qa_repo_url: "https://gitlab.company.com/qa/pipelines"
        qa_repo_branch: "main"
        repo_type: "gitlab"
        auth:
          username: "${GITLAB_USERNAME}"
          token: "${GITLAB_TOKEN}"
        project_name: "chatbot"
        commands:
          - "cd .tekton/chatbot"
          - "kubectl apply -f . --namespace=tekton-pipelines"

  # Global settings
  global:
    tmp_dir: "/tmp/sentry"
    cleanup: true
    log_level: "info"
    timeout: 300

# Secret configuration
secrets:
  githubToken: "your_github_token"
  gitlabToken: "your_gitlab_token"

# Security configuration
rbac:
  create: true
  rules:
    - apiGroups: [""]
      resources: ["configmaps", "secrets"]
      verbs: ["get", "list", "watch"]
    - apiGroups: ["apps"]
      resources: ["deployments"]
      verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
    - apiGroups: ["tekton.dev"]
      resources: ["pipelines", "pipelineruns", "tasks", "taskruns"]
      verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Auto-scaling (optional)
autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 3
  targetCPUUtilizationPercentage: 80
```

#### Deploy with Custom Configuration

```bash
# Deploy
helm install sentry ./helm/sentry \
  --create-namespace \
  --namespace sentry-system \
  -f my-values.yaml

# Upgrade existing deployment
helm upgrade sentry ./helm/sentry \
  --namespace sentry-system \
  -f my-values.yaml
```

### 3. Environment-Specific Deployment

#### Development Environment

```bash
# Deploy with development configuration
helm install sentry-dev ./helm/sentry \
  --create-namespace \
  --namespace sentry-dev \
  -f ./helm/sentry/values-dev.yaml \
  --set secrets.githubToken="$GITHUB_TOKEN" \
  --set secrets.gitlabToken="$GITLAB_TOKEN"
```

#### Production Environment

```bash
# Deploy with production configuration
helm install sentry-prod ./helm/sentry \
  --create-namespace \
  --namespace sentry-prod \
  -f ./helm/sentry/values-production.yaml \
  --set secrets.githubToken="$GITHUB_TOKEN" \
  --set secrets.gitlabToken="$GITLAB_TOKEN"
```

### 4. Helm Management Operations

#### View Deployment Status

```bash
# List all Helm releases
helm list -A

# Check specific release status
helm status sentry -n sentry-system

# View release history
helm history sentry -n sentry-system
```

#### Upgrade and Rollback

```bash
# Upgrade deployment
helm upgrade sentry ./helm/sentry -n sentry-system

# Rollback to previous version
helm rollback sentry -n sentry-system

# Rollback to specific version
helm rollback sentry 2 -n sentry-system
```

#### Uninstall Deployment

```bash
# Uninstall Helm release
helm uninstall sentry -n sentry-system

# Delete namespace (optional)
kubectl delete namespace sentry-system
```

### 5. Troubleshooting

#### Common Issue Diagnosis

```bash
# Check Pod status
kubectl describe pod -l app.kubernetes.io/name=sentry -n sentry-system

# View container logs
kubectl logs -f deployment/sentry -n sentry-system

# Check configuration
kubectl get configmap sentry-config -n sentry-system -o yaml

# Check secrets
kubectl get secret sentry-secrets -n sentry-system

# Check RBAC permissions
kubectl auth can-i --list --as=system:serviceaccount:sentry-system:sentry
```

#### Common Errors and Solutions

| Error | Cause | Solution |
|-------|-------|----------|
| `ImagePullBackOff` | Unable to pull image | Check image tag and repository permissions |
| `CrashLoopBackOff` | Application startup failure | Check configuration file and environment variables |
| `Authentication failed` | Invalid token | Verify GitHub/GitLab token permissions |
| `Permission denied` | Insufficient RBAC permissions | Check ServiceAccount permission configuration |

## 🔧 Method 2: Raw YAML Manifest Deployment

If not using Helm, you can use raw Kubernetes YAML manifests:

### 1. Prepare Configuration

```bash
# Copy environment variable template
cp env.example .env

# Edit environment variables
vi .env
```

### 2. Create Secrets

```bash
# Create namespace
kubectl apply -f k8s/01-namespace.yaml

# Create secrets
kubectl create secret generic sentry-secrets \
  --from-literal=github-token="your_github_token" \
  --from-literal=gitlab-token="your_gitlab_token" \
  -n sentry-system
```

### 3. Deploy Application

```bash
# Deploy all components in order
kubectl apply -f k8s/02-secret.yaml
kubectl apply -f k8s/03-configmap.yaml
kubectl apply -f k8s/04-rbac.yaml
kubectl apply -f k8s/05-deployment.yaml

# Or deploy all at once
kubectl apply -f k8s/
```

### 4. Verify Deployment

```bash
kubectl get all -n sentry-system
```

## 🔧 Method 3: Docker Deployment (Local Testing)

Suitable for local development and testing:

### 1. Build Image

```bash
# Build Docker image
make docker

# Or manually build
docker build -t sentry:latest .
```

### 2. Run Container

```bash
# Create environment variable file
cat > .env << EOF
GITHUB_USERNAME=your_username
GITHUB_TOKEN=your_github_token
GITLAB_USERNAME=your_username
GITLAB_TOKEN=your_gitlab_token
EOF

# Run container
docker run -d \
  --name sentry \
  --env-file .env \
  -v $(pwd)/sentry.yaml:/app/sentry.yaml:ro \
  -v ~/.kube/config:/root/.kube/config:ro \
  sentry:latest -action=watch
```

## 📊 Monitoring and Maintenance

### 1. Health Checks

```bash
# Check application status
kubectl exec -it deployment/sentry -n sentry-system -- ./sentry -action=validate

# View configuration
kubectl exec -it deployment/sentry -n sentry-system -- cat /app/sentry.yaml
```

### 2. Log Management

```bash
# Real-time log viewing
kubectl logs -f deployment/sentry -n sentry-system

# View historical logs
kubectl logs deployment/sentry -n sentry-system --previous

# View logs for specific time period
kubectl logs deployment/sentry -n sentry-system --since=1h
```

### 3. Performance Monitoring

```bash
# View resource usage
kubectl top pod -n sentry-system

# View events
kubectl get events -n sentry-system --sort-by='.lastTimestamp'
```

## 🔒 Security Best Practices

### 1. Secret Management

- Use Kubernetes Secrets to store sensitive information
- Regularly rotate access tokens
- Limit token permission scope
- Consider using external secret management systems (like HashiCorp Vault)

### 2. RBAC Configuration

- Use the principle of least privilege
- Use different ServiceAccounts for different environments
- Regularly review and update permissions

### 3. Network Security

```yaml
# Network policy example
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: sentry-network-policy
  namespace: sentry-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: sentry
  policyTypes:
  - Egress
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS access to Git repositories
    - protocol: TCP
      port: 6443 # Kubernetes API
```

## 🚀 Advanced Configuration

### 1. Multi-Cluster Deployment

```bash
# Use different values files for different clusters
helm install sentry-cluster1 ./helm/sentry -f values-cluster1.yaml
helm install sentry-cluster2 ./helm/sentry -f values-cluster2.yaml
```

### 2. Auto-scaling

```yaml
# Enable HPA
autoscaling:
  enabled: true
  minReplicas: 1
  maxReplicas: 5
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
```

### 3. Persistent Storage (if needed)

```yaml
# Use persistent volume for temporary files
persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 10Gi
  mountPath: /tmp/sentry
```

## 📝 Configuration Reference

### Complete Helm Values Configuration

See `helm/sentry/values.yaml` for detailed descriptions of all configurable options.

### Environment Variable Reference

| Variable Name | Description | Required |
|---------------|-------------|----------|
| `GITHUB_USERNAME` | GitHub username | Yes |
| `GITHUB_TOKEN` | GitHub access token | Yes |
| `GITLAB_USERNAME` | GitLab username | Yes |
| `GITLAB_TOKEN` | GitLab access token | Yes |

### Configuration File Reference

See `sentry.yaml` for complete configuration file format and options.

---

## 📞 Support and Help

If you encounter issues during deployment:

1. Check the [Troubleshooting section](#5-troubleshooting)
2. Review application logs and Kubernetes events
3. Refer to the project's main README documentation
4. Submit an issue to the project repository

Happy deploying! 🎉
