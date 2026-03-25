# 任务 06 - MCP Server（SSE + stdio）

## 状态: 🟢 待执行

## 描述

实现 MCP Server，让 Claude Code、Cursor、JB、VSCode、OpenClaw 可以通过 MCP 协议直接使用 ctl_device 工具。

## MCP 协议说明

MCP（Model Context Protocol）是 Anthropic 定义的标准，AI 工具通过它调用外部工具。
- **stdio 模式**：工具作为子进程运行，通过 stdin/stdout 通信（本地 IDE 用这种）
- **SSE 模式**：工具作为 HTTP server，通过 Server-Sent Events 推送（远程服务用这种）

## 详细要求

### internal/server/mcp_stdio.go

```go
// MCP stdio server：从 stdin 读 JSON-RPC，向 stdout 写响应
// 这是 MCP 规范的标准 stdio transport
func RunMCPStdio(scheduler *project.Scheduler, agentMgr *agent.Manager) error {
    // 1. 发送 initialize response（MCP 握手）
    // 2. 循环读 stdin，解析 MCP 请求
    // 3. 路由到对应 handler
    // 4. 写 stdout
}
```

MCP 握手：
```json
// 客户端发送
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}

// server 响应
{"jsonrpc":"2.0","id":1,"result":{
  "protocolVersion":"2024-11-05",
  "capabilities":{"tools":{}},
  "serverInfo":{"name":"ctl_device","version":"0.1.0"}
}}
```

### internal/server/mcp_sse.go

```go
// MCP SSE server：监听 :3710
// GET  /sse      → SSE stream（server → client 推送）
// POST /message  → client → server 消息
func RunMCPSSE(scheduler *project.Scheduler, agentMgr *agent.Manager) error
```

### pkg/protocol/mcp_tools.go（完整实现）

定义所有工具的 MCP schema，server 在 `tools/list` 响应时返回：

```json
{
  "tools": [
    {
      "name": "task_get",
      "description": "Get the current task for a project. Use this to receive your coding assignment.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "project": {"type": "string", "description": "Project name"}
        },
        "required": ["project"]
      }
    },
    {
      "name": "task_complete",
      "description": "Report task completion with commit hash and test results.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "project":     {"type": "string"},
          "summary":     {"type": "string", "description": "Brief summary of what was implemented"},
          "commit":      {"type": "string", "description": "Git commit hash"},
          "test_output": {"type": "string", "description": "Output of test run"},
          "issues":      {"type": "string", "description": "Any issues encountered (empty if none)"}
        },
        "required": ["project", "summary", "commit"]
      }
    },
    {
      "name": "task_block",
      "description": "Report that you are blocked and cannot proceed.",
      "inputSchema": { ... }
    },
    {
      "name": "task_status",
      "description": "Update task execution status.",
      "inputSchema": { ... }
    },
    {
      "name": "project_list",
      "description": "List all projects and their current status.",
      "inputSchema": {"type": "object", "properties": {}}
    },
    {
      "name": "project_register",
      "description": "Register a new project for management.",
      "inputSchema": { ... }
    },
    {
      "name": "task_dispatch",
      "description": "Dispatch a task to a project's executor.",
      "inputSchema": { ... }
    },
    {
      "name": "task_advance",
      "description": "Mark current task as verified and advance to next task.",
      "inputSchema": {"type": "object", "properties": {"project": {"type": "string"}}}
    },
    {
      "name": "agent_list",
      "description": "List all connected agents and their status.",
      "inputSchema": {"type": "object", "properties": {}}
    }
  ]
}
```

### Client 端 MCP stdio 模式

`ctl_device client mcp` 以 stdio 模式运行，连接远程 server：

```go
// 从 stdin 读 MCP 请求
// 转发给远程 JSON-RPC server（HTTP）
// 把响应写回 stdout
// 这样本地 IDE 认为在和本地 MCP server 通信，实际请求转发到远端
```

OpenClaw 配置：
```json
{
  "mcp": {
    "servers": {
      "ctl_device": {
        "command": "ctl_device",
        "args": ["client", "mcp", "--server", "http://localhost:3711", "--token", ""]
      }
    }
  }
}
```

Claude Code 配置（`.claude.json`）：
```json
{
  "mcpServers": {
    "ctl_device": {
      "command": "ctl_device",
      "args": ["client", "mcp", "--server", "https://your-vps:3711", "--token", "xxx"]
    }
  }
}
```

## 验收标准

1. `go test ./internal/server/...` 包含 MCP 握手测试
2. stdio 模式：
   - 发送 `initialize` → 收到正确的 `serverInfo`
   - 发送 `tools/list` → 收到所有工具定义
   - 发送 `tools/call` task_get → 收到任务数据
3. SSE 模式：
   - `GET /sse` 建立连接
   - `POST /message` 发送工具调用
   - SSE stream 收到响应
4. Client mcp 模式：能透明代理到远程 server

## 估时

3 小时
