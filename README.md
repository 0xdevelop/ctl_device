# ctl_device

分布式 AI 任务调度中间层，连接 OpenClaw 与 Trae CN / Claude Code / Cursor。

## 快速开始

### Server

#### 下载最新版

```bash
# Linux (amd64)
curl -L https://github.com/0xdevelop/ctl_device/releases/latest/download/ctl_device_linux_amd64 -o ctl_device
chmod +x ctl_device

# Linux (arm64)
curl -L https://github.com/0xdevelop/ctl_device/releases/latest/download/ctl_device_linux_arm64 -o ctl_device
chmod +x ctl_device

# macOS (amd64)
curl -L https://github.com/0xdevelop/ctl_device/releases/latest/download/ctl_device_darwin_amd64 -o ctl_device
chmod +x ctl_device

# macOS (arm64)
curl -L https://github.com/0xdevelop/ctl_device/releases/latest/download/ctl_device_darwin_arm64 -o ctl_device
chmod +x ctl_device

# Windows (amd64)
curl -L https://github.com/0xdevelop/ctl_device/releases/latest/download/ctl_device_windows_amd64.exe -o ctl_device.exe
```

#### 启动 Server

```bash
# 启动（本地开发，无认证）
./ctl_device server

# 生产部署（带 token）
./ctl_device server --token your-secret --config bridge.yaml

# 指定端口和状态目录
./ctl_device server --addr :3711 --state-dir ~/.config/ctl_device
```

#### 环境变量配置

```bash
export CTL_DEVICE_TOKEN=your-secret-token
export CTL_DEVICE_ADDR=:3711
export CTL_DEVICE_STATE_DIR=~/.config/ctl_device
./ctl_device server
```

### Client（IDE 配置）

#### Claude Code（.claude.json）

```json
{
  "mcpServers": {
    "ctl_device": {
      "command": "/path/to/ctl_device",
      "args": ["client", "mcp", "--server", "http://your-server:3711", "--token", "xxx"]
    }
  }
}
```

#### Trae CN

在 Trae CN 的 MCP 配置中添加：

```json
{
  "mcpServers": {
    "ctl_device": {
      "command": "ctl_device",
      "args": ["client", "mcp", "--server", "http://localhost:3711"]
    }
  }
}
```

#### Cursor

在 Cursor 的 MCP 配置中添加：

```json
{
  "mcpServers": {
    "ctl_device": {
      "command": "/path/to/ctl_device",
      "args": ["client", "mcp", "--server", "http://localhost:3711"]
    }
  }
}
```

### OpenClaw 配置

```json
{
  "mcp": {
    "servers": {
      "ctl_device": {
        "command": "ctl_device",
        "args": ["client", "mcp", "--server", "http://localhost:3711"]
      }
    }
  }
}
```

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      ctl_device Server                           │
│                  （VPS / 局域网任意一台机器）                      │
│                                                                  │
│  ┌─────────────┐  ┌─────────────────┐  ┌────────────────────┐  │
│  │ MCP SSE     │  │ JSON-RPC HTTP   │  │ Web Dashboard      │  │
│  │ :3710/sse   │  │ :3711           │  │ :3712              │  │
│  └──────┬──────┘  └────────┬────────┘  └────────────────────┘  │
│         └──────────────────┘                                    │
│                     ▼                                           │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Core Engine                                             │  │
│  │  AgentManager / ProjectStore / TaskScheduler             │  │
│  │  EventBus / Notifier / RecoveryManager                   │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│  持久化层：~/.config/ctl_device/ (JSON files)                    │
└─────────────────────────────────────────────────────────────────┘
         ▲                              ▲
         │ MCP SSE / JSON-RPC           │ MCP stdio / JSON-RPC
┌────────────────────┐      ┌──────────────────────────────────┐
│  调度者             │      │  执行者（任意机器）               │
│  OpenClaw + MCP    │      │  ctl_device client --mcp-stdio   │
│  任何有 API 的工具  │      │  配置到 Claude Code / Cursor / JB│
└────────────────────┘      └──────────────────────────────────┘
```

## 核心功能

- **分布式调度**：支持跨平台（Linux / Windows / macOS）、跨网络（局域网 / 公网）、多机并发
- **MCP 协议**：兼容 Model Context Protocol，无缝对接各种 AI IDE
- **JSON-RPC**：简单的 HTTP API，易于集成
- **容灾恢复**：断线重连、Server 重启恢复、Token 限制处理、超时重试
- **Token 认证**：可选的共享 token 认证，支持 TLS
- **Web Dashboard**：实时查看在线 Agent、项目状态、任务进度
- **事件流**：SSE 实时推送任务状态变更

## CLI 命令

```bash
# Server
ctl_device server              # 启动 server
ctl_device server --help       # 查看帮助

# Client
ctl_device client mcp          # MCP stdio 模式（供 IDE 配置）
ctl_device client status       # 查询项目/任务状态
ctl_device client dispatch     # 下发任务
ctl_device client logs         # 实时日志（SSE）
```

## 配置

### Server 配置（~/.config/ctl_device/server.yaml）

```yaml
server:
  bind: "0.0.0.0"
  jsonrpc_port: 3711
  dashboard_port: 3712
  token: "your-secret-token"  # 可选
  state_dir: "~/.config/ctl_device"
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
    auto_tls: false
    domain: ""
```

### Client 配置（~/.config/ctl_device/client.yaml）

```yaml
server: "http://localhost:3711"
token: "your-secret-token"  # 可选
agent_id: "macbook-m4"      # 唯一标识
role: "executor"             # scheduler / executor / both
capabilities: ["go", "python"]
```

## 开发

### 构建

```bash
go build ./...
```

### 测试

```bash
go test -race -timeout 5m ./...
```

### 运行

```bash
go run ./cmd/ctl_device server
```

## 协议

### MCP Tools

#### 执行者工具
- `task_get` - 拉取当前任务
- `task_status` - 更新执行状态
- `task_complete` - 提交完成报告（含 commit hash）
- `task_block` - 报告阻塞

#### 调度者工具
- `project_register` - 注册新项目
- `project_list` - 列出所有项目及状态
- `task_dispatch` - 下发任务
- `task_advance` - 验证完成，推进下一任务
- `agent_list` - 列出在线 executor
- `subscribe` - 订阅项目事件（SSE）

### JSON-RPC 方法

```
bridge.task.get / status / complete / block
bridge.project.register / list
bridge.task.dispatch / advance
bridge.agent.register / list / heartbeat
bridge.event.subscribe
```

## 许可证

MIT License
