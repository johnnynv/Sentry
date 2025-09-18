# Sentry 中文文档

欢迎使用Sentry - Tekton Pipeline自动部署器！本目录包含完整的中文文档。

## 📚 文档目录

### 核心文档

- **[部署指南](deployment.md)** - 详细的部署说明，以Helm Chart为主
- **[架构设计](architecture.md)** - 系统架构和设计原理
- **[实施方案](implementation.md)** - 项目实施和开发计划

### 快速链接

| 文档 | 描述 | 推荐用户 |
|------|------|----------|
| [部署指南](deployment.md) | 🚀 Helm Chart部署、YAML清单部署、Docker部署 | 运维工程师、DevOps |
| [架构设计](architecture.md) | 🏗️ 系统架构、组件关系、技术选型 | 开发人员、架构师 |
| [实施方案](implementation.md) | 📋 开发计划、阶段划分、时间安排 | 项目管理、开发团队 |

## 🚀 快速开始

如果您是第一次使用Sentry，推荐按以下顺序阅读：

1. **了解系统** - 阅读[架构设计](architecture.md)了解Sentry的工作原理
2. **部署系统** - 按照[部署指南](deployment.md)在您的环境中部署Sentry
3. **配置使用** - 根据您的需求配置监控的仓库和部署策略

## 📖 主要特性

Sentry提供以下核心功能：

- ✅ **多平台支持** - 支持GitHub、GitLab、Gitea等Git平台
- ✅ **智能监控** - 自动检测代码仓库变化并触发部署
- ✅ **组级部署** - 支持并行和串行的批量部署策略
- ✅ **灵活配置** - 支持多种部署命令和自定义脚本
- ✅ **安全可靠** - 完整的RBAC权限控制和错误恢复机制
- ✅ **云原生** - 为Kubernetes和Tekton Pipelines优化设计

## 🎯 部署方式对比

| 部署方式 | 适用场景 | 复杂度 | 推荐度 |
|----------|----------|--------|--------|
| **Helm Chart** | 生产环境、多环境管理 | 中等 | ⭐⭐⭐⭐⭐ |
| **原始YAML** | 简单环境、自定义需求 | 简单 | ⭐⭐⭐ |
| **Docker** | 本地测试、开发调试 | 简单 | ⭐⭐ |

## 🔧 系统要求

### 基础环境
- Kubernetes 1.20+
- Tekton Pipelines
- kubectl访问权限

### 可选组件
- Helm 3.0+（推荐）
- Docker（本地开发）
- Git客户端

## 📝 配置示例

### 最小配置
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

### 高级配置（组级部署）
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
    # ... 详细配置见部署指南
```

## 🆘 获取帮助

遇到问题时的解决路径：

1. **查看日志** - 使用`kubectl logs`查看详细错误信息
2. **检查配置** - 验证YAML配置文件格式和内容
3. **权限验证** - 确认Token权限和RBAC配置
4. **参考文档** - 查看相关章节的故障排查部分
5. **社区支持** - 在项目仓库提交Issue

## 🔄 文档更新

本文档随项目版本同步更新。当前文档对应版本：

- **Sentry版本**: v1.0.0
- **文档版本**: v1.0.0
- **最后更新**: 2025-09-18

---

**注意**: 英文版本的文档请参考项目根目录的README.md文件。
