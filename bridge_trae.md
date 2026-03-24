# BRIDGE - OpenClaw ↔ Trae CN 协作桥

## 协议
- OpenClaw 写任务，状态设为 🟢 待执行
- Trae CN 接单后改为 🔵 执行中
- 完成后改为 ✅ 已完成，在 ## 回报 写结果
- 遇到问题改为 🔴 阻塞，在 ## 回报 写问题描述
- 每个任务完成后必须 `git add -A && git commit && git push yerikokay main`

## 当前任务

### 任务编号: 03
### 任务名称: 状态持久化层
### 状态: 🟢 待执行
### 描述:

参考 `plan/tasks/03-state-store.md` 完整实现。

核心要点：
1. internal/project/store.go：JSON 文件持久化，原子写（tmp+rename），flock 并发保护
2. internal/project/scheduler.go：Dispatch/Advance/UpdateTaskStatus/CompleteTask/BlockTask
3. Snapshot 机制：Server 重启后状态完整恢复
4. 超时看门狗 goroutine（60s 检查）
5. 单元测试：并发写、重启恢复、超时触发

### 验收标准:
1. go test ./internal/project/... 通过
2. 并发写入不丢数据（5 goroutine）
3. 写一半崩溃后，加载到上一个完整版本
4. go test -race ./... 无报警

## 回报

### 任务 01 - 项目初始化: ✅ 已完成
- 创建完整目录结构（cmd/、internal/、pkg/）
- cobra CLI：server / client 子命令
- 所有包空实现骨架
- go build ./... 通过

### 任务 02 - 协议类型定义: ✅ 已完成
- pkg/protocol/task.go：TaskStatus + Task struct
- pkg/protocol/agent.go：AgentRole + Agent struct
- pkg/protocol/project.go：Project struct
- pkg/protocol/mcp_tools.go：9 个 MCP tool schema
- pkg/protocol/jsonrpc.go：Request/Response/Error struct
- go test ./pkg/protocol/... 通过

## 历史

### 任务 01 - 项目初始化: ✅ (cobra CLI + 目录骨架)
### 任务 02 - 协议类型定义: ✅ (MCP tool schema + 数据结构)
