# 任务 03 - 状态持久化层

## 状态: 🟢 待执行

## 描述

实现状态持久化，保证 Server 重启后所有项目/任务/agent 状态完整恢复。这是容灾的核心。

## 详细要求

### internal/project/store.go

#### 存储路径
```
~/.config/ctl_device/
├── state.json          # 全局状态快照
├── projects/
│   ├── scrypt-wallet.json
│   └── new-project.json
└── tasks/
    ├── scrypt-wallet/
    │   ├── 01.json
    │   ├── 02.json
    │   └── 03.json
    └── new-project/
        └── 01.json
```

#### Store 接口
```go
type Store interface {
    // Project
    SaveProject(p *protocol.Project) error
    LoadProject(name string) (*protocol.Project, error)
    ListProjects() ([]*protocol.Project, error)
    DeleteProject(name string) error

    // Task
    SaveTask(t *protocol.Task) error
    LoadTask(projectName, taskNum string) (*protocol.Task, error)
    ListTasks(projectName string) ([]*protocol.Task, error)
    DeleteTask(projectName, taskNum string) error

    // State snapshot（Server 重启恢复）
    SaveSnapshot(s *Snapshot) error
    LoadSnapshot() (*Snapshot, error)
}

type Snapshot struct {
    Version   string              `json:"version"`
    SavedAt   time.Time           `json:"saved_at"`
    Projects  []*protocol.Project `json:"projects"`
    Tasks     []*protocol.Task    `json:"tasks"`
    Agents    []*protocol.Agent   `json:"agents"`
}
```

#### 实现要求
- 使用文件锁（`flock` 等价，Go 用 `syscall.Flock` 或 `github.com/gofrs/flock`）防止并发写
- 写操作：先写临时文件，再 `os.Rename`（原子操作，防止写一半崩溃）
- 读操作：容忍文件不存在（首次启动）
- 快照每 30 秒自动保存一次（goroutine）
- Server 启动时调用 `LoadSnapshot` 恢复状态

### internal/project/scheduler.go

```go
type Scheduler struct {
    store    Store
    agents   *agent.Manager
    eventBus *event.Bus
}

// 核心方法
func (s *Scheduler) Dispatch(projectName string, task *protocol.Task) error
func (s *Scheduler) Advance(projectName string) error   // 验证完成 → 下发下一任务
func (s *Scheduler) GetCurrentTask(projectName string) (*protocol.Task, error)
func (s *Scheduler) UpdateTaskStatus(projectName, taskNum string, status protocol.TaskStatus) error
func (s *Scheduler) CompleteTask(projectName, taskNum string, report *CompleteReport) error
func (s *Scheduler) BlockTask(projectName, taskNum string, reason string) error

// 容灾相关
func (s *Scheduler) HandleExecutorReconnect(agentID string) error
func (s *Scheduler) CheckTimeouts() error  // 定期检查超时任务
```

### 超时检查 goroutine

```go
// 每 60 秒检查一次
func (s *Scheduler) startTimeoutWatcher(ctx context.Context) {
    ticker := time.NewTicker(60 * time.Second)
    for {
        select {
        case <-ticker.C:
            s.CheckTimeouts()
        case <-ctx.Done():
            return
        }
    }
}
```

超时逻辑：
- 任务状态为 `executing`
- `started_at` 距现在超过 `timeout_minutes`
- 发送通知 → 等待 30 分钟 → 自动重置为 `pending`

## 验收标准

1. `go test ./internal/project/...` 通过
2. 测试覆盖：
   - 写入 project/task 后重启（重新加载），数据一致
   - 并发写入不丢数据（5 goroutine 同时写）
   - 写一半崩溃（模拟）后，加载到的是上一个完整版本
3. 快照保存/加载 round-trip 测试
4. 超时检查：任务超时后状态正确变更

## 估时

2 小时
