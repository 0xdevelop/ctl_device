# BRIDGE - OpenClaw ↔ Trae CN 协作桥

## 协议
- OpenClaw 写任务，状态设为 🟢 待执行
- Trae CN 接单后改为 🔵 执行中
- 完成后改为 ✅ 已完成，在 ## 回报 写结果
- 遇到问题改为 🔴 阻塞，在 ## 回报 写问题描述
- 每个任务完成后必须 `git add -A && git commit && git push yerikokay main`

## 当前任务

### 任务编号: 07
### 任务名称: 容灾恢复系统
### 状态: 🟢 待执行
### 描述:

参考 `plan/tasks/07-recovery.md` 完整实现。

核心要点：
1. `internal/recovery/recovery.go` — Manager，订阅所有 agent/task 事件，驱动恢复逻辑
2. 场景1：执行者断线重连 → OnAgentReconnect，找回持有任务继续
3. 场景2：Server 重启 → OnServerStart，从 snapshot 恢复，通知在线 executor
4. 场景3：调度者重连 → OnSchedulerReconnect，返回期间所有状态变更摘要
5. 场景4：token 限制 → HandleExecutorLimit，任务标记 executor_limit，幂等恢复
6. 场景5：超时自动重置 → CheckTimeouts，超时→通知→30min→重置pending
7. 场景6：Git push 失败 → commit_pending 状态，幂等重试
8. `internal/notify/notify.go` — 完整实现：openclaw-weixin/telegram/webhook/none 多渠道
9. 集成测试：模拟断线重连、server重启、超时重置、幂等调用

### 验收标准:
1. `go test ./internal/recovery/... -v` 通过
2. `go test -race ./...` 无报警
3. `go build ./...` 无报错
4. 断线重连测试：任务正确恢复，不丢不重

## 回报

### 已完成：01✅ 02✅ 03✅ 04✅ 05✅ 06✅

## 历史
task01~04: 骨架+协议+持久化+Agent心跳
task05: JSON-RPC Server+SSE
task06: MCP Server stdio+SSE，tools/list/call，MCP握手
