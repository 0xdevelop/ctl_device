# 任务 05 - JSON-RPC Server

## 状态: 🟢 待执行

## 描述

实现 JSON-RPC HTTP Server，这是所有客户端（OpenClaw CLI、cURL、脚本）的通用接入点。

## 详细要求

### internal/server/jsonrpc.go

监听 `:3711`，路由 `POST /rpc`。

#### 认证中间件
```go
func authMiddleware(token string, next http.Handler) http.Handler {
    // 从 request body 读 auth.token 或 Authorization header
    // token 为空字符串时跳过认证（本地开发）
}
```

#### 方法路由表

| 方法 | 处理函数 |
|------|---------|
| `bridge.task.get` | 执行者：拉取当前任务 |
| `bridge.task.status` | 执行者：更新状态 |
| `bridge.task.complete` | 执行者：提交完成 |
| `bridge.task.block` | 执行者：报告阻塞 |
| `bridge.agent.register` | 注册/重连 |
| `bridge.agent.heartbeat` | 心跳 |
| `bridge.agent.list` | 列出在线 agents |
| `bridge.project.register` | 调度者：注册项目 |
| `bridge.project.list` | 调度者：列出项目 |
| `bridge.task.dispatch` | 调度者：下发任务 |
| `bridge.task.advance` | 调度者：推进下一任务 |
| `bridge.event.subscribe` | SSE 事件订阅（GET /events） |

#### SSE 事件流

`GET /events?project=scrypt-wallet&token=xxx`

```
data: {"type":"task_status_changed","project":"scrypt-wallet","payload":{"status":"executing"}}

data: {"type":"task_completed","project":"scrypt-wallet","payload":{"commit":"abc123"}}
```

OpenClaw 订阅这个流，实现实时感知，不需要轮询。

#### 错误码
```go
const (
    ErrCodeInvalidParams  = -32602
    ErrCodeUnauthorized   = -32001
    ErrCodeProjectNotFound = -32002
    ErrCodeTaskNotFound   = -32003
    ErrCodeAgentNotFound  = -32004
    ErrCodeNoExecutor     = -32005  // 没有可用的执行者
)
```

### CLI 客户端（internal/client/jsonrpc_client.go）

```go
type Client struct {
    ServerURL string
    Token     string
    AgentID   string
}

func (c *Client) TaskGet(project string) (*protocol.Task, error)
func (c *Client) TaskComplete(project string, report *CompleteReport) error
func (c *Client) AgentRegister(req *RegisterRequest) error
func (c *Client) Heartbeat() error
// ... 对应所有 RPC 方法的 Go 封装
```

CLI 命令绑定（`cmd/ctl_device/main.go`）：
```bash
ctl_device client status --project scrypt-wallet
# 调用 bridge.project.list + bridge.task.get，格式化输出

ctl_device client dispatch --project scrypt-wallet --task-file plan/tasks/03.md
# 读取 task 文件 → bridge.task.dispatch

ctl_device client logs --project scrypt-wallet --follow
# 连接 GET /events，实时打印事件
```

## 验收标准

1. `go test ./internal/server/...` 通过（httptest 集成测试）
2. 测试覆盖：
   - 所有 RPC 方法的 happy path
   - 认证失败返回 -32001
   - 参数错误返回 -32602
   - SSE 连接，发布事件，客户端收到
3. `ctl_device server` 启动后：
   - `ctl_device client status` 能输出结果
   - `curl -X POST http://localhost:3711/rpc -d '{"jsonrpc":"2.0","id":1,"method":"bridge.agent.list","params":{}}'` 返回 JSON
4. `go test -race ./...` 无报警

## 估时

3 小时
