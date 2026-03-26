# 任务 09 - Web Dashboard

## 状态: ✅ 已完成

## 描述

实现轻量 Web Dashboard，直观展示所有 agent 在线状态、项目进度、最近事件。不依赖前端框架，纯 Go 内嵌 HTML。

## 详细要求

### internal/server/dashboard.go

使用 `embed` 包内嵌静态文件，无需外部依赖：

```go
//go:embed static/*
var staticFiles embed.FS

func NewDashboard(scheduler *project.Scheduler, agentMgr *agent.Manager, eventBus *event.Bus) *Dashboard
func (d *Dashboard) Handler() http.Handler
```

### 页面结构（单页，自动刷新）

```
GET /          → dashboard 主页（HTML）
GET /api/state → 返回当前状态 JSON（前端定时轮询，5秒）
GET /stream    → SSE 流（实时事件推送到前端）
```

### 主页 UI

```
┌──────────────────────────────────────────────────────┐
│  🦞 ctl_device  v0.1.0    uptime: 2h 15m            │
├──────────────────────────────────────────────────────┤
│  Agents (2 online / 1 offline)                       │
│  ● vps-openclaw    scheduler  在线 2h                │
│  ● macbook-m4      executor   在线 45m  go,python   │
│  ○ home-pc         executor   离线 3h                │
├──────────────────────────────────────────────────────┤
│  Projects                                            │
│  scrypt-wallet  🔵 executing  task 03/08  macbook-m4 │
│  new-project    🟢 pending    task 01/05  -          │
├──────────────────────────────────────────────────────┤
│  Recent Events                                (live) │
│  12:35  scrypt-wallet task02 ✅ commit abc123        │
│  12:20  macbook-m4 connected                         │
│  12:15  scrypt-wallet task02 🔵 executing            │
│  12:10  scrypt-wallet task01 ✅ commit def456        │
└──────────────────────────────────────────────────────┘
```

### 技术实现

- 纯 HTML + CSS + 原生 JS（无框架）
- 深色主题
- SSE 实时更新事件列表（不刷新页面）
- `GET /api/state` 每 5 秒轮询更新 agent/project 状态

### internal/server/static/

```
static/
├── index.html
├── style.css
└── app.js
```

## 验收标准

1. `ctl_device server` 启动后：
   - 浏览器访问 `http://localhost:3712` 能看到 dashboard
   - 注册一个 project，页面更新显示
   - 连接一个 agent，状态显示在线
   - 触发一个事件，实时出现在 Events 列表
2. `go build ./...` 静态文件正确内嵌（二进制可独立运行）
3. 页面在 Chrome / Firefox / Safari 正常显示

## 估时

2 小时
