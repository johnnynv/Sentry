# Sentry 简化架构设计

## 1. 核心需求

- 监控 repo A 的指定分支变化
- 变化时克隆 repo B，找到 .tekton 目录的 yaml 文件
- 执行 kubectl apply 部署
- 支持手动触发
- 支持 GitHub 和 GitLab
- Go 语言实现，YAML 配置

## 2. 简化架构

```
┌─────────────────────────────────────────┐
│              Sentry CLI                 │
├─────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────────┐   │
│  │   Monitor   │  │    Deployer     │   │
│  │   Service   │  │    Service      │   │
│  └─────────────┘  └─────────────────┘   │
├─────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────────┐   │
│  │ Git Client  │  │ kubectl Exec    │   │
│  └─────────────┘  └─────────────────┘   │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│     External Dependencies              │
├─────────────────────────────────────────┤
│  Repo A (GitHub/GitLab)                │
│  Repo B (GitHub/GitLab)                │
│  Kubernetes + Tekton                    │
└─────────────────────────────────────────┘
```

## 3. 核心组件

### 3.1 Monitor Service (监控服务)
- **职责**：定时检查 repo A 的分支变化
- **实现**：简单的定时器 + Git API 调用

### 3.2 Deployer Service (部署服务) 
- **职责**：克隆 repo B，扫描 .tekton 目录，执行 kubectl
- **实现**：git clone + 文件扫描 + shell 调用

### 3.3 Git Client (Git客户端)
- **职责**：与 GitHub/GitLab API 交互
- **实现**：HTTP 客户端调用 REST API

### 3.4 kubectl Executor (kubectl执行器)
- **职责**：执行 kubectl apply 命令
- **实现**：简单的 shell 命令执行

## 4. 数据流

```
配置加载 → 启动监控 → 检测变化 → 触发部署 → 完成
    ↓          ↓         ↓         ↓        ↓
  YAML      定时器    Git API   Git Clone  kubectl
```

1. **监控循环**：每N秒检查一次 repo A 的最新 commit
2. **变化检测**：对比上次记录的 commit SHA
3. **触发部署**：变化时克隆 repo B，扫描 .tekton，执行 kubectl

## 5. 配置设计

```yaml
# sentry.yaml
target_repo:
  url: "https://github.com/example/repo-a"
  branches: "main,develop"
  type: "github"

qa_repo:
  url: "https://gitlab-master.nvidia.com/example/repo-b"  
  branch: "main"
  type: "gitlab"

auth:
  github_token: "${GITHUB_TOKEN}"
  gitlab_token: "${GITLAB_TOKEN}"

polling_interval: 300  # 秒，最小60秒
tekton_dir: ".tekton"  # Tekton配置目录
kubectl_context: "default"  # kubectl上下文
namespace: "tekton-pipelines"  # 部署命名空间

# 可选配置
log_level: "info"  # debug, info, warn, error
max_retries: 3     # 失败重试次数
timeout: 300       # 超时时间（秒）
```

## 6. 目录结构

```
sentry/
├── main.go                    # 主程序 (~200行)
├── config.go                  # 配置管理 (~100行)
├── monitor.go                 # 监控服务 (~150行)
├── deploy.go                  # 部署服务 (~100行)
├── sentry.yaml                # 配置文件示例
├── .env.example               # 环境变量示例
├── go.mod
├── go.sum
├── Dockerfile
├── Makefile
├── README.md
└── docs/
    └── zh/
        ├── architecture.md    # 架构文档
        └── implementation.md  # 实现文档
```

**说明**：移除了单独的 git.go 和 kubectl.go，将其功能整合到 monitor.go 和 deploy.go 中，进一步简化结构。总代码量控制在 ~550行以内。

## 7. 核心依赖

```go
// go.mod
require (
    gopkg.in/yaml.v3           // YAML配置
    github.com/joho/godotenv   // .env文件支持
)
```

**优化说明**：移除了 `robfig/cron` 依赖，使用标准库的 `time.Ticker` 实现定时任务，减少外部依赖。

## 8. 部署方式

### 8.1 本地运行
```bash
# 1. 创建 .env 文件（推荐方式）
cat > .env << EOF
GITHUB_TOKEN=ghp_your_token_here
GITLAB_TOKEN=glpat_your_token_here
EOF

# 2. 运行
./sentry -action=watch     # 监控模式
./sentry -action=trigger   # 手动触发

# 或者直接设置环境变量
export GITHUB_TOKEN=xxx GITLAB_TOKEN=xxx
./sentry -action=watch
```

### 8.2 Docker 运行
```dockerfile
FROM golang:alpine AS builder
COPY . .
RUN go build -o sentry .

FROM alpine
RUN apk add --no-cache git kubectl
COPY --from=builder /sentry /usr/bin/
CMD ["sentry", "watch"]
```

### 8.3 Kubernetes 部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sentry
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sentry
  template:
    spec:
      containers:
      - name: sentry
        image: sentry:latest
        env:
        - name: GITHUB_TOKEN
          valueFrom:
            secretKeyRef:
              name: git-tokens
              key: github
        - name: GITLAB_TOKEN  
          valueFrom:
            secretKeyRef:
              name: git-tokens
              key: gitlab
        volumeMounts:
        - name: config
          mountPath: /config
      volumes:
      - name: config
        configMap:
          name: sentry-config
```

## 9. 实现策略

### 9.1 最小可行产品 (MVP)
1. **核心功能**：监控 + 部署
2. **单文件实现**：所有逻辑在 main.go
3. **简单配置**：YAML + 环境变量
4. **基本错误处理**：记录日志，继续运行

### 9.2 渐进优化
1. **第一版**：功能可用
2. **第二版**：代码结构优化
3. **第三版**：错误处理完善
4. **第四版**：性能优化

## 10. 错误处理策略

### 10.1 错误分类
- **配置错误**：启动时检查，快速失败
- **网络错误**：API调用失败，指数退避重试
- **部署错误**：kubectl失败，记录日志继续监控
- **Git错误**：仓库克隆失败，跳过本次部署

### 10.2 监控和日志
```go
// 日志级别
DEBUG: Git API详细调用信息
INFO:  启动、变更检测、部署成功
WARN:  重试、部分失败
ERROR: 致命错误、配置错误
```

### 10.3 健康检查
- 监控服务状态检查
- 配置文件验证命令
- 手动触发测试命令

## 11. 开发计划

**Day 1-2: 基础功能**
- [x] 项目结构设计
- [ ] 配置加载和验证
- [ ] Git API调用（GitHub/GitLab）

**Day 3-4: 核心逻辑**
- [ ] 监控循环实现
- [ ] 变更检测逻辑
- [ ] 部署服务实现

**Day 5-6: 集成测试**
- [ ] 端到端测试
- [ ] 错误处理完善
- [ ] 日志和监控

**Day 7: 部署和文档**
- [ ] Docker镜像构建
- [ ] K8s部署配置
- [ ] 用户文档

**总工作量**：1周，专注于核心功能实现。
