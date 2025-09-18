# Sentry 系统架构设计

## 1. 核心需求

- **多仓库监控**：监控多个Git仓库的指定分支变化
- **组级部署**：支持并行和串行的批量部署策略
- **灵活命令**：支持自定义部署命令，而非仅限于YAML文件扫描
- **多平台支持**：支持GitHub、GitLab、Gitea等Git平台
- **安全可靠**：完整的RBAC权限控制和错误恢复机制
- **云原生设计**：为Kubernetes和Tekton Pipelines优化

## 2. 系统架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                          Sentry 系统                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   配置管理器     │  │    监控服务     │  │    部署服务     │  │
│  │  (ConfigMgr)   │  │ (MonitorSvc)   │  │  (DeploySvc)   │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│           │                     │                     │          │
├───────────┼─────────────────────┼─────────────────────┼──────────┤
│           ▼                     ▼                     ▼          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   YAML配置      │  │   Git API      │  │  命令执行器     │  │
│  │     解析        │  │    客户端       │  │    & kubectl    │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
              ┌─────────────────────────────────────┐
              │          外部系统集成                │
              ├─────────────────────────────────────┤
              │  GitHub API │ GitLab API │ Gitea API │
              │      │            │           │      │
              │      ▼            ▼           ▼      │
              │ ┌─────────┐ ┌─────────┐ ┌─────────┐ │
              │ │ Repo A1 │ │ Repo A2 │ │ Repo B  │ │
              │ │监控仓库 │ │监控仓库 │ │QA仓库   │ │
              │ └─────────┘ └─────────┘ └─────────┘ │
              └─────────────────────────────────────┘
                               │
                               ▼
              ┌─────────────────────────────────────┐
              │       Kubernetes集群                │
              ├─────────────────────────────────────┤
              │  Tekton Pipeline │ 其他Workloads    │
              └─────────────────────────────────────┘
```

## 3. 核心组件设计

### 3.1 配置管理器 (Config Manager)

**职责**：
- 加载和解析YAML配置文件
- 环境变量展开和验证
- 支持全局组配置和多仓库配置
- 配置热重载（未来功能）

**关键特性**：
- 方案C设计：全局组配置 + 简化repository配置
- 支持组级执行策略（并行/串行）
- Kubernetes命名规范验证
- 敏感信息环境变量管理

**配置结构**：
```yaml
# 全局组配置
groups:
  ai-projects:
    execution_strategy: "parallel"  # parallel | sequential
    max_parallel: 3
    continue_on_error: true
    global_timeout: 900

# 仓库配置
repositories:
  - name: "rag-project"
    group: "ai-projects"  # 可选分组
    monitor: { ... }      # 监控配置
    deploy: { ... }       # 部署配置
```

### 3.2 监控服务 (Monitor Service)

**职责**：
- 定时轮询多个Git仓库
- 检测commit变化并记录状态
- 支持多分支监控和正则匹配
- 触发组级别或独立的部署任务

**关键特性**：
- 并发监控多个仓库
- 智能变化检测（SHA比较）
- 分支模式匹配支持
- 重试机制和错误恢复
- 组级触发策略

**工作流程**：
```
定时器触发 → 并发检查所有仓库 → 检测变化 → 分组触发部署
     ↓              ↓               ↓           ↓
   60s轮询      Git API调用      SHA比较    组策略执行
```

### 3.3 部署服务 (Deploy Service)

**职责**：
- 执行组级别的批量部署
- 克隆QA仓库并执行自定义命令
- 支持并行和串行执行策略
- 部署结果聚合和错误处理

**关键特性**：
- 组级部署协调
- 并行/串行执行策略
- 自定义命令执行（非YAML扫描）
- 临时文件管理和清理
- 部署结果统计和回滚

**部署策略**：
```
组级触发 → 确定执行策略 → 并行/串行执行 → 结果聚合
    ↓           ↓              ↓           ↓
  触发事件   策略决策        命令执行     状态报告
```

### 3.4 日志系统 (Logging System)

**职责**：
- 结构化日志记录
- 支持不同日志级别
- 操作审计和追踪
- 性能监控数据收集

**特性**：
- 结构化日志格式：`[timestamp] LEVEL: message [key=value]`
- 操作链路追踪
- 错误详情记录
- 性能指标统计

## 4. 数据流架构

### 4.1 监控数据流

```
配置加载 → 服务初始化 → 监控循环 → 变化检测 → 组级触发
    ↓          ↓          ↓         ↓          ↓
 YAML解析   服务启动    定时轮询   SHA比较   策略执行
```

**详细步骤**：
1. **配置阶段**：加载YAML，验证仓库配置，初始化认证
2. **监控阶段**：并发检查所有仓库的最新commit
3. **检测阶段**：比较当前SHA与上次记录，识别变化
4. **分组阶段**：根据仓库分组配置决定触发策略
5. **执行阶段**：按组策略执行部署（并行/串行）

### 4.2 部署数据流

```
组级触发 → 策略选择 → 任务分发 → 并发执行 → 结果聚合
    ↓         ↓         ↓         ↓         ↓
  变化事件   执行策略   工作队列   命令执行   状态汇总
```

**执行模式**：

**并行模式**：
```go
for _, repo := range group {
    go func(repoName string) {
        // 克隆QA仓库
        // 执行自定义命令
        // 报告结果
    }(repo)
}
wait() // 等待所有goroutine完成
```

**串行模式**：
```go
for _, repo := range group {
    result := deployRepo(repo)
    if !result.Success && !continueOnError {
        break // 失败时停止
    }
}
```

## 5. 高级配置设计

### 5.1 完整配置示例

```yaml
# 全局设置
polling_interval: 60  # 最小60秒

# 组配置
groups:
  ai-blueprints:
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
  - name: "rag-service"
    group: "ai-blueprints"
    monitor:
      repo_url: "https://github.com/company/rag"
      branches: ["main", "dev.*"]  # 支持正则
      repo_type: "github"
      auth:
        username: "${GITHUB_USERNAME}"
        token: "${GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab.com/qa/pipelines"
      qa_repo_branch: "main"
      repo_type: "gitlab"
      auth:
        username: "${GITLAB_USERNAME}"
        token: "${GITLAB_TOKEN}"
      project_name: "rag"  # K8s命名规范
      commands:
        - "cd .tekton/rag"
        - "kubectl apply -f . --namespace=tekton-pipelines"
        - "./scripts/verify-deployment.sh"
    webhook_url: ""  # 预留webhook功能

  - name: "standalone-service"
    # 无group字段 = 独立执行
    monitor: { ... }
    deploy: { ... }

# 全局设置
global:
  tmp_dir: "/tmp/sentry"
  cleanup: true
  log_level: "info"
  timeout: 300
```

### 5.2 配置层级关系

```
Global Config (全局配置)
    ├── Groups (组配置)
    │   ├── execution_strategy
    │   ├── max_parallel
    │   ├── continue_on_error
    │   └── global_timeout
    └── Repositories (仓库配置)
        ├── Monitor Config (监控配置)
        │   ├── repo_url, branches, repo_type
        │   └── auth (username, token)
        └── Deploy Config (部署配置)
            ├── qa_repo_url, qa_repo_branch
            ├── project_name, commands
            └── auth (username, token)
```

## 6. 安全架构

### 6.1 认证和授权

**Git平台认证**：
- 支持Personal Access Token
- 环境变量安全管理
- Token权限最小化原则

**Kubernetes RBAC**：
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sentry-deployer
rules:
- apiGroups: ["tekton.dev"]
  resources: ["pipelines", "pipelineruns", "tasks", "taskruns"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
```

### 6.2 安全边界

```
┌─────────────────────────────────────────┐
│            Sentry Pod                   │
│  ┌─────────────────────────────────────┐│
│  │        应用进程                     ││
│  │  ┌─────────────┐ ┌─────────────┐   ││
│  │  │   监控服务  │ │   部署服务  │   ││
│  │  └─────────────┘ └─────────────┘   ││
│  └─────────────────────────────────────┘│
│               │                         │
├───────────────┼─────────────────────────┤
│          网络边界                       │
└───────────────┼─────────────────────────┘
                │
          ┌─────┴─────┐
          │           │
    ┌─────▼─────┐ ┌──▼──────────┐
    │  Git APIs │ │ K8s APIs    │
    │  (HTTPS)  │ │ (RBAC限制)  │
    └───────────┘ └─────────────┘
```

## 7. 性能和可扩展性

### 7.1 性能特征

- **内存使用**：约200-500MB（取决于仓库数量）
- **CPU使用**：轻量级，主要是网络I/O等待
- **网络**：定时Git API调用，部署时的Git clone
- **存储**：临时文件存储，自动清理

### 7.2 扩展性设计

**水平扩展**：
- 无状态设计，支持多实例部署
- 基于仓库分片的负载均衡
- Kubernetes HPA自动扩缩容

**垂直扩展**：
- 配置调优：轮询间隔、并发数、超时时间
- 资源配额：内存、CPU限制
- 存储优化：临时文件清理策略

## 8. 监控和运维

### 8.1 健康检查

```yaml
# Kubernetes健康检查
livenessProbe:
  exec:
    command: ["./sentry", "-action=validate"]
  initialDelaySeconds: 30
  periodSeconds: 60

readinessProbe:
  exec:
    command: ["./sentry", "-action=validate"]
  initialDelaySeconds: 10
  periodSeconds: 30
```

### 8.2 指标收集

**应用指标**：
- 监控仓库数量和状态
- 部署成功/失败率
- 平均部署时间
- API调用延迟和错误率

**系统指标**：
- Pod资源使用情况
- 网络连接状态
- 存储空间使用

## 9. 未来扩展

### 9.1 计划中功能

- **Webhook支持**：接收Git平台推送事件
- **Web UI**：可视化监控界面和配置管理
- **插件系统**：支持自定义部署策略
- **多集群部署**：跨集群的部署协调

### 9.2 技术演进

- **事件驱动架构**：从轮询模式升级到事件驱动
- **分布式部署**：支持更大规模的仓库监控
- **AI集成**：智能部署策略推荐和异常检测

---

该架构设计确保了Sentry系统的可靠性、可扩展性和安全性，为企业级的CI/CD自动化提供坚实的技术基础。