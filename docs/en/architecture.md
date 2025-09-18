# Sentry System Architecture Design

## 1. Core Requirements

- **Multi-Repository Monitoring**: Monitor changes in specified branches of multiple Git repositories
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
│  │  Config Manager │  │ Monitor Service │  │  Deploy Service │  │
│  │  (ConfigMgr)   │  │ (MonitorSvc)   │  │  (DeploySvc)   │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│           │                     │                     │          │
├───────────┼─────────────────────┼─────────────────────┼──────────┤
│           ▼                     ▼                     ▼          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   YAML Config   │  │   Git API      │  │ Command Executor│  │
│  │     Parser      │  │    Client      │  │    & kubectl    │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
              ┌─────────────────────────────────────┐
              │         External System Integration  │
              ├─────────────────────────────────────┤
              │  GitHub API │ GitLab API │ Gitea API │
              │      │            │           │      │
              │      ▼            ▼           ▼      │
              │ ┌─────────┐ ┌─────────┐ ┌─────────┐ │
              │ │ Repo A1 │ │ Repo A2 │ │ Repo B  │ │
              │ │Monitor  │ │Monitor  │ │QA Repo  │ │
              │ │Repository│ │Repository│ │        │ │
              │ └─────────┘ └─────────┘ └─────────┘ │
              └─────────────────────────────────────┘
                               │
                               ▼
               ┌─────────────────────────────────────┐
               │       Kubernetes & Tekton           │
               ├─────────────────────────────────────┤
               │  ┌─────────────┐ ┌─────────────────┐ │
               │  │ Tekton      │ │ Kubernetes      │ │
               │  │ Pipelines   │ │ Resources       │ │
               │  │ ┌─────────┐ │ │ ┌─────────────┐ │ │
               │  │ │Pipeline │ │ │ │ Pods        │ │ │
               │  │ │Runs     │ │ │ │ Services    │ │ │
               │  │ └─────────┘ │ │ │ Deployments │ │ │
               │  └─────────────┘ │ │ └─────────────┘ │ │
               │                  │ │                 │ │
               └─────────────────────────────────────┘
```

## 3. Configuration Architecture

### 3.1 Scheme C: Global Group Configuration + Simplified Repository Configuration

The current architecture adopts **Scheme C**, which provides:

#### Global Group Settings
```yaml
groups:
  ai-blueprints:
    execution_strategy: "parallel"  # parallel | sequential  
    max_parallel: 3                 # Maximum concurrent deployments
    continue_on_error: true          # Continue if one repository fails
    global_timeout: 900              # Global timeout in seconds
```

#### Simplified Repository Configuration
```yaml
repositories:
  - name: "rag-project"
    group: "ai-blueprints"           # Associate with group
    monitor:
      repo_url: "https://github.com/johnnynv/rag"
      branches: ["main", "dev"]      # Support multiple branches
      repo_type: "github"
      auth:
        username: "${GITHUB_USERNAME}"
        token: "${GITHUB_TOKEN}"
    deploy:
      qa_repo_url: "https://gitlab-master.nvidia.com/..."
      qa_repo_branch: "rag-tekton-workflow"
      repo_type: "gitlab"
      project_name: "rag"            # Maps to .tekton/rag/
      commands:                      # Direct command execution
        - "cd .tekton/rag && export TMPDIR=/tmp/sentry && bash scripts/container-deployment-pipeline-onclick.sh"
      auth:
        username: "${GITLAB_USERNAME}"
        token: "${GITLAB_TOKEN}"
```

### 3.2 Configuration Benefits

1. **Group Management**: Repositories can be organized into logical groups
2. **Execution Strategy**: Support both parallel and sequential deployment within groups
3. **Error Handling**: Configurable failure handling at group level
4. **Direct Commands**: Avoid complex YAML scanning, execute predefined commands
5. **Multi-Platform**: Support different Git platforms for monitoring and deployment

## 4. Workflow Architecture

### 4.1 Monitoring Workflow

```
┌─────────────────┐
│ Start Monitoring│
└─────────┬───────┘
          │
          ▼
┌─────────────────┐     ┌─────────────────┐
│ Load Config     │────▶│ Initialize APIs │
│ & Credentials   │     │ & Services      │
└─────────┬───────┘     └─────────┬───────┘
          │                       │
          ▼                       │
┌─────────────────┐               │
│ Record Initial  │◀──────────────┘
│ Commit SHAs     │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Start Polling   │
│ Loop (60s)      │
└─────────┬───────┘
          │
    ┌─────▼─────┐
    │For Each   │
    │Repository │
    └─────┬─────┘
          │
          ▼
    ┌─────────────────┐
    │Check Branches   │ 
    │for Changes      │
    └─────┬─────┬─────┘
          │     │
      No  │     │ Yes
    Change│     │
          │     ▼
          │ ┌─────────────────┐
          │ │ Record New SHA  │
          │ │ & Trigger Group │
          │ │ Deployment      │
          │ └─────────────────┘
          │
          ▼
    ┌─────────────────┐
    │ Continue        │
    │ Monitoring      │
    └─────────────────┘
```

### 4.2 Deployment Workflow

```
┌─────────────────┐
│ Deployment      │
│ Trigger         │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Identify Group  │
│ & Strategy      │
└─────────┬───────┘
          │
    ┌─────▼─────┐
    │Strategy?  │
    └─────┬─────┘
          │
    ┌─────▼─────────────────────────────▼─────┐
    │Parallel           Sequential             │
    │                                         │
    │┌─────────────┐   ┌─────────────────────┐│
    ││Start All    │   │Start First Repository││
    ││Repositories │   │                     ││
    ││Concurrently │   │        │            ││
    │└─────────────┘   │        ▼            ││
    │       │           │┌─────────────────────┐│
    │       ▼           ││Wait for Completion  ││
    │┌─────────────┐   │└─────────────────────┘│
    ││Wait for All │   │        │            ││
    ││to Complete  │   │        ▼            ││
    │└─────────────┘   │┌─────────────────────┐│
    │                  ││Start Next Repository││
    │                  │└─────────────────────┘│
    └─────┬───────────────────────┬─────────────┘
          │                       │
          ▼                       │
    ┌─────────────────┐           │
    │ Aggregate       │◀──────────┘
    │ Results         │
    └─────────┬───────┘
              │
              ▼
    ┌─────────────────┐
    │ Return Status   │
    │ (Success/Fail)  │
    └─────────────────┘
```

### 4.3 Individual Repository Deployment

```
┌─────────────────┐
│ Start Repository│
│ Deployment      │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Create Temp     │
│ Directory       │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Clone QA        │
│ Repository      │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Execute         │
│ Commands        │
│ Sequentially    │
└─────────┬───────┘
          │
    ┌─────▼─────┐
    │Command    │
    │Success?   │
    └─────┬─────┘
          │
      Yes │  No
          │  │
          │  ▼
          │ ┌─────────────────┐
          │ │ Log Error &     │
          │ │ Stop Execution  │
          │ └─────────────────┘
          │
          ▼
┌─────────────────┐
│ Cleanup Temp    │
│ Directory       │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Return Result   │
│ (Success/Error) │
└─────────────────┘
```

## 5. Component Architecture

### 5.1 Configuration Manager (config.go)

**Responsibilities**:
- YAML configuration file parsing
- Environment variable substitution
- Configuration validation
- Group and repository settings management

**Key Features**:
- Support for `${VAR}` environment variable expansion
- Hierarchical configuration validation
- Multi-platform repository configuration

### 5.2 Monitor Service (monitor.go)

**Responsibilities**:
- Git repository polling
- Commit change detection
- Repository connectivity testing
- Deployment trigger coordination

**Key Features**:
- Multi-platform API support (GitHub, GitLab, Gitea)
- Commit SHA tracking and comparison
- Group-based deployment triggering
- Error handling and retry logic

### 5.3 Deploy Service (deploy.go)

**Responsibilities**:
- QA repository cloning
- Command execution in proper context
- Temporary directory management
- Deployment result aggregation

**Key Features**:
- Parallel and sequential execution strategies
- Command chaining with proper environment setup
- Temporary file cleanup
- Error isolation and reporting

### 5.4 Logger Service (logger.go)

**Responsibilities**:
- Structured logging with key-value pairs
- Multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
- Repository operation logging
- Deployment tracking

**Key Features**:
- Contextual logging with repository/group information
- Consistent log format for parsing
- Performance and operation tracking

## 6. Security Architecture

### 6.1 Authentication & Authorization

```
┌─────────────────────────────────────────┐
│              Security Layers             │
├─────────────────────────────────────────┤
│ ┌─────────────────┐ ┌─────────────────┐ │
│ │   Git Platform  │ │   Kubernetes    │ │
│ │   Credentials   │ │     RBAC        │ │
│ │                 │ │                 │ │
│ │ • GitHub Token  │ │ • ServiceAccount│ │
│ │ • GitLab Token  │ │ • ClusterRole   │ │
│ │ • Gitea Token   │ │ • RoleBinding   │ │
│ └─────────────────┘ └─────────────────┘ │
│           │                   │         │
│           ▼                   ▼         │
│ ┌─────────────────────────────────────┐ │
│ │        Environment Variables        │ │
│ │      & Kubernetes Secrets          │ │
│ │                                     │ │
│ │ • Token storage                     │ │
│ │ • Environment isolation            │ │
│ │ • Secret rotation support          │ │
│ └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### 6.2 Command Execution Security

**Container Sandboxing**:
- Runs in isolated container environment
- Non-root user execution
- Limited file system access
- Resource constraints

**RBAC Controls**:
- ServiceAccount with minimal required permissions
- ClusterRole for specific Tekton resources
- Namespace-scoped operations where possible

**Environment Isolation**:
- Temporary directories with proper cleanup
- Environment variable isolation
- Command execution in controlled context

## 7. Integration Architecture

### 7.1 Git Platform Integration

```
Sentry ←→ Git Platform APIs
   │
   ├─ GitHub API
   │  ├─ GET /repos/{owner}/{repo}/commits/{sha}
   │  ├─ Authentication: Bearer Token
   │  └─ Rate Limiting: 5000 requests/hour
   │
   ├─ GitLab API  
   │  ├─ GET /projects/{id}/repository/commits/{sha}
   │  ├─ Authentication: Private Token
   │  └─ Rate Limiting: 2000 requests/hour
   │
   └─ Gitea API
      ├─ GET /repos/{owner}/{repo}/commits/{sha}
      ├─ Authentication: Token
      └─ Rate Limiting: Configurable
```

### 7.2 Kubernetes & Tekton Integration

```
Sentry Container ←→ Kubernetes API
   │
   ├─ kubectl commands
   │  ├─ Pipeline/Task CRUD operations
   │  ├─ PipelineRun creation
   │  └─ Resource status monitoring
   │
   ├─ ServiceAccount Authentication
   │  ├─ Token-based authentication
   │  ├─ RBAC permission validation
   │  └─ Namespace-scoped operations
   │
   └─ Tekton Resources
      ├─ Pipeline definitions
      ├─ Task definitions
      ├─ PipelineRun executions
      └─ Result tracking
```

## 8. Data Flow Architecture

### 8.1 Configuration Data Flow

```
sentry.yaml ──→ ConfigManager ──→ Environment Variables ──→ Runtime Config
     │               │                        │                    │
     │               ▼                        ▼                    ▼
     │         Validation              Secret Resolution    Service Initialization
     │               │                        │                    │
     └───────────────┴────────────────────────┴────────────────────┘
                                    │
                                    ▼
                            Monitoring & Deployment
```

### 8.2 Monitoring Data Flow

```
Git Repository ──→ API Request ──→ Commit SHA ──→ Comparison ──→ Change Detection
      │                │              │             │              │
      ▼                ▼              ▼             ▼              ▼
   Branches         Response       New SHA      Old SHA         Trigger
      │                │              │             │              │
      └────────────────┴──────────────┴─────────────┴──────────────┘
                                      │
                                      ▼
                              Group Deployment
```

### 8.3 Deployment Data Flow

```
Trigger ──→ Group Strategy ──→ Repository Clone ──→ Command Execution ──→ Result
   │             │                    │                   │                │
   ▼             ▼                    ▼                   ▼                ▼
Repository   Parallel/          QA Repository        Script/kubectl     Success/
Selection    Sequential           Download             Commands          Error
   │             │                    │                   │                │
   └─────────────┴────────────────────┴───────────────────┴────────────────┘
                                      │
                                      ▼
                               Tekton PipelineRun
```

## 9. Scalability and Performance

### 9.1 Horizontal Scaling

- **Multiple Sentry Instances**: Deploy multiple Sentry instances for different repository groups
- **Load Distribution**: Distribute repositories across instances
- **Resource Isolation**: Separate resource limits per instance

### 9.2 Performance Optimization

- **API Rate Limiting**: Respect Git platform rate limits
- **Efficient Polling**: Optimized polling intervals based on repository activity
- **Parallel Execution**: Concurrent deployment execution where safe
- **Resource Management**: Proper cleanup and resource usage

### 9.3 Monitoring and Observability

- **Structured Logging**: Consistent, parseable log format
- **Metrics Collection**: Repository check success/failure rates
- **Health Checks**: Built-in validation and connectivity testing
- **Performance Tracking**: Deployment duration and success metrics

---

This architecture provides a robust, scalable, and secure foundation for automated Git repository monitoring and Tekton pipeline deployment in Kubernetes environments.