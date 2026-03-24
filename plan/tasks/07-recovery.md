# 任务 07 - 容灾恢复系统

## 状态: 🟢 待执行

## 描述

实现所有断线/断电/token限制场景的自动恢复逻辑，确保任务不丢失、不重复、自动续接。

## 详细要求

### internal/recovery/recovery.go

```go
type Manager struct {
    scheduler  *project.Scheduler
    agentMgr   *agent.Manager
    notifier   *notify.Notifier
    eventBus   *event.Bus
}

func (m *Manager) Start(ctx context.Context)
// 订阅所有 agent/task 事件，驱动恢复逻辑
```

### 场景1：执行者断线重连

```go
func (m *Manager) OnAgentReconnect(agentID string) error {
    // 1. 找到该 agent 持有的 executing 任务
    // 2. 任务状态 < 30 分钟超时：推送恢复消息给 agent
    //    → agent 收到 pending_tasks，继续执行
    // 3. 任务超时：重置为 pending，重新分配
}
```

### 场景2：Server 重启恢复

```go
func (m *Manager) OnServerStart() error {
    // 在 store.LoadSnapshot() 之后调用
    // 1. 检查所有 executing 任务
    // 2. 对应 executor 在线：发送恢复通知
    // 3. 对应 executor 离线：标记 executor_offline，等重连
    // 4. 检查超时任务，重置
    // 5. 打印恢复摘要日志
}
```

### 场景3：调度者（OpenClaw）断开重连

```go
func (m *Manager) OnSchedulerReconnect(agentID string) (*RecoverySummary, error) {
    // 返回期间所有状态变更摘要：
    // {
    //   completed: [{task, commit, report}],
    //   blocked: [{task, reason}],
    //   in_progress: [{task, executor, started_at}],
    //   pending: [{task}]
    // }
    // OpenClaw 收到摘要后可以继续 advance/dispatch
}
```

### 场景4：token 限制（executor_limit 状态）

```go
func (m *Manager) HandleExecutorLimit(project, taskNum, agentID string) error {
    // 1. 任务状态 → executor_limit
    // 2. 发送通知（告知用户）
    // 3. 记录 limit_at 时间
    // 不自动重试，等 executor 调用 task_get 重新拿任务（幂等）
}
```

executor 恢复后只需调用 `task_get`，Server 返回同一个任务，状态重置为 `executing`。

### 场景5：超时自动重置

```go
func (m *Manager) CheckTimeouts() {
    // 定期（60s）检查所有 executing 任务
    // 超时判断：now - started_at > timeout_minutes
    // 步骤：
    //   1. 状态 → timeout，发通知
    //   2. 等 30 分钟（通过定时任务）
    //   3. 状态 → pending，assigned_to 清空
    //   4. 重新等待 executor 接单
}
```

### 场景6：Git push 失败

executor 调用 `task_complete` 时，加一个 `push_failed bool` 字段：
- `push_failed=false`：正常流程
- `push_failed=true`：任务标记 `commit_pending`，commit hash 已记录
  - executor 恢复后调用 `task_complete` 补发（幂等，同一 commit hash 不重复处理）

### internal/notify/notify.go

```go
type Notifier struct {
    channel string  // openclaw-weixin / telegram / webhook / none
    target  string
}

func (n *Notifier) Send(msg string) error {
    switch n.channel {
    case "openclaw-weixin", "telegram", "discord", "slack":
        // exec: openclaw message send --channel xxx --message "..."
    case "webhook":
        // HTTP POST to n.target
    case "none":
        // log only
    }
}

// 预定义消息
func (n *Notifier) TaskCompleted(project, taskNum, commit string)
func (n *Notifier) TaskBlocked(project, taskNum, reason string)
func (n *Notifier) AgentOffline(agentID, taskID string)
func (n *Notifier) AgentReconnected(agentID string)
func (n *Notifier) TaskTimeout(project, taskNum string)
func (n *Notifier) ServerRestarted(summary *RecoverySummary)
```

## 验收标准

1. `go test ./internal/recovery/...` 通过
2. 集成测试（internal/server/integration_test.go）：
   - 模拟断线重连：任务正确恢复
   - 模拟 server 重启：状态从文件恢复，executing 任务不丢
   - 模拟超时：30+分钟后任务重置为 pending
   - 幂等测试：同一 task_complete 调用两次不出错
3. `go test -race ./...` 无报警

## 估时

3 小时
