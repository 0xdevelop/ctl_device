# 任务 04 - Agent 管理（心跳 + 多机路由）

## 状态: 🟢 待执行

## 描述

实现 Agent 注册、心跳检测、在线状态管理，以及执行者断线重连后的任务恢复。

## 详细要求

### internal/agent/manager.go

```go
type Manager struct {
    agents   map[string]*protocol.Agent  // agent_id → Agent
    mu       sync.RWMutex
    store    project.Store
    eventBus *event.Bus
}

// 注册（执行者上线）
func (m *Manager) Register(req *RegisterRequest) (*RegisterResponse, error)
// RegisterRequest: {agent_id, role, capabilities, projects, resume: bool}
// RegisterResponse: {ok, pending_tasks: []Task}  // resume=true 时返回待恢复任务

// 心跳（每 15 秒）
func (m *Manager) Heartbeat(agentID string) error

// 查询
func (m *Manager) GetOnlineExecutors() []*protocol.Agent
func (m *Manager) FindExecutorForProject(projectName string) (*protocol.Agent, error)
// 优先找指定 executor，其次找 capabilities 匹配的在线 executor

// 内部：心跳超时检测（每 30 秒）
func (m *Manager) startHeartbeatWatcher(ctx context.Context)
// 超时阈值：45 秒（3 次心跳）
// 超时后：标记离线，发布 AgentOffline 事件，通知调度层
```

### 断线重连流程

```
executor 断线 (心跳超时 45s)
    → AgentManager 标记离线
    → EventBus 发布 AgentOffline{agentID, taskID}
    → TaskScheduler 订阅事件，任务状态改为 executor_offline（子状态）
    → Notifier 发送告警

executor 重新上线
    → Register(resume=true)
    → Manager 检查该 agent 持有的任务
    → 返回 pending_tasks（status=executing 或 executor_offline）
    → executor 收到后继续执行（调用 task_get 获取详情）
```

### internal/agent/registry.go

持久化 agent 配置（不是在线状态，在线状态是 in-memory）：

```go
type Registry struct {
    store project.Store
}

func (r *Registry) Save(agent *protocol.Agent) error
func (r *Registry) Load(agentID string) (*protocol.Agent, error)
func (r *Registry) LoadAll() ([]*protocol.Agent, error)
```

### internal/event/bus.go

```go
type EventType string
const (
    EventTaskStatusChanged EventType = "task_status_changed"
    EventTaskCompleted     EventType = "task_completed"
    EventTaskBlocked       EventType = "task_blocked"
    EventAgentOnline       EventType = "agent_online"
    EventAgentOffline      EventType = "agent_offline"
    EventProjectRegistered EventType = "project_registered"
)

type Event struct {
    Type      EventType   `json:"type"`
    Project   string      `json:"project,omitempty"`
    AgentID   string      `json:"agent_id,omitempty"`
    TaskID    string      `json:"task_id,omitempty"`
    Payload   interface{} `json:"payload"`
    Timestamp time.Time   `json:"timestamp"`
}

type Bus struct { ... }

func (b *Bus) Publish(e Event)
func (b *Bus) Subscribe(ch chan<- Event, filters ...EventType) (unsubscribe func())
// unsubscribe 是关闭 subscription 的函数，避免 goroutine 泄漏
```

## 验收标准

1. `go test ./internal/agent/...` 通过
2. `go test ./internal/event/...` 通过
3. 测试覆盖：
   - 注册 → 心跳 → 超时 → 标记离线 → 事件触发
   - 断线重连 → 返回待恢复任务
   - 多 executor 同时注册，FindExecutorForProject 路由正确
   - EventBus：发布事件，多个订阅者都收到；取消订阅后不再收到
4. 并发安全（race detector 无报警）：`go test -race ./...`

## 估时

2 小时
