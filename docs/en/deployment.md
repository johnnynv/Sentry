# Sentry Deployment Guide

This guide provides complete Sentry deployment, operations, and troubleshooting instructions based on actual E2E testing experience.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Deployment](#quick-deployment)
- [Detailed Deployment Steps](#detailed-deployment-steps)
- [Post-Deployment Operations](#post-deployment-operations)
- [Configuration Updates](#configuration-updates)
- [Log Viewing and Debugging](#log-viewing-and-debugging)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Prerequisites

### 1. Prepare GitHub/GitLab Access Tokens
```bash
# GitHub Personal Access Token (requires repo permissions)
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# GitLab Access Token (requires api, read_repository permissions)  
export GITLAB_TOKEN="glpat-xxxxxxxxxxxxxxxxxxxxx"
```

### 2. Prepare Container Image Access
If using private image registry (like GHCR), prepare Docker registry authentication:
```bash
# Create docker registry secret (must be created in each target namespace)
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=your_github_username \
  --docker-password=$GITHUB_TOKEN \
  --namespace=target-namespace
```

### 3. Build and Push Image (if needed)
```bash
# Build image
cd /path/to/sentry
docker build -t ghcr.io/your_username/sentry:1.0.0 .

# Push to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u your_username --password-stdin
docker push ghcr.io/your_username/sentry:1.0.0
```

## Quick Deployment

If you already have the required secrets, use this one-command deployment:

```bash
# Create namespace
kubectl create namespace sentry-system

# Create GitHub/GitLab tokens secret
kubectl create secret generic sentry-tokens \
  --from-literal=github-token="$GITHUB_TOKEN" \
  --from-literal=gitlab-token="$GITLAB_TOKEN" \
  --namespace=sentry-system

# Create image pull secret for GHCR
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=your_github_username \
  --docker-password="$GITHUB_TOKEN" \
  --namespace=sentry-system

# Deploy with Helm
helm install sentry-deployment helm/sentry \
  --namespace sentry-system \
  --set image.repository=ghcr.io/your_username/sentry \
  --set image.tag=1.0.2 \
  --set-json='imagePullSecrets=[{"name":"ghcr-secret"}]' \
  --set config.github.username=your_github_username \
  --set config.gitlab.username=your_gitlab_username \
  --set secrets.create=false \
  --set secrets.existingSecret=sentry-tokens \
  --wait --timeout=300s
```

## Detailed Deployment Steps

### Step 1: Environment Preparation

```bash
# Set variables
export GITHUB_USERNAME="your_github_username"
export GITLAB_USERNAME="your_gitlab_username"
export IMAGE_TAG="1.0.2"

# Verify cluster access
kubectl cluster-info
```

### Step 2: Create Namespace and Secrets

```bash
# Create dedicated namespace
kubectl create namespace sentry-system

# Create tokens secret
kubectl create secret generic sentry-tokens \
  --from-literal=github-token="$GITHUB_TOKEN" \
  --from-literal=gitlab-token="$GITLAB_TOKEN" \
  --namespace=sentry-system

# Verify secret creation
kubectl get secret sentry-tokens -n sentry-system -o yaml
```

### Step 3: Configure Image Access (for Private Registry)

```bash
# Create image pull secret
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username="$GITHUB_USERNAME" \
  --docker-password="$GITHUB_TOKEN" \
  --namespace=sentry-system

# Verify secret
kubectl get secret ghcr-secret -n sentry-system
```

### Step 4: Deploy Sentry via Helm

```bash
# Navigate to project directory
cd /path/to/sentry

# Deploy with complete configuration
helm install sentry-deployment helm/sentry \
  --namespace sentry-system \
  --set image.repository=ghcr.io/$GITHUB_USERNAME/sentry \
  --set image.tag=$IMAGE_TAG \
  --set-json='imagePullSecrets=[{"name":"ghcr-secret"}]' \
  --set config.github.username=$GITHUB_USERNAME \
  --set config.gitlab.username=$GITLAB_USERNAME \
  --set secrets.create=false \
  --set secrets.existingSecret=sentry-tokens \
  --wait --timeout=300s
```

### Step 5: Deployment Verification

```bash
# Check deployment status
kubectl get deployment sentry-deployment -n sentry-system

# Check pod status
kubectl get pods -n sentry-system

# Check service account and RBAC
kubectl get serviceaccount sentry-deployment -n sentry-system
kubectl get clusterrole sentry-deployment
kubectl get clusterrolebinding sentry-deployment

# Verify configuration
kubectl exec deployment/sentry-deployment -n sentry-system -- \
  sentry -action=validate -config=/etc/sentry/sentry.yaml
```

## Post-Deployment Operations

### Manual Trigger Test

```bash
# Manually trigger deployment
kubectl exec deployment/sentry-deployment -n sentry-system -- \
  sentry -action=trigger -config=/etc/sentry/sentry.yaml

# Check Tekton pipeline runs
kubectl get pipelinerun -n tekton-rag --sort-by='.metadata.creationTimestamp'
```

### Real-time Monitoring

```bash
# View real-time logs
kubectl logs -f deployment/sentry-deployment -n sentry-system

# Monitor specific events
kubectl logs deployment/sentry-deployment -n sentry-system | grep "Repository change detected"
```

## Configuration Updates

### Updating Repository Configuration

1. **Edit Helm values**:
```bash
# Edit values file
vim helm/sentry/values.yaml

# Or update via command line
helm upgrade sentry-deployment helm/sentry \
  --namespace sentry-system \
  --set config.polling_interval=30 \
  --reuse-values
```

2. **Update access tokens**:
```bash
# Update tokens secret
kubectl delete secret sentry-tokens -n sentry-system
kubectl create secret generic sentry-tokens \
  --from-literal=github-token="$NEW_GITHUB_TOKEN" \
  --from-literal=gitlab-token="$NEW_GITLAB_TOKEN" \
  --namespace=sentry-system

# Restart deployment to pick up new secrets
kubectl rollout restart deployment/sentry-deployment -n sentry-system
```

### Adding New Repositories

Edit `helm/sentry/values.yaml` and add new repository configuration:

```yaml
repositories:
  - name: "new-project"
    group: "ai-blueprints"
    monitor:
      repo_url: "https://github.com/username/new-repo"
      branches: ["main", "develop"]
      repo_type: "github"
      auth:
        username: "placeholder_github_username"
        token: "${GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab.com/qa/deployment-repo"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      project_name: "new-project"
      commands:
        - "cd .tekton/new-project && export TMPDIR=/tmp/sentry && bash scripts/deploy.sh"
      auth:
        username: "placeholder_gitlab_username"
        token: "${GITLAB_TOKEN}"
```

Then upgrade the deployment:
```bash
helm upgrade sentry-deployment helm/sentry \
  --namespace sentry-system \
  --reuse-values
```

## Log Viewing and Debugging

### Real-time Log Monitoring

```bash
# Follow all logs
kubectl logs -f deployment/sentry-deployment -n sentry-system

# Filter specific events
kubectl logs deployment/sentry-deployment -n sentry-system | grep -E "(ERROR|WARN|Repository change detected)"

# View last N lines
kubectl logs deployment/sentry-deployment -n sentry-system --tail=50
```

### Structured Log Analysis

Sentry uses structured logging. Key log patterns:

```bash
# Repository monitoring events
[2025-09-18 08:42:35] INFO: New commit detected [repo=rag-project] [branch=dev] [old_sha=abc123] [new_sha=def456]

# Deployment events  
[2025-09-18 08:42:35] INFO: Starting group deployment [group=ai-blueprints] [strategy=parallel]

# Command execution
[2025-09-18 08:42:41] INFO: Command executed successfully [repo=rag-project] [step=1] [output_size=403]

# Error events
[2025-09-18 08:42:41] ERROR: Command execution failed [repo=rag-project] [error=exit status 1]
```

### Debug Mode

Enable verbose logging:
```bash
kubectl exec deployment/sentry-deployment -n sentry-system -- \
  sentry -action=watch -verbose -config=/etc/sentry/sentry.yaml
```

## Troubleshooting

### Common Issues and Solutions

#### 1. ImagePullBackOff Error

**Problem**: Pod stuck in `ImagePullBackOff` state
```bash
kubectl describe pod -n sentry-system
# Error: Failed to pull image "ghcr.io/username/sentry:1.0.0": unauthorized
```

**Solution**:
```bash
# Verify image exists and is accessible
docker pull ghcr.io/username/sentry:1.0.0

# Check/recreate image pull secret
kubectl delete secret ghcr-secret -n sentry-system
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username="$GITHUB_USERNAME" \
  --docker-password="$GITHUB_TOKEN" \
  --namespace=sentry-system

# Restart deployment
kubectl rollout restart deployment/sentry-deployment -n sentry-system
```

#### 2. Configuration Validation Failed

**Problem**: Configuration validation errors
```bash
kubectl logs deployment/sentry-deployment -n sentry-system
# FATAL: Config validation failed: polling_interval must be positive
```

**Solution**:
```bash
# Check current config
kubectl get configmap sentry-deployment-config -n sentry-system -o yaml

# Fix via Helm values update
helm upgrade sentry-deployment helm/sentry \
  --namespace sentry-system \
  --set config.polling_interval=60 \
  --reuse-values
```

#### 3. API Authentication Failed

**Problem**: GitHub/GitLab API authentication failures
```bash
# Error: repository check failed: gitHub API error (status 401): Bad credentials
```

**Solution**:
```bash
# Verify tokens
echo $GITHUB_TOKEN | cut -c1-10
echo $GITLAB_TOKEN | cut -c1-10

# Test API access
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user

# Update secrets
kubectl delete secret sentry-tokens -n sentry-system
kubectl create secret generic sentry-tokens \
  --from-literal=github-token="$GITHUB_TOKEN" \
  --from-literal=gitlab-token="$GITLAB_TOKEN" \
  --namespace=sentry-system
```

#### 4. Tekton Command Execution Failed

**Problem**: Deployment commands fail with permission or file system errors
```bash
# Error: mktemp: failed to create file via template '/tmp/tmp.XXXXXXXXXX.yaml': Read-only file system
```

**Solution**: Ensure commands set proper TMPDIR:
```yaml
commands:
  - "cd .tekton/project && export TMPDIR=/tmp/sentry && bash scripts/deploy.sh"
```

#### 5. RBAC Permission Denied

**Problem**: kubectl commands fail with permission errors
```bash
# Error: error when creating resource: User "system:serviceaccount:sentry-system:sentry-deployment" cannot create resource "pipelineruns"
```

**Solution**:
```bash
# Check RBAC configuration
kubectl get clusterrole sentry-deployment -o yaml
kubectl get clusterrolebinding sentry-deployment -o yaml

# Verify service account
kubectl get serviceaccount sentry-deployment -n sentry-system
```

### Health Check Commands

```bash
# Complete system health check
echo "=== System Health Check ==="

echo "1. Pod Status:"
kubectl get pods -n sentry-system

echo "2. Deployment Status:"
kubectl get deployment sentry-deployment -n sentry-system

echo "3. Service Account & RBAC:"
kubectl get serviceaccount sentry-deployment -n sentry-system
kubectl get clusterrole sentry-deployment >/dev/null 2>&1 && echo "ClusterRole: OK" || echo "ClusterRole: MISSING"

echo "4. Secrets:"
kubectl get secret sentry-tokens -n sentry-system >/dev/null 2>&1 && echo "Tokens Secret: OK" || echo "Tokens Secret: MISSING"
kubectl get secret ghcr-secret -n sentry-system >/dev/null 2>&1 && echo "Image Pull Secret: OK" || echo "Image Pull Secret: MISSING"

echo "5. Configuration Validation:"
kubectl exec deployment/sentry-deployment -n sentry-system -- \
  sentry -action=validate -config=/etc/sentry/sentry.yaml
```

## Complete Environment Cleanup Steps

### Manual Cleanup
```bash
echo "=== Sentry Environment Cleanup ==="

# 1. Uninstall Helm release
helm uninstall sentry-deployment -n sentry-system

# 2. Delete namespace (will delete all resources within)
kubectl delete namespace sentry-system

# 3. Clean up RBAC resources (if created at cluster level)
kubectl delete clusterrole sentry-deployment 2>/dev/null || true
kubectl delete clusterrolebinding sentry-deployment 2>/dev/null || true

# 4. Clean up Tekton resources (optional - be careful!)
# kubectl delete pipelinerun -n tekton-rag -l app.kubernetes.io/name=container-deployment

echo "Cleanup completed!"
```

### One-Click Cleanup Script
```bash
#!/bin/bash
# save as cleanup-sentry.sh

echo "=== Sentry Complete Cleanup ==="
read -p "This will delete ALL Sentry resources. Continue? (y/N): " confirm

if [[ $confirm == [yY] || $confirm == [yY][eE][sS] ]]; then
    echo "Starting cleanup..."
    
    # Uninstall Helm release
    helm uninstall sentry-deployment -n sentry-system 2>/dev/null || echo "Helm release not found"
    
    # Delete namespace
    kubectl delete namespace sentry-system 2>/dev/null || echo "Namespace not found"
    
    # Clean up cluster-level RBAC
    kubectl delete clusterrole sentry-deployment 2>/dev/null || echo "ClusterRole not found"
    kubectl delete clusterrolebinding sentry-deployment 2>/dev/null || echo "ClusterRoleBinding not found"
    
    echo "Cleanup completed!"
else
    echo "Cleanup cancelled."
fi
```

### Cleanup Considerations

⚠️ **Important Notes**:

1. **Namespace Deletion**: Deleting namespace will remove ALL resources within, including secrets
2. **RBAC Cleanup**: ClusterRole and ClusterRoleBinding are cluster-level resources  
3. **Tekton Resources**: Consider whether to clean up created PipelineRuns
4. **Backup First**: Export important configurations before cleanup:
   ```bash
   # Backup configurations
   kubectl get configmap sentry-deployment-config -n sentry-system -o yaml > sentry-config-backup.yaml
   kubectl get secret sentry-tokens -n sentry-system -o yaml > sentry-secrets-backup.yaml
   ```

## Best Practices

### Security

1. **Token Management**: 
   - Use environment variables for tokens
   - Rotate tokens regularly
   - Use least-privilege access

2. **RBAC Configuration**:
   - Grant minimal required permissions
   - Use namespace-scoped roles when possible
   - Regular audit of permissions

3. **Image Security**:
   - Use specific image tags (not `latest`)
   - Scan images for vulnerabilities
   - Use private registries for sensitive deployments

### Monitoring

1. **Log Management**:
   - Centralized log aggregation (ELK, Grafana Loki)
   - Set up log alerts for ERROR/FATAL events
   - Regular log rotation

2. **Health Monitoring**:
   - Kubernetes liveness/readiness probes
   - External monitoring of repository connectivity
   - Tekton pipeline success rate monitoring

### Maintenance

1. **Regular Updates**:
   - Update Sentry image tags
   - Keep Helm charts updated
   - Review and update configurations

2. **Backup Strategy**:
   - Export configurations before changes
   - Document custom configurations
   - Test restore procedures

3. **Capacity Planning**:
   - Monitor resource usage
   - Plan for concurrent deployments
   - Set appropriate timeouts and limits

---

For more information, refer to:
- [Architecture Design](architecture.md)
- [Implementation Plan](implementation.md)
- [Project Overview](README.md)