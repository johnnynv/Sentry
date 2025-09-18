# Sentry 部署指南

本指南基于实际E2E测试经验，提供完整的Sentry部署、运维和故障排除指南。

## 目录
- [前置条件](#前置条件)
- [快速部署](#快速部署)
- [部署步骤详解](#部署步骤详解)
- [部署后运维](#部署后运维)
- [配置更新](#配置更新)
- [日志查看与调试](#日志查看与调试)
- [故障排除](#故障排除)
- [最佳实践](#最佳实践)

## 前置条件

### 1. 准备GitHub/GitLab访问令牌
```bash
# GitHub Personal Access Token (需要repo权限)
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# GitLab Access Token (需要api, read_repository权限)  
export GITLAB_TOKEN="glpat-xxxxxxxxxxxxxxxxxxxxx"
```

### 2. 准备容器镜像访问
如果使用私有镜像仓库（如GHCR），需要准备Docker registry认证：
```bash
# 创建docker registry secret（需要在每个目标namespace中创建）
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=your_github_username \
  --docker-password=$GITHUB_TOKEN \
  --namespace=target-namespace
```

### 3. 构建和推送镜像（如果需要）
```bash
# 构建镜像
cd /path/to/sentry
docker build -t ghcr.io/your_username/sentry:1.0.0 .

# 推送到GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u your_username --password-stdin
docker push ghcr.io/your_username/sentry:1.0.0
```

## 快速部署

如果您已经有了所需的secrets，可以使用以下一键部署命令：

```bash
# 确保已在目标namespace中创建了ghcr-secret
kubectl create namespace sentry-system
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=your_github_username \
  --docker-password=$GITHUB_TOKEN \
  --namespace=sentry-system

# 创建token secrets
kubectl create secret generic sentry-tokens \
  --from-literal=github-token=$GITHUB_TOKEN \
  --from-literal=gitlab-token=$GITLAB_TOKEN \
  --namespace=sentry-system

# 一键部署
helm install sentry-deployment helm/sentry \
  --namespace sentry-system \
  --set image.repository=ghcr.io/your_username/sentry \
  --set image.tag=1.0.0 \
  --set-json='imagePullSecrets=[{"name":"ghcr-secret"}]' \
  --set config.github.username=your_github_username \
  --set config.gitlab.username=your_gitlab_username \
  --set secrets.create=false \
  --set secrets.existingSecret=sentry-tokens \
  --wait --timeout=300s
```

## 部署步骤详解

### 第1步：创建Namespace
```bash
# 方式1：让Helm管理namespace（推荐）
# 在Helm install时使用 --create-namespace

# 方式2：手动创建（如果需要预先配置secrets）
kubectl create namespace sentry-system
```

### 第2步：创建Secrets

#### 创建GitHub/GitLab Token Secret
```bash
kubectl create secret generic sentry-tokens \
  --from-literal=github-token="$GITHUB_TOKEN" \
  --from-literal=gitlab-token="$GITLAB_TOKEN" \
  --namespace=sentry-system
```

#### 创建镜像拉取Secret（如果使用私有镜像）
```bash
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=your_github_username \
  --docker-password="$GITHUB_TOKEN" \
  --namespace=sentry-system
```

### 第3步：验证Helm Chart
```bash
cd /path/to/sentry
helm lint helm/sentry
```

### 第4步：部署应用
```bash
helm install sentry-deployment helm/sentry \
  --namespace sentry-system \
  --create-namespace \
  --set image.repository=ghcr.io/your_username/sentry \
  --set image.tag=1.0.0 \
  --set-json='imagePullSecrets=[{"name":"ghcr-secret"}]' \
  --set config.github.username=your_github_username \
  --set config.gitlab.username=your_gitlab_username \
  --set secrets.create=false \
  --set secrets.existingSecret=sentry-tokens \
  --wait --timeout=300s
```

### 第5步：验证部署
```bash
# 检查部署状态
kubectl get deployment sentry-deployment -n sentry-system

# 检查Pod状态
kubectl get pods -n sentry-system

# 查看启动日志
kubectl logs deployment/sentry-deployment -n sentry-system
```

## 部署后运维

### 验证配置
部署完成后，验证Sentry配置和仓库连接：
```bash
kubectl exec deployment/sentry-deployment -n sentry-system -- \
  sentry -action=validate -config=/etc/sentry/sentry.yaml
```

预期输出示例：
```
[2025-09-18 07:50:35] INFO: All validation checks passed successfully!
```

### 手动触发部署
测试手动触发功能：
```bash
kubectl exec deployment/sentry-deployment -n sentry-system -- \
  sentry -action=trigger -config=/etc/sentry/sentry.yaml
```

### 检查应用健康状态
```bash
# 查看部署详情
kubectl describe deployment sentry-deployment -n sentry-system

# 查看Pod详情
kubectl get pods -n sentry-system -o wide

# 查看资源使用情况
kubectl top pods -n sentry-system
```

## 配置更新

### 方式1：通过Helm升级更新配置
```bash
# 更新配置参数（例如：修改轮询间隔、仓库配置等）
helm upgrade sentry-deployment helm/sentry \
  --namespace sentry-system \
  --set image.repository=ghcr.io/your_username/sentry \
  --set image.tag=1.0.0 \
  --set-json='imagePullSecrets=[{"name":"ghcr-secret"}]' \
  --set config.github.username=your_github_username \
  --set config.gitlab.username=your_gitlab_username \
  --set secrets.create=false \
  --set secrets.existingSecret=sentry-tokens \
  --set config.polling_interval=90 \
  --wait --timeout=300s
```

### 方式2：直接编辑ConfigMap（临时修改）
```bash
# 编辑ConfigMap
kubectl edit configmap sentry-deployment-config -n sentry-system

# 重启Pod以应用新配置
kubectl rollout restart deployment/sentry-deployment -n sentry-system
```

### 方式3：更新Secrets
```bash
# 更新GitHub token
kubectl create secret generic sentry-tokens \
  --from-literal=github-token="$NEW_GITHUB_TOKEN" \
  --from-literal=gitlab-token="$GITLAB_TOKEN" \
  --namespace=sentry-system \
  --dry-run=client -o yaml | kubectl apply -f -

# 重启Pod以使用新的token
kubectl rollout restart deployment/sentry-deployment -n sentry-system
```

## 日志查看与调试

### 实时查看日志
```bash
# 查看实时监控日志
kubectl logs -f deployment/sentry-deployment -n sentry-system

# 查看最近的日志（指定行数）
kubectl logs deployment/sentry-deployment -n sentry-system --tail=50
```

### 应用日志分析
正常启动日志应该包含：
```
╔═══════════════════════════════════════╗
║           SENTRY v1.0.0                ║
║     Tekton Pipeline Auto-Deployer    ║
╚═══════════════════════════════════════╝

[2025-09-18 07:50:03] INFO: Starting continuous repository monitoring...
[2025-09-18 07:50:03] INFO: Initial commit recorded [repo=xxx] [branch=main] [sha=xxxxxxxx]
[2025-09-18 07:50:03] INFO: Starting monitoring loop (checking every 60 seconds)...
```

### 常见日志模式

#### 成功的仓库检查
```
[2025-09-18 07:50:35] INFO: Repository Monitor repo rag-project:main check successful - Latest commit: 1a82e183 by Author Name
```

#### 认证错误
```
[2025-09-18 07:48:02] FATAL: gitHub API error (status 401): {"message":"Bad credentials"...}
```

#### 配置验证错误
```
[2025-09-18 07:51:35] FATAL: Failed to load configuration: config validation failed: polling_interval must be at least 60 seconds
```

#### 部署执行日志
```
[2025-09-18 07:50:45] INFO: Starting repository deployment [repo=rag-project] [qa_repo=...] [project=rag]
[2025-09-18 07:50:45] INFO: Cloning QA repository [repo=...] [branch=main] [dest=...]
[2025-09-18 07:50:45] INFO: Executing command [repo=rag-project] [step=1] [command=cd .tekton/rag && kubectl apply -f .]
```

### 深度调试

#### 进入容器调试
```bash
# 进入容器
kubectl exec -it deployment/sentry-deployment -n sentry-system -- /bin/sh

# 查看配置文件
cat /etc/sentry/sentry.yaml

# 手动运行命令
sentry -action=validate -config=/etc/sentry/sentry.yaml
```

#### 检查挂载的配置
```bash
# 查看ConfigMap内容
kubectl get configmap sentry-deployment-config -n sentry-system -o yaml

# 查看Secret内容（base64解码）
kubectl get secret sentry-tokens -n sentry-system -o jsonpath='{.data.github-token}' | base64 -d
```

## 故障排除

### 1. Pod一直处于Pending状态
**可能原因：**
- imagePullSecrets配置错误
- 节点资源不足
- RBAC权限问题

**排查步骤：**
```bash
kubectl describe pod <pod-name> -n sentry-system
kubectl get events -n sentry-system --sort-by='.lastTimestamp'
```

### 2. Pod处于ImagePullBackOff状态
**可能原因：**
- 镜像不存在或路径错误
- 私有镜像缺少拉取权限

**解决方案：**
```bash
# 检查镜像是否存在
docker pull ghcr.io/your_username/sentry:1.0.0

# 检查imagePullSecret
kubectl get secret ghcr-secret -n sentry-system

# 重新创建imagePullSecret
kubectl delete secret ghcr-secret -n sentry-system
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=your_username \
  --docker-password="$GITHUB_TOKEN" \
  --namespace=sentry-system
```

### 3. Pod处于CrashLoopBackOff状态
**排查步骤：**
```bash
# 查看Pod日志
kubectl logs <pod-name> -n sentry-system --previous

# 常见错误及解决方案：
# - 配置文件格式错误：检查ConfigMap
# - Token认证失败：检查Secret
# - 配置验证失败：检查配置参数
```

### 4. Token认证失败
**解决步骤：**
```bash
# 1. 验证token有效性
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user

# 2. 检查Secret内容
kubectl get secret sentry-tokens -n sentry-system -o yaml

# 3. 重新创建Secret
kubectl delete secret sentry-tokens -n sentry-system
kubectl create secret generic sentry-tokens \
  --from-literal=github-token="$GITHUB_TOKEN" \
  --from-literal=gitlab-token="$GITLAB_TOKEN" \
  --namespace=sentry-system

# 4. 重启Pod
kubectl rollout restart deployment/sentry-deployment -n sentry-system
```

### 5. 配置更新后Pod仍使用旧配置
**解决方案：**
```bash
# 强制重启Pod
kubectl rollout restart deployment/sentry-deployment -n sentry-system

# 等待新Pod启动
kubectl rollout status deployment/sentry-deployment -n sentry-system

# 验证新配置
kubectl logs deployment/sentry-deployment -n sentry-system --tail=20
```

## 最佳实践

### 1. 部署前检查清单
- [ ] GitHub/GitLab token有效且权限足够
- [ ] 镜像已构建并推送到可访问的仓库
- [ ] 目标namespace已创建必要的secrets
- [ ] Helm Chart语法验证通过
- [ ] 配置参数符合应用要求

### 2. 生产环境配置建议
```yaml
# 建议的生产环境配置
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

# 启用水平扩展（可选）
autoscaling:
  enabled: true
  minReplicas: 1
  maxReplicas: 3
  targetCPUUtilizationPercentage: 80
```

### 3. 监控和告警
```bash
# 设置资源监控
kubectl top pods -n sentry-system

# 设置日志告警（示例）
kubectl logs deployment/sentry-deployment -n sentry-system | grep "ERROR\|FATAL"
```

### 4. 安全最佳实践
- 使用最小权限原则配置RBAC
- 定期轮换访问token
- 不要在配置文件中硬编码敏感信息
- 使用Kubernetes Secrets管理敏感数据
- 限制容器的运行权限

### 5. 维护操作
```bash
# 定期检查应用状态
kubectl get pods -n sentry-system
kubectl logs deployment/sentry-deployment -n sentry-system --tail=10

# 备份重要配置
kubectl get configmap sentry-deployment-config -n sentry-system -o yaml > sentry-config-backup.yaml
kubectl get secret sentry-tokens -n sentry-system -o yaml > sentry-secrets-backup.yaml

# 升级应用
helm upgrade sentry-deployment helm/sentry --namespace sentry-system --set image.tag=1.1.0
```

## 常用运维命令总结

```bash
# 部署状态检查
kubectl get deployment,pods,services -n sentry-system

# 实时日志查看
kubectl logs -f deployment/sentry-deployment -n sentry-system

# 配置验证
kubectl exec deployment/sentry-deployment -n sentry-system -- sentry -action=validate -config=/etc/sentry/sentry.yaml

# 手动触发
kubectl exec deployment/sentry-deployment -n sentry-system -- sentry -action=trigger -config=/etc/sentry/sentry.yaml

# 重启应用
kubectl rollout restart deployment/sentry-deployment -n sentry-system

# 查看配置
kubectl get configmap sentry-deployment-config -n sentry-system -o yaml

# 更新配置
helm upgrade sentry-deployment helm/sentry --namespace sentry-system --set config.polling_interval=120

# 卸载应用
helm uninstall sentry-deployment -n sentry-system
```

## 完整环境清理步骤

当需要完全清理Sentry部署环境时（例如重新部署、迁移环境或解决严重问题），请按以下步骤操作：

### 清理步骤

#### 第1步：查看当前资源状态
```bash
# 查看Helm releases
echo "=== 当前Sentry Helm Releases ==="
helm list --all-namespaces | grep sentry || echo "无Sentry releases"

# 查看相关namespace
echo "=== 相关Namespace ==="
kubectl get namespaces | grep sentry
```

#### 第2步：卸载Helm部署
```bash
# 卸载Sentry Helm release
helm uninstall sentry-deployment -n sentry-system

# 如果有多个release，需要逐一卸载
# helm uninstall <release-name> -n <namespace>
```

#### 第3步：删除Namespace
```bash
# 删除sentry-system namespace（会删除其中所有资源）
kubectl delete namespace sentry-system

# 等待namespace完全删除
kubectl get namespace sentry-system 2>/dev/null && echo "Namespace仍存在，等待删除..." || echo "✅ Namespace已删除"
```

#### 第4步：清理ClusterRole和ClusterRoleBinding资源
```bash
# 查看可能残留的RBAC资源
kubectl get clusterrole | grep sentry
kubectl get clusterrolebinding | grep sentry

# 如果有残留资源，手动删除
# kubectl delete clusterrole sentry-deployment-role
# kubectl delete clusterrolebinding sentry-deployment-rolebinding
```

#### 第5步：清理验证
```bash
# 验证清理完成
echo "=== 清理验证 ==="
echo "Helm releases:"
helm list --all-namespaces | grep sentry || echo "✅ 无Sentry releases"

echo "Namespaces:"
kubectl get namespaces | grep sentry-system || echo "✅ sentry-system namespace已删除"

echo "ClusterRole资源:"
kubectl get clusterrole | grep sentry-deployment || echo "✅ 无残留ClusterRole"

echo "ClusterRoleBinding资源:"
kubectl get clusterrolebinding | grep sentry-deployment || echo "✅ 无残留ClusterRoleBinding"
```

### 一键清理脚本

为方便操作，可以使用以下一键清理脚本：

```bash
#!/bin/bash
# sentry-cleanup.sh - Sentry环境完整清理脚本

echo "🧹 开始清理Sentry环境..."

# 1. 卸载Helm release
echo "📦 卸载Helm release..."
helm uninstall sentry-deployment -n sentry-system 2>/dev/null || echo "⚠️  Helm release不存在或已删除"

# 2. 删除namespace
echo "🗂️  删除namespace..."
kubectl delete namespace sentry-system 2>/dev/null || echo "⚠️  Namespace不存在或已删除"

# 3. 等待namespace完全删除
echo "⏳ 等待namespace完全删除..."
while kubectl get namespace sentry-system 2>/dev/null; do
    echo "等待namespace删除..."
    sleep 2
done

# 4. 清理可能的ClusterRole资源
echo "🔐 清理RBAC资源..."
kubectl delete clusterrole sentry-deployment-role 2>/dev/null || echo "⚠️  ClusterRole不存在或已删除"
kubectl delete clusterrolebinding sentry-deployment-rolebinding 2>/dev/null || echo "⚠️  ClusterRoleBinding不存在或已删除"

# 5. 验证清理结果
echo "✅ 验证清理结果..."
HELM_CHECK=$(helm list --all-namespaces | grep sentry || echo "")
NS_CHECK=$(kubectl get namespaces | grep sentry-system || echo "")
CR_CHECK=$(kubectl get clusterrole | grep sentry-deployment || echo "")
CRB_CHECK=$(kubectl get clusterrolebinding | grep sentry-deployment || echo "")

if [[ -z "$HELM_CHECK" && -z "$NS_CHECK" && -z "$CR_CHECK" && -z "$CRB_CHECK" ]]; then
    echo "🎉 Sentry环境清理完成！"
else
    echo "⚠️  以下资源可能需要手动清理："
    [[ -n "$HELM_CHECK" ]] && echo "  - Helm releases: $HELM_CHECK"
    [[ -n "$NS_CHECK" ]] && echo "  - Namespaces: $NS_CHECK"
    [[ -n "$CR_CHECK" ]] && echo "  - ClusterRoles: $CR_CHECK"
    [[ -n "$CRB_CHECK" ]] && echo "  - ClusterRoleBindings: $CRB_CHECK"
fi

echo "🚀 现在可以重新部署Sentry了！"
```

### 使用清理脚本

```bash
# 创建清理脚本
cat > sentry-cleanup.sh << 'EOF'
[脚本内容如上]
EOF

# 添加执行权限
chmod +x sentry-cleanup.sh

# 执行清理
./sentry-cleanup.sh

# 清理完成后删除脚本（可选）
rm sentry-cleanup.sh
```

### 清理注意事项

1. **数据备份**：清理前确保备份重要的配置和数据
   ```bash
   # 备份配置
   kubectl get configmap sentry-deployment-config -n sentry-system -o yaml > sentry-config-backup.yaml
   
   # 备份secrets（注意：包含敏感信息）
   kubectl get secret sentry-tokens -n sentry-system -o yaml > sentry-secrets-backup.yaml
   ```

2. **确认影响范围**：确认删除的资源不会影响其他应用

3. **权限检查**：确保有足够的权限删除ClusterRole和ClusterRoleBinding

4. **网络策略**：如果配置了网络策略，可能需要单独清理

5. **持久化存储**：如果使用了PV/PVC，需要单独处理

### 清理后的重新部署

清理完成后，可以按照本文档前面的"快速部署"或"部署步骤详解"重新部署Sentry。

---

**注意：** 本文档基于实际E2E测试经验编写，涵盖了部署过程中遇到的常见问题和解决方案。如遇到文档中未涵盖的问题，请查看应用日志进行具体分析。