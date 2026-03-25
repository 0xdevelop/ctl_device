# BRIDGE - OpenClaw ↔ Trae CN 协作桥

## 协议
- OpenClaw 写任务，状态设为 🟢 待执行
- Trae CN 接单后改为 🔵 执行中
- 完成后改为 ✅ 已完成，在 ## 回报 写结果
- 遇到问题改为 🔴 阻塞，在 ## 回报 写问题描述
- 每个任务完成后必须 `git add -A && git commit && git push yerikokay main`

## 当前任务

### 任务编号: 06
### 任务名称: MCP Server（SSE + stdio）
### 状态: 🟢 待执行
### 描述:

参考 `plan/tasks/06-mcp-server.md` 完整实现。

核心要点：
1. `internal/server/mcp_stdio.go` — MCP stdio transport，从 stdin 读 JSON-RPC，向 stdout 写响应
2. `internal/server/mcp_sse.go` — MCP SSE server 监听 :3710（GET /sse + POST /message）
3. MCP 握手：处理 initialize / tools/list / tools/call
4. `pkg/protocol/mcp_tools.go` — 完整实现所有 9 个 tool 的 schema（task_get/task_complete/task_block/task_status/project_register/project_list/task_dispatch/task_advance/agent_list）
5. `ctl_device client mcp` — stdio 模式代理到远程 JSON-RPC server
6. 单元测试：MCP 握手、tools/list、tools/call

### 验收标准:
1. `go test ./internal/server/... -v` 通过（含 MCP 握手测试）
2. `go build ./...` 无报错
3. `go test -race ./...` 无报警
4. stdio 模式：发 initialize → 收到 serverInfo；发 tools/list → 收到所有工具

## 回报

### 已完成：01✅ 02✅ 03✅ 04✅ 05✅

## 历史
task01: cobra CLI骨架 | task02: 协议类型定义
task03: 状态持久化原子写+快照 | task04: Agent心跳+EventBus
task05: JSON-RPC Server+SSE事件流+CLI client
