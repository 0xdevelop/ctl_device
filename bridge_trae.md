# BRIDGE - OpenClaw ↔ Trae CN 协作桥

## 协议
- OpenClaw 写任务，状态设为 🟢 待执行
- Trae CN 接单后改为 🔵 执行中
- 完成后改为 ✅ 已完成，在 ## 回报 写结果
- 遇到问题改为 🔴 阻塞，在 ## 回报 写问题描述
- 每个任务完成后必须 `git add -A && git commit && git push yerikokay main`

## 当前任务

### 任务编号: 04
### 任务名称: Agent 管理（心跳 + 多机路由）
### 状态: 🟢 待执行
### 描述:

参考 `plan/tasks/04-agent-manager.md` 完整实现。

核心要点：
1. `internal/agent/manager.go`：Register/Heartbeat/GetOnlineExecutors/FindExecutorForProject
2. 心跳超时检测 goroutine（45s 阈值，每 30s 检查）
3. 断线重连：Resume=true 时返回该 agent 持有的 executing 任务
4. `internal/event/bus.go`：Publish/Subscribe，Subscribe 返回 channel + unsubscribe func
5. 单元测试：注册→心跳→超时→离线→重连→恢复任务；EventBus 多订阅者；race detector

### 验收标准:
1. `go test -race ./internal/agent/... ./internal/event/... -v` 通过
2. 心跳超时后触发 AgentOffline 事件
3. resume=true 重连后返回 pending_tasks
4. go build ./... 无报错

## 回报

### 任务 03 - 状态持久化层: ✅ 已完成
- FileStore：原子写（tmp+rename）、sync.RWMutex、目录自动创建
- Scheduler：Dispatch/GetCurrentTask/UpdateTaskStatus/CompleteTask/BlockTask/Advance
- StartSnapshotLoop：定时快照
- CheckTimeouts：超时任务重置为 pending
- 测试全部通过（TestSaveLoadProject/TestAtomicWrite/TestSnapshotRoundtrip/TestSchedulerCompleteTask）
- go test -race 无报警

## 历史

### 任务 01 - 项目初始化: ✅
### 任务 02 - 协议类型定义: ✅
### 任务 03 - 状态持久化层: ✅ (commit 4af78f1)
