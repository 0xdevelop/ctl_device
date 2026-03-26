# BRIDGE - OpenClaw ↔ Trae CN 协作桥

## 协议
- OpenClaw 写任务，状态设为 🟢 待执行
- Trae CN 接单后改为 🔵 执行中
- 完成后改为 ✅ 已完成，在 ## 回报 写结果
- 遇到问题改为 🔴 阻塞，在 ## 回报 写问题描述
- 每个任务完成后必须 `git add -A && git commit && git push yerikokay main`

## 当前任务

### 任务编号: 08
### 任务名称: 认证 + 配置文件
### 状态: 🟢 待执行
### 描述:

参考 `plan/tasks/08-auth-config.md` 完整实现。

核心要点：
1. `internal/auth/auth.go` — token 认证中间件（Header Bearer 或 JSON body auth.token，空 token 跳过）
2. TLS 配置结构（enabled/cert_file/key_file/auto_tls/domain）
3. `config/server_config.go` — YAML 配置加载，ServerConfig struct（server/notify/projects 三段）
4. `~/.config/ctl_device/client.yaml` 客户端配置
5. CLI flag 优先级：flags > 环境变量（CTL_DEVICE_SERVER/TOKEN/AGENT_ID）> 配置文件 > 默认值
6. `go get gopkg.in/yaml.v3` 作为 YAML 解析依赖
7. 单元测试：token 验证、YAML 加载、flag 覆盖配置文件

### 验收标准:
1. `go test ./internal/auth/... -v` 通过
2. `go test ./config/... -v` 通过
3. `go test -race ./...` 无报警
4. `ctl_device server --config bridge.yaml` 正常启动
5. `ctl_device server --token mytoken` 覆盖配置文件

## 回报

### 已完成：01✅ 02✅ 03✅ 04✅ 05✅ 06✅ 07✅

## 历史
task01~06: 骨架/协议/持久化/Agent/JSON-RPC/MCP
task07: 容灾恢复6场景（断线/重启/调度者重连/token限制/超时/push失败），race通过
