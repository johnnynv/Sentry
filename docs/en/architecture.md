# Sentry System Architecture Design

## 1. Core Requirements

- **Multi-Repository Monitoring**: Monitor changes across multiple Git repositories on specified branches
- **Group-Level Deployment**: Support parallel and sequential batch deployment strategies
- **Flexible Commands**: Support custom deployment commands, not limited to YAML file scanning
- **Multi-Platform Support**: Support GitHub, GitLab, Gitea and other Git platforms
- **Security and Reliability**: Complete RBAC permission control and error recovery mechanisms
- **Cloud-Native Design**: Optimized for Kubernetes and Tekton Pipelines

## 2. System Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          Sentry System                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │  Config Manager │  │ Monitor Service │  │ Deploy Service  │  │
│  │   (ConfigMgr)   │  │ (MonitorSvc)    │  │  (DeploySvc)    │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│           │                     │                     │          │
├───────────┼─────────────────────┼─────────────────────┼──────────┤
│           ▼                     ▼                     ▼          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   YAML Config   │  │   Git API       │  │ Command Executor│  │
│  │     Parser      │  │    Client       │  │   & kubectl     │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
              ┌─────────────────────────────────────┐
              │        External System Integration   │
              ├─────────────────────────────────────┤
              │  GitHub API │ GitLab API │ Gitea API │
              │      │            │           │      │
              │      ▼            ▼           ▼      │
              │ ┌─────────┐ ┌─────────┐ ┌─────────┐ │
              │ │ Repo A1 │ │ Repo A2 │ │ Repo B  │ │
              │ │Monitor  │ │Monitor  │ │QA Repo  │ │
              │ │Repos    │ │Repos    │ │         │ │
              │ └─────────┘ └─────────┘ └─────────┘ │
              └─────────────────────────────────────┘
                               │
                               ▼
              ┌─────────────────────────────────────┐
              │       Kubernetes Cluster            │
              ├─────────────────────────────────────┤
              │  Tekton Pipeline │ Other Workloads  │
              └─────────────────────────────────────┘
```

## 3. Core Component Design

### 3.1 Config Manager

**Responsibilities**:
- Load and parse YAML configuration files
- Environment variable expansion and validation
- Support global group configuration and multi-repository configuration
- Configuration hot reload (future feature)

**Key Features**:
- Scheme C design: Global group configuration + simplified repository configuration
- Support group-level execution strategies (parallel/sequential)
- Kubernetes naming convention validation
- Sensitive information environment variable management

**Configuration Structure**:
```yaml
# Global group configuration
groups:
  ai-projects:
    execution_strategy: "parallel"  # parallel | sequential
    max_parallel: 3
    continue_on_error: true
    global_timeout: 900

# Repository configuration
repositories:
  - name: "rag-project"
    group: "ai-projects"  # Optional grouping
    monitor: { ... }      # Monitor configuration
    deploy: { ... }       # Deploy configuration
```

### 3.2 Monitor Service

**Responsibilities**:
- Periodically poll multiple Git repositories
- Detect commit changes and record status
- Support multi-branch monitoring and regex matching
- Trigger group-level or independent deployment tasks

**Key Features**:
- Concurrent monitoring of multiple repositories
- Intelligent change detection (SHA comparison)
- Branch pattern matching support
- Retry mechanism and error recovery
- Group-level trigger strategy

**Workflow**:
```
Timer Trigger → Concurrent Check All Repos → Detect Changes → Group Trigger Deploy
     ↓                    ↓                      ↓               ↓
   60s Poll           Git API Calls          SHA Compare    Group Strategy
```

### 3.3 Deploy Service

**Responsibilities**:
- Execute group-level batch deployments
- Clone QA repositories and execute custom commands
- Support parallel and sequential execution strategies
- Deployment result aggregation and error handling

**Key Features**:
- Group-level deployment coordination
- Parallel/sequential execution strategies
- Custom command execution (not YAML scanning)
- Temporary file management and cleanup
- Deployment result statistics and rollback

**Deployment Strategy**:
```
Group Trigger → Determine Strategy → Parallel/Sequential Execute → Result Aggregate
    ↓               ↓                        ↓                      ↓
  Trigger Event  Strategy Decision      Command Execute        Status Report
```

### 3.4 Logging System

**Responsibilities**:
- Structured logging
- Support different log levels
- Operation audit and tracing
- Performance monitoring data collection

**Features**:
- Structured log format: `[timestamp] LEVEL: message [key=value]`
- Operation chain tracing
- Error detail recording
- Performance metrics statistics

## 4. Data Flow Architecture

### 4.1 Monitor Data Flow

```
Config Load → Service Init → Monitor Loop → Change Detection → Group Trigger
    ↓            ↓             ↓              ↓                ↓
 YAML Parse  Service Start  Timer Poll    SHA Compare    Strategy Execute
```

**Detailed Steps**:
1. **Configuration Phase**: Load YAML, validate repository configuration, initialize authentication
2. **Monitoring Phase**: Concurrently check latest commits of all repositories
3. **Detection Phase**: Compare current SHA with last recorded, identify changes
4. **Grouping Phase**: Determine trigger strategy based on repository group configuration
5. **Execution Phase**: Execute deployment according to group strategy (parallel/sequential)

### 4.2 Deploy Data Flow

```
Group Trigger → Strategy Selection → Task Distribution → Concurrent Execute → Result Aggregate
    ↓              ↓                   ↓                   ↓                  ↓
  Change Event  Execute Strategy    Work Queue        Command Execute     Status Summary
```

**Execution Modes**:

**Parallel Mode**:
```go
for _, repo := range group {
    go func(repoName string) {
        // Clone QA repository
        // Execute custom commands
        // Report results
    }(repo)
}
wait() // Wait for all goroutines to complete
```

**Sequential Mode**:
```go
for _, repo := range group {
    result := deployRepo(repo)
    if !result.Success && !continueOnError {
        break // Stop on failure
    }
}
```

## 5. Advanced Configuration Design

### 5.1 Complete Configuration Example

```yaml
# Global settings
polling_interval: 60  # Minimum 60 seconds

# Group configuration
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

# Repository configuration
repositories:
  - name: "rag-service"
    group: "ai-blueprints"
    monitor:
      repo_url: "https://github.com/company/rag"
      branches: ["main", "dev.*"]  # Regex support
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
      project_name: "rag"  # K8s naming convention
      commands:
        - "cd .tekton/rag"
        - "kubectl apply -f . --namespace=tekton-pipelines"
        - "./scripts/verify-deployment.sh"
    webhook_url: ""  # Reserved for webhook functionality

  - name: "standalone-service"
    # No group field = independent execution
    monitor: { ... }
    deploy: { ... }

# Global settings
global:
  tmp_dir: "/tmp/sentry"
  cleanup: true
  log_level: "info"
  timeout: 300
```

### 5.2 Configuration Hierarchy

```
Global Config
    ├── Groups
    │   ├── execution_strategy
    │   ├── max_parallel
    │   ├── continue_on_error
    │   └── global_timeout
    └── Repositories
        ├── Monitor Config
        │   ├── repo_url, branches, repo_type
        │   └── auth (username, token)
        └── Deploy Config
            ├── qa_repo_url, qa_repo_branch
            ├── project_name, commands
            └── auth (username, token)
```

## 6. Security Architecture

### 6.1 Authentication and Authorization

**Git Platform Authentication**:
- Support Personal Access Token
- Secure environment variable management
- Token permission minimization principle

**Kubernetes RBAC**:
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

### 6.2 Security Boundaries

```
┌─────────────────────────────────────────┐
│            Sentry Pod                   │
│  ┌─────────────────────────────────────┐│
│  │        Application Process          ││
│  │  ┌─────────────┐ ┌─────────────┐   ││
│  │  │Monitor Svc  │ │Deploy Svc   │   ││
│  │  └─────────────┘ └─────────────┘   ││
│  └─────────────────────────────────────┘│
│               │                         │
├───────────────┼─────────────────────────┤
│         Network Boundary                │
└───────────────┼─────────────────────────┘
                │
          ┌─────┴─────┐
          │           │
    ┌─────▼─────┐ ┌──▼──────────┐
    │  Git APIs │ │ K8s APIs    │
    │  (HTTPS)  │ │ (RBAC限制)  │
    └───────────┘ └─────────────┘
```

## 7. Performance and Scalability

### 7.1 Performance Characteristics

- **Memory Usage**: ~200-500MB (depends on repository count)
- **CPU Usage**: Lightweight, mainly network I/O waiting
- **Network**: Periodic Git API calls, Git clone during deployment
- **Storage**: Temporary file storage, auto cleanup

### 7.2 Scalability Design

**Horizontal Scaling**:
- Stateless design, supports multi-instance deployment
- Repository sharding-based load balancing
- Kubernetes HPA auto-scaling

**Vertical Scaling**:
- Configuration tuning: polling interval, concurrency, timeout
- Resource quotas: memory, CPU limits
- Storage optimization: temporary file cleanup strategy

## 8. Monitoring and Operations

### 8.1 Health Checks

```yaml
# Kubernetes health checks
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

### 8.2 Metrics Collection

**Application Metrics**:
- Number and status of monitored repositories
- Deployment success/failure rate
- Average deployment time
- API call latency and error rate

**System Metrics**:
- Pod resource usage
- Network connection status
- Storage space usage

## 9. Future Extensions

### 9.1 Planned Features

- **Webhook Support**: Receive Git platform push events
- **Web UI**: Visual monitoring interface and configuration management
- **Plugin System**: Support custom deployment strategies
- **Multi-Cluster Deployment**: Cross-cluster deployment coordination

### 9.2 Technical Evolution

- **Event-Driven Architecture**: Upgrade from polling mode to event-driven
- **Distributed Deployment**: Support larger scale repository monitoring
- **AI Integration**: Intelligent deployment strategy recommendation and anomaly detection

---

This architecture design ensures the reliability, scalability, and security of the Sentry system, providing a solid technical foundation for enterprise-level CI/CD automation.
