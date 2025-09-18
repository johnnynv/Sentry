# Sentry 部署指南

本文档详细介绍如何在Kubernetes集群中部署Sentry - Tekton Pipeline自动部署器。我们推荐使用Helm Chart进行部署，同时提供原始YAML清单等其他部署方式作为备选。

## 📋 前置条件

### 必需组件

- **Kubernetes集群**: 1.20+
- **Helm**: 3.0+
- **Tekton Pipelines**: 已安装在集群中
- **kubectl**: 配置好集群访问权限

### 访问凭证

- **GitHub Token**: 具有仓库读取权限的Personal Access Token
- **GitLab Token**: 具有API和仓库读取权限的Access Token

### 验证环境

```bash
# 检查Kubernetes连接
kubectl cluster-info

# 检查Helm版本
helm version

# 检查Tekton Pipelines
kubectl get pods -n tekton-pipelines

# 检查命名空间权限
kubectl auth can-i create deployments --namespace=sentry-system
```

## 🚀 方式一：Helm Chart部署（推荐）

### 1. 快速开始

#### 基础部署

```bash
# 克隆项目（如果还没有）
git clone <your-repo-url>
cd Sentry

# 使用默认配置安装
helm install sentry ./helm/sentry \
  --create-namespace \
  --namespace sentry-system \
  --set secrets.githubToken="your_github_token_here" \
  --set secrets.gitlabToken="your_gitlab_token_here"
```

#### 验证部署

```bash
# 查看Pod状态
kubectl get pods -n sentry-system

# 查看日志
kubectl logs -f deployment/sentry -n sentry-system

# 查看服务状态
kubectl get all -n sentry-system
```

### 2. 自定义配置部署

#### 创建自定义值文件

创建 `my-values.yaml`:

```yaml
# 镜像配置
image:
  repository: your-registry/sentry
  tag: "1.0.0"
  pullPolicy: IfNotPresent

# 资源配置
resources:
  requests:
    memory: "256Mi"
    cpu: "200m"
  limits:
    memory: "512Mi"
    cpu: "500m"

# 应用配置
config:
  pollingInterval: 60
  
  # 全局组配置
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

  # 仓库配置
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

  # 全局设置
  global:
    tmp_dir: "/tmp/sentry"
    cleanup: true
    log_level: "info"
    timeout: 300

# 密钥配置
secrets:
  githubToken: "your_github_token"
  gitlabToken: "your_gitlab_token"

# 安全配置
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

# 自动扩缩容（可选）
autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 3
  targetCPUUtilizationPercentage: 80
```

#### 使用自定义配置部署

```bash
# 部署
helm install sentry ./helm/sentry \
  --create-namespace \
  --namespace sentry-system \
  -f my-values.yaml

# 升级现有部署
helm upgrade sentry ./helm/sentry \
  --namespace sentry-system \
  -f my-values.yaml
```

### 3. 环境特定部署

#### 开发环境

```bash
# 使用开发环境配置
helm install sentry-dev ./helm/sentry \
  --create-namespace \
  --namespace sentry-dev \
  -f ./helm/sentry/values-dev.yaml \
  --set secrets.githubToken="$GITHUB_TOKEN" \
  --set secrets.gitlabToken="$GITLAB_TOKEN"
```

#### 生产环境

```bash
# 使用生产环境配置
helm install sentry-prod ./helm/sentry \
  --create-namespace \
  --namespace sentry-prod \
  -f ./helm/sentry/values-production.yaml \
  --set secrets.githubToken="$GITHUB_TOKEN" \
  --set secrets.gitlabToken="$GITLAB_TOKEN"
```

### 4. Helm管理操作

#### 查看部署状态

```bash
# 列出所有Helm发布
helm list -A

# 查看特定发布状态
helm status sentry -n sentry-system

# 查看发布历史
helm history sentry -n sentry-system
```

#### 升级和回滚

```bash
# 升级部署
helm upgrade sentry ./helm/sentry -n sentry-system

# 回滚到上一个版本
helm rollback sentry -n sentry-system

# 回滚到特定版本
helm rollback sentry 2 -n sentry-system
```

#### 卸载部署

```bash
# 卸载Helm发布
helm uninstall sentry -n sentry-system

# 删除命名空间（可选）
kubectl delete namespace sentry-system
```

### 5. 故障排查

#### 常见问题诊断

```bash
# 检查Pod状态
kubectl describe pod -l app.kubernetes.io/name=sentry -n sentry-system

# 查看容器日志
kubectl logs -f deployment/sentry -n sentry-system

# 检查配置
kubectl get configmap sentry-config -n sentry-system -o yaml

# 检查密钥
kubectl get secret sentry-secrets -n sentry-system

# 检查RBAC权限
kubectl auth can-i --list --as=system:serviceaccount:sentry-system:sentry
```

#### 常见错误及解决方案

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `ImagePullBackOff` | 镜像无法拉取 | 检查镜像标签和仓库权限 |
| `CrashLoopBackOff` | 应用启动失败 | 检查配置文件和环境变量 |
| `Authentication failed` | Token无效 | 验证GitHub/GitLab Token权限 |
| `Permission denied` | RBAC权限不足 | 检查ServiceAccount权限配置 |

## 🔧 方式二：原始YAML清单部署

如果不使用Helm，可以使用原始Kubernetes YAML清单：

### 1. 准备配置

```bash
# 复制环境变量模板
cp env.example .env

# 编辑环境变量
vi .env
```

### 2. 创建密钥

```bash
# 创建命名空间
kubectl apply -f k8s/01-namespace.yaml

# 创建密钥
kubectl create secret generic sentry-secrets \
  --from-literal=github-token="your_github_token" \
  --from-literal=gitlab-token="your_gitlab_token" \
  -n sentry-system
```

### 3. 部署应用

```bash
# 按顺序部署所有组件
kubectl apply -f k8s/02-secret.yaml
kubectl apply -f k8s/03-configmap.yaml
kubectl apply -f k8s/04-rbac.yaml
kubectl apply -f k8s/05-deployment.yaml

# 或一次性部署
kubectl apply -f k8s/
```

### 4. 验证部署

```bash
kubectl get all -n sentry-system
```

## 🔧 方式三：Docker部署（本地测试）

适用于本地开发和测试：

### 1. 构建镜像

```bash
# 构建Docker镜像
make docker

# 或手动构建
docker build -t sentry:latest .
```

### 2. 运行容器

```bash
# 创建环境变量文件
cat > .env << EOF
GITHUB_USERNAME=your_username
GITHUB_TOKEN=your_github_token
GITLAB_USERNAME=your_username
GITLAB_TOKEN=your_gitlab_token
EOF

# 运行容器
docker run -d \
  --name sentry \
  --env-file .env \
  -v $(pwd)/sentry.yaml:/app/sentry.yaml:ro \
  -v ~/.kube/config:/root/.kube/config:ro \
  sentry:latest -action=watch
```

## 📊 监控和维护

### 1. 健康检查

```bash
# 检查应用状态
kubectl exec -it deployment/sentry -n sentry-system -- ./sentry -action=validate

# 查看配置
kubectl exec -it deployment/sentry -n sentry-system -- cat /app/sentry.yaml
```

### 2. 日志管理

```bash
# 实时查看日志
kubectl logs -f deployment/sentry -n sentry-system

# 查看历史日志
kubectl logs deployment/sentry -n sentry-system --previous

# 查看特定时间段日志
kubectl logs deployment/sentry -n sentry-system --since=1h
```

### 3. 性能监控

```bash
# 查看资源使用
kubectl top pod -n sentry-system

# 查看事件
kubectl get events -n sentry-system --sort-by='.lastTimestamp'
```

## 🔒 安全最佳实践

### 1. 密钥管理

- 使用Kubernetes Secrets存储敏感信息
- 定期轮换访问Token
- 限制Token权限范围
- 考虑使用外部密钥管理系统（如HashiCorp Vault）

### 2. RBAC配置

- 使用最小权限原则
- 为不同环境使用不同的ServiceAccount
- 定期审查和更新权限

### 3. 网络安全

```yaml
# 网络策略示例
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
      port: 443  # HTTPS访问Git仓库
    - protocol: TCP
      port: 6443 # Kubernetes API
```

## 🚀 高级配置

### 1. 多集群部署

```bash
# 为不同集群使用不同的values文件
helm install sentry-cluster1 ./helm/sentry -f values-cluster1.yaml
helm install sentry-cluster2 ./helm/sentry -f values-cluster2.yaml
```

### 2. 自动扩缩容

```yaml
# 启用HPA
autoscaling:
  enabled: true
  minReplicas: 1
  maxReplicas: 5
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
```

### 3. 持久化存储（如果需要）

```yaml
# 为临时文件使用持久化卷
persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 10Gi
  mountPath: /tmp/sentry
```

## 📝 配置参考

### 完整的Helm Values配置

查看 `helm/sentry/values.yaml` 了解所有可配置选项的详细说明。

### 环境变量参考

| 变量名 | 说明 | 必需 |
|--------|------|------|
| `GITHUB_USERNAME` | GitHub用户名 | 是 |
| `GITHUB_TOKEN` | GitHub访问Token | 是 |
| `GITLAB_USERNAME` | GitLab用户名 | 是 |
| `GITLAB_TOKEN` | GitLab访问Token | 是 |

### 配置文件参考

查看 `sentry.yaml` 了解完整的配置文件格式和选项。

---

## 📞 支持和帮助

如果在部署过程中遇到问题：

1. 查看[故障排查部分](#5-故障排查)
2. 检查应用日志和Kubernetes事件
3. 参考项目主README文档
4. 提交Issue到项目仓库

祝您部署顺利！🎉
