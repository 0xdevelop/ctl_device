# 任务 02 - 协议类型定义

## 状态: 🟢 待执行

## 描述

定义所有核心数据结构和 MCP tool schema，这是后续所有功能的基础。

## 详细要求

### pkg/protocol/task.go
```go
type TaskStatus string
const (
    TaskPending       TaskStatus = "pending"
    TaskExecuting     TaskStatus = "executing"
    TaskCompleted     TaskStatus = "completed"
    TaskBlocked       TaskStatus = "blocked"
    TaskExecutorLimit TaskStatus = "executor_limit"
    TaskTimeout       TaskStatus = "timeout"
    TaskArchived      TaskStatus = "archived"
)

type Task struct {
    ID                 string     `json:"id"`           // "project:num"
    Project            string     `json:"project"`
    Num                string     `json:"num"`          // "03"
    Name               string     `json:"name"`
    Description        string     `json:"description"`
    AcceptanceCriteria []string   `json:"acceptance_criteria"`
    ContextFiles       []string   `json:"context_files"`
    Status             TaskStatus `json:"status"`
    AssignedTo         string     `json:"assigned_to"`
    StartedAt          time.Time  `json:"started_at,omitempty"`
    UpdatedAt          time.Time  `json:"updated_at"`
    Commit             string     `json:"commit,omitempty"`
    Report             string     `json:"report,omitempty"`
    TimeoutMinutes     int        `json:"timeout_minutes"`
}
```

### pkg/protocol/agent.go
```go
type AgentRole string
const (
    RoleScheduler AgentRole = "scheduler"
    RoleExecutor  AgentRole = "executor"
    RoleBoth      AgentRole = "both"
)

type Agent struct {
    ID            string    `json:"id"`
    Role          AgentRole `json:"role"`
    Capabilities  []string  `json:"capabilities"`
    LastHeartbeat time.Time `json:"last_heartbeat"`
    Online        bool      `json:"online"`
    CurrentTask   string    `json:"current_task,omitempty"`
    ResumeOnline  bool      `json:"resume_online"` // 上线时自动恢复任务
}
```

### pkg/protocol/project.go
```go
type Project struct {
    Name           string `json:"name"`
    Dir            string `json:"dir"`
    Tech           string `json:"tech"`
    TestCmd        string `json:"test_cmd"`
    Executor       string `json:"executor"`       // agent ID 或 "any"
    TimeoutMinutes int    `json:"timeout_minutes"`
    NotifyChannel  string `json:"notify_channel"`
    NotifyTarget   string `json:"notify_target,omitempty"`
}
```

### pkg/protocol/mcp_tools.go

定义所有 MCP tool 的 JSON Schema：

执行者工具：
- `task_get` - input: {project: string}
- `task_status` - input: {project: string, status: string}
- `task_complete` - input: {project, summary, commit, test_output, issues}
- `task_block` - input: {project, reason, details}

调度者工具：
- `project_register` - input: {name, dir, tech, test_cmd, executor, timeout_minutes}
- `project_list` - input: {} output: []ProjectStatus
- `task_dispatch` - input: {project, task: Task}
- `task_advance` - input: {project}
- `agent_list` - input: {} output: []Agent

### pkg/protocol/jsonrpc.go

JSON-RPC 请求/响应结构：
```go
type Request struct {
    JSONRPC string      `json:"jsonrpc"` // "2.0"
    ID      interface{} `json:"id"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params"`
    Auth    *AuthToken  `json:"auth,omitempty"`
}

type Response struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      interface{} `json:"id"`
    Result  interface{} `json:"result,omitempty"`
    Error   *RPCError   `json:"error,omitempty"`
}

type AuthToken struct {
    Token string `json:"token"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

## 验收标准

1. `go build ./...` 无报错
2. `go vet ./...` 无报错
3. 所有类型有完整的 JSON tag
4. 所有 MCP tool schema 有 description 字段（MCP 规范要求）
5. 单元测试：JSON 序列化/反序列化正确

## 估时

1 小时
