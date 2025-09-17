# Sentry Helm Chart

This Helm chart deploys Sentry - Tekton Pipeline Auto-Deployer to a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.20+
- Helm 3.0+
- Tekton Pipelines installed in your cluster
- GitHub and/or GitLab access tokens

## Installation

### Quick Start

1. **Add required tokens to values:**

```bash
# Create a custom values file
cat > my-values.yaml << EOF
secrets:
  githubToken: "your_github_token_here"
  gitlabToken: "your_gitlab_token_here"

config:
  monitor:
    repo_a:
      type: "github"
      url: "https://github.com/your-org/your-repo"
      branch: "main"
      token: "\${GITHUB_TOKEN}"
EOF
```

2. **Install the chart:**

```bash
# Install with custom values
helm install sentry ./helm/sentry -f my-values.yaml

# Or install with inline values
helm install sentry ./helm/sentry \
  --set secrets.githubToken="your_github_token" \
  --set secrets.gitlabToken="your_gitlab_token"
```

### Environment-Specific Deployments

#### Development Environment

```bash
helm install sentry-dev ./helm/sentry -f ./helm/sentry/values-dev.yaml
```

#### Production Environment

```bash
# First create production secrets
kubectl create secret generic sentry-production-tokens \
  --from-literal=github-token="your_production_github_token" \
  --from-literal=gitlab-token="your_production_gitlab_token"

# Install with production values
helm install sentry-prod ./helm/sentry -f ./helm/sentry/values-production.yaml
```

## Configuration

### Repository Configuration

Configure the repositories to monitor in `values.yaml`:

```yaml
config:
  monitor:
    repo_a:
      type: "github"              # or "gitlab"
      url: "repository_url"
      branch: "branch_name"
      token: "${GITHUB_TOKEN}"    # Environment variable reference
    
    repo_b:
      type: "gitlab"
      url: "repository_url"
      branch: "branch_name"
      token: "${GITLAB_TOKEN}"
    
    poll:
      interval: 30               # Poll interval in seconds
      timeout: 10                # Request timeout in seconds
```

### Secret Management

#### Option 1: Let Helm create secrets (for development)

```yaml
secrets:
  create: true
  githubToken: "your_github_token"
  gitlabToken: "your_gitlab_token"
```

#### Option 2: Use existing secrets (recommended for production)

```yaml
secrets:
  create: false
  existingSecret: "my-existing-secret"
```

Create the secret manually:

```bash
kubectl create secret generic my-existing-secret \
  --from-literal=github-token="your_github_token" \
  --from-literal=gitlab-token="your_gitlab_token"
```

### Resource Configuration

Adjust resource limits and requests:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 64Mi
```

### Autoscaling

Enable horizontal pod autoscaling:

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 5
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80
```

### Security Context

Configure pod and container security contexts:

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  fsGroup: 1000

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
```

## Values Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `localhost:5000/sentry` |
| `image.tag` | Image tag | `1.0.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `replicaCount` | Number of replicas | `1` |
| `serviceAccount.create` | Create service account | `true` |
| `secrets.create` | Create secret for tokens | `true` |
| `secrets.githubToken` | GitHub access token | `""` |
| `secrets.gitlabToken` | GitLab access token | `""` |
| `secrets.existingSecret` | Use existing secret | `""` |
| `rbac.create` | Create RBAC resources | `true` |
| `namespace.create` | Create namespace | `true` |
| `namespace.name` | Namespace name | `sentry-system` |
| `autoscaling.enabled` | Enable autoscaling | `false` |
| `verbose` | Enable verbose logging | `true` |

For a complete list of values, see `values.yaml`.

## Commands

### Deployment Management

```bash
# Install
helm install sentry ./helm/sentry

# Upgrade
helm upgrade sentry ./helm/sentry

# Uninstall
helm uninstall sentry

# Get status
helm status sentry

# Get values
helm get values sentry
```

### Application Operations

```bash
# View logs
kubectl logs -f deployment/sentry -n sentry-system

# Validate configuration
kubectl exec -it deployment/sentry -n sentry-system -- \
  sentry -action=validate -config=/etc/sentry/sentry.yaml

# Manual trigger
kubectl exec -it deployment/sentry -n sentry-system -- \
  sentry -action=trigger -config=/etc/sentry/sentry.yaml

# Check deployment status
kubectl get deployment sentry -n sentry-system
```

## Validation

### Chart Validation

```bash
# Lint the chart
helm lint ./helm/sentry

# Validate templates
helm template sentry ./helm/sentry --debug

# Dry run installation
helm install sentry ./helm/sentry --dry-run --debug
```

### Deployment Validation

```bash
# Check pods
kubectl get pods -n sentry-system

# Check events
kubectl get events -n sentry-system --sort-by='.lastTimestamp'

# Check logs
kubectl logs -f deployment/sentry -n sentry-system
```

## Troubleshooting

### Common Issues

1. **ImagePullBackOff**: Check image repository and tag
2. **CrashLoopBackOff**: Check configuration and logs
3. **Permission Denied**: Verify RBAC configuration
4. **Authentication Errors**: Check token validity

### Debug Commands

```bash
# Describe pod for detailed information
kubectl describe pod -l app.kubernetes.io/name=sentry -n sentry-system

# Check secret contents
kubectl get secret sentry-tokens -n sentry-system -o yaml

# Check configmap contents
kubectl get configmap sentry-config -n sentry-system -o yaml

# Test configuration manually
kubectl run debug --image=localhost:5000/sentry:1.0.0 --rm -it -- \
  sentry -action=validate -config=/etc/sentry/sentry.yaml
```

## Upgrading

### Chart Upgrades

```bash
# Upgrade to new chart version
helm upgrade sentry ./helm/sentry

# Upgrade with new values
helm upgrade sentry ./helm/sentry -f new-values.yaml

# Force upgrade (recreate pods)
helm upgrade sentry ./helm/sentry --force
```

### Application Upgrades

Update the image tag in values and upgrade:

```yaml
image:
  tag: "1.1.0"  # New version
```

```bash
helm upgrade sentry ./helm/sentry
```

## Security Considerations

1. **Use dedicated namespaces** for different environments
2. **Store sensitive data in secrets**, not values files
3. **Use existing secrets** in production
4. **Enable security contexts** and run as non-root
5. **Limit RBAC permissions** to minimum required
6. **Regularly rotate access tokens**

## Support

For support with the Helm chart:

- Check the main project documentation
- Review troubleshooting section above
- Create an issue with chart-specific problems
- Consult Helm documentation for general Helm issues
