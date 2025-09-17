# Sentry Deployment Guide

This guide provides detailed instructions for deploying Sentry in various environments.

## Prerequisites

Before deploying Sentry, ensure you have:

1. **Kubernetes Cluster**: Version 1.20 or later
2. **Tekton Pipelines**: Installed in your cluster
3. **kubectl**: Configured to access your cluster
4. **Access Tokens**: For GitHub and/or GitLab repositories
5. **RBAC Permissions**: To create resources in the target namespace

## Deployment Options

### Option 1: Kubernetes Deployment (Recommended)

#### Step 1: Prepare Secrets

1. Encode your tokens:

```bash
# GitHub token
echo -n "ghp_your_github_token_here" | base64

# GitLab token
echo -n "glpat_your_gitlab_token_here" | base64
```

2. Update `k8s/02-secret.yaml` with the encoded values:

```yaml
data:
  github-token: <base64_encoded_github_token>
  gitlab-token: <base64_encoded_gitlab_token>
```

#### Step 2: Configure Repositories

Edit `k8s/03-configmap.yaml` to specify your repositories:

```yaml
data:
  sentry.yaml: |
    monitor:
      repo_a:
        type: "github"
        url: "https://github.com/your-org/your-repo"
        branch: "main"
        token: "${GITHUB_TOKEN}"
      
      repo_b:
        type: "gitlab"
        url: "https://gitlab.com/your-org/your-repo"
        branch: "main"
        token: "${GITLAB_TOKEN}"
```

#### Step 3: Deploy

```bash
# Apply all Kubernetes manifests
kubectl apply -f k8s/

# Verify deployment
kubectl get pods -n sentry-system
kubectl logs -f deployment/sentry -n sentry-system
```

#### Step 4: Validate

```bash
# Check if Sentry can access repositories
kubectl exec -it deployment/sentry -n sentry-system -- \
  sentry -action=validate -config=/etc/sentry/sentry.yaml
```

### Option 2: Docker Deployment

#### Step 1: Build Image

```bash
# Build the Docker image
make docker

# Or pull from registry
docker pull localhost:5000/sentry:1.0.0
```

#### Step 2: Prepare Configuration

Create a local `sentry.yaml` file with your configuration.

#### Step 3: Run Container

```bash
docker run -d \
  --name sentry \
  -v $(pwd)/sentry.yaml:/app/sentry.yaml \
  -v /path/to/kubeconfig:/root/.kube/config \
  -e GITHUB_TOKEN=your_github_token \
  -e GITLAB_TOKEN=your_gitlab_token \
  localhost:5000/sentry:1.0.0 \
  -action=watch -verbose
```

### Option 3: Local Binary

#### Step 1: Build

```bash
make build
```

#### Step 2: Configure

```bash
# Set environment variables
export GITHUB_TOKEN="your_github_token"
export GITLAB_TOKEN="your_gitlab_token"

# Ensure kubectl is configured
kubectl cluster-info
```

#### Step 3: Run

```bash
# Validate configuration
./build/sentry -action=validate

# Start monitoring
./build/sentry -action=watch -verbose
```

## Configuration Reference

### Repository Configuration

```yaml
monitor:
  repo_a:
    type: "github"        # or "gitlab"
    url: "repository_url"
    branch: "branch_name"
    token: "${TOKEN_VAR}"
  
  poll:
    interval: 30          # Poll interval in seconds
    timeout: 10           # Request timeout in seconds
```

### Deployment Configuration

```yaml
deploy:
  namespace: "tekton-pipelines"  # Target namespace for Tekton resources
  tmp_dir: "/tmp/sentry"         # Temporary directory for cloning
  cleanup: true                  # Auto-cleanup temporary files
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `GITHUB_TOKEN` | GitHub personal access token | Yes (if using GitHub) |
| `GITLAB_TOKEN` | GitLab access token | Yes (if using GitLab) |
| `SENTRY_CONFIG_PATH` | Path to configuration file | No (default: sentry.yaml) |
| `SENTRY_TMP_DIR` | Temporary directory override | No |

## RBAC Configuration

Sentry requires the following permissions:

```yaml
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]
- apiGroups: ["tekton.dev"]
  resources: ["pipelines", "pipelineruns", "tasks", "taskruns"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
- apiGroups: ["triggers.tekton.dev"]
  resources: ["triggerbindings", "triggertemplates", "eventlisteners"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
```

## Monitoring and Logging

### Health Checks

Sentry includes built-in health checks:

- **Liveness Probe**: Validates configuration every 60 seconds
- **Readiness Probe**: Checks repository connectivity every 30 seconds

### Logging

Configure logging verbosity:

```bash
# Verbose logging
sentry -action=watch -verbose

# Normal logging (default)
sentry -action=watch
```

Log levels include:
- `DEBUG`: Detailed API calls and operations
- `INFO`: General information and status updates
- `WARN`: Warnings and retry attempts
- `ERROR`: Errors and failures

### Monitoring Commands

```bash
# Check deployment status
kubectl get deployment sentry -n sentry-system

# View logs
kubectl logs -f deployment/sentry -n sentry-system

# Check resource usage
kubectl top pod -n sentry-system

# Validate configuration
kubectl exec -it deployment/sentry -n sentry-system -- \
  sentry -action=validate
```

## Troubleshooting

### Common Issues

#### 1. Authentication Errors

**Problem**: "401 Unauthorized" errors

**Solution**:
- Verify token validity
- Check token permissions (repo access required)
- Ensure tokens are correctly base64 encoded in secrets

#### 2. Repository Access Issues

**Problem**: "Repository not found" errors

**Solution**:
- Verify repository URLs
- Check branch names
- Ensure tokens have access to specified repositories

#### 3. Tekton Deployment Failures

**Problem**: "kubectl apply failed" errors

**Solution**:
- Verify RBAC permissions
- Check if tekton-pipelines namespace exists
- Ensure Tekton CRDs are installed

#### 4. Network Connectivity

**Problem**: Timeout errors

**Solution**:
- Check cluster's egress connectivity
- Verify DNS resolution
- Adjust timeout settings in configuration

### Debug Commands

```bash
# Test repository connectivity
sentry -action=validate -verbose

# Manual deployment trigger
sentry -action=trigger

# Check Kubernetes connectivity
kubectl cluster-info

# Verify Tekton installation
kubectl get crd | grep tekton
```

## Security Best Practices

1. **Use dedicated service accounts** with minimal required permissions
2. **Store sensitive data in Secrets**, not ConfigMaps
3. **Enable security contexts** in pod specifications
4. **Use read-only root filesystems** where possible
5. **Run containers as non-root users**
6. **Regularly rotate access tokens**
7. **Monitor resource access** and API calls

## Performance Tuning

### Resource Limits

Adjust based on your workload:

```yaml
resources:
  requests:
    memory: "64Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "500m"
```

### Polling Frequency

Balance between responsiveness and API rate limits:

```yaml
poll:
  interval: 30    # Increase for less frequent checks
  timeout: 10     # Adjust based on network latency
```

### Replica Configuration

For high availability:

```yaml
spec:
  replicas: 2     # Multiple instances for redundancy
```

## Backup and Recovery

### Configuration Backup

```bash
# Backup Kubernetes manifests
kubectl get secret sentry-tokens -n sentry-system -o yaml > backup-secret.yaml
kubectl get configmap sentry-config -n sentry-system -o yaml > backup-config.yaml
```

### Recovery Procedure

```bash
# Restore from backup
kubectl apply -f backup-secret.yaml
kubectl apply -f backup-config.yaml
kubectl rollout restart deployment/sentry -n sentry-system
```

## Upgrading

### Version Upgrade

1. Update the image tag in `k8s/05-deployment.yaml`
2. Apply the updated manifest:

```bash
kubectl apply -f k8s/05-deployment.yaml
```

3. Monitor the rollout:

```bash
kubectl rollout status deployment/sentry -n sentry-system
```

### Configuration Updates

1. Update the ConfigMap:

```bash
kubectl apply -f k8s/03-configmap.yaml
```

2. Restart the deployment to pick up changes:

```bash
kubectl rollout restart deployment/sentry -n sentry-system
```

## Support

For additional support:

- Review logs: `kubectl logs -f deployment/sentry -n sentry-system`
- Check the main README.md for general usage
- Consult the architecture documentation for technical details
- Create an issue for bugs or feature requests
