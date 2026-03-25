# BRIDGE - OpenClaw ↔ Trae CN 协作桥

## 协议
- OpenClaw 写任务，状态设为 🟢 待执行
- Trae CN 接单后改为 🔵 执行中
- 完成后改为 ✅ 已完成，在 ## 回报 写结果
- 遇到问题改为 🔴 阻塞，在 ## 回报 写问题描述
- 每个任务完成后必须 `git add -A && git commit && git push yerikokay main`

## 当前任务

### 任务编号: 05
### 任务名称: JSON-RPC Server
### 状态: 🟢 待执行
### 描述:

参考 `plan/tasks/05-jsonrpc-server.md` 完整实现。

核心要点：
1. `internal/server/jsonrpc.go` — HTTP server 监听 :3711，路由 POST /rpc
2. token 认证中间件（空 token = 跳过认证）
3. 实现所有 RPC 方法：task.get/status/complete/block，agent.register/heartbeat/list，project.register/list，task.dispatch/advance，event.subscribe
4. GET /events — SSE 事件流
5. `internal/client/jsonrpc_client.go` — Go 客户端封装
6. CLI 命令绑定：`ctl_device client status/dispatch/logs`
7. httptest 集成测试

### 验收标准:
1. `go test ./internal/server/... -v` 通过
2. `go test -race ./...` 无报警
3. `ctl_device server` 启动后 curl POST /rpc 能返回 JSON
4. `ctl_device client status` 能输出结果

## 回报

### 任务 01 - 项目初始化: ✅
### 任务 02 - 协议类型定义: ✅
### 任务 03 - 状态持久化: ✅ (原子写+快照+超时检测，race通过)
### 任务 04 - Agent管理: ✅ (心跳检测+断线恢复+EventBus，race通过)

## 历史

task01: cobra CLI + 目录骨架
task02: MCP tool schema + 数据结构
task03: FileStore原子写、Snapshot恢复、Scheduler、超时看门狗
task04: AgentManager心跳/断线重连、EventBus发布订阅
