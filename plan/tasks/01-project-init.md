# 任务 01 - 项目初始化

## 状态: 🟢 待执行

## 描述

初始化 Go 项目结构，建立所有包骨架，确保可以编译通过。

## 详细要求

### 目录结构
```
ctl_device/
├── cmd/ctl_device/
│   └── main.go              # 入口：解析 server/client 子命令
├── internal/
│   ├── server/
│   │   ├── mcp_sse.go       # 空实现占位
│   │   ├── mcp_stdio.go     # 空实现占位
│   │   ├── jsonrpc.go       # 空实现占位
│   │   └── dashboard.go     # 空实现占位
│   ├── agent/
│   │   ├── manager.go       # Agent 管理
│   │   └── registry.go      # Agent 注册表
│   ├── project/
│   │   ├── store.go         # 项目/任务持久化
│   │   └── scheduler.go     # 任务调度
│   ├── event/
│   │   └── bus.go           # 事件总线
│   ├── notify/
│   │   └── notify.go        # 通知（stub）
│   └── recovery/
│       └── recovery.go      # 容灾恢复
├── pkg/protocol/
│   ├── task.go              # Task / Status 数据结构
│   ├── agent.go             # Agent 数据结构
│   ├── project.go           # Project 数据结构
│   └── mcp_tools.go         # MCP tool schema 定义
├── config/
│   └── config.go            # 版本号（保持现有）
├── plan/                    # 已存在，不改
├── go.mod                   # 更新 module name
└── README.md                # 更新
```

### go.mod
- module 名保持 `github.com/0xdevelop/ctl_device`
- Go 版本：`1.22`（兼容 CI）
- 初始依赖：
  - `github.com/spf13/cobra` - CLI
  - `github.com/sourcegraph/jsonrpc2` 或 `net/http` 内置（先用内置）

### main.go 骨架
```go
func main() {
    // cobra root command
    // 子命令: server, client
    // client 子命令: mcp, status, dispatch, logs
}
```

### 所有内部包
- 只需要定义类型和空函数签名，能 `go build ./...` 通过即可
- 不需要实现逻辑

## 验收标准

1. `go build ./...` 无报错
2. `go vet ./...` 无报错
3. `./ctl_device --help` 能显示帮助
4. `./ctl_device server --help` 能显示帮助
5. `./ctl_device client --help` 能显示帮助
6. `go test ./...` 通过（只有空测试）

## 估时

45 分钟
