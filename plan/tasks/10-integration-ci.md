# 任务 10 - 集成测试 + CI/CD

## 状态: 🟢 待执行

## 描述

端到端集成测试，更新 GitHub Actions，跨平台构建并发布二进制。

## 详细要求

### 集成测试（internal/integration/e2e_test.go）

```go
// TestFullWorkflow：完整工作流
func TestFullWorkflow(t *testing.T) {
    // 1. 启动 ctl_device server（测试端口）
    // 2. 注册一个 project
    // 3. 注册 executor agent
    // 4. dispatcher 下发 task
    // 5. executor 调用 task_get，拿到任务
    // 6. executor 调用 task_status(executing)
    // 7. executor 调用 task_complete(commit=abc)
    // 8. dispatcher 收到 SSE 事件 task_completed
    // 9. dispatcher 调用 task_advance
    // 10. 验证状态正确
}

// TestDisconnectRecovery：断线恢复
func TestDisconnectRecovery(t *testing.T) {
    // executor 执行到一半
    // 模拟断线（停止心跳）
    // 等待 server 标记离线
    // executor 重新注册（resume=true）
    // 验证拿回同一任务
}

// TestServerRestart：server 重启恢复
func TestServerRestart(t *testing.T) {
    // 注册项目，下发任务，executor 开始执行
    // 停止 server
    // 重启 server
    // 验证状态从文件正确恢复
}

// TestMCPStdio：MCP stdio 协议
func TestMCPStdio(t *testing.T) {
    // 启动 MCP stdio server
    // 发送 initialize，验证握手
    // 发送 tools/list，验证工具列表
    // 调用 task_get，验证返回
}

// TestTokenAuth：认证
func TestTokenAuth(t *testing.T) {
    // 配置 token="secret"
    // 无 token 请求 → 401
    // 错误 token → 401
    // 正确 token → 200
}
```

### 更新 .github/workflows/release.yml

#### 测试矩阵
```yaml
Testing:
  strategy:
    matrix:
      os: [ubuntu-latest, windows-latest, macos-latest]
      arch: [amd64, arm64]
      exclude:
        - os: windows-latest
          arch: arm64
  steps:
    - go test -race -timeout 5m ./...  # 加 -race flag
```

#### 构建矩阵（跨平台二进制）
```yaml
Build:
  strategy:
    matrix:
      include:
        - os: ubuntu-latest,   GOOS: linux,   GOARCH: amd64
        - os: ubuntu-latest,   GOOS: linux,   GOARCH: arm64
        - os: windows-latest,  GOOS: windows, GOARCH: amd64
        - os: macos-latest,    GOOS: darwin,  GOARCH: amd64
        - os: macos-latest,    GOOS: darwin,  GOARCH: arm64
  steps:
    - go build -ldflags="-X main.Version=${TAG}" -o ctl_device_${GOOS}_${GOARCH} ./cmd/ctl_device
    - 上传到 Release artifacts
```

### README.md 更新

```markdown
# ctl_device

分布式 AI 任务调度中间层，连接 OpenClaw 与 Trae CN / Claude Code / Cursor。

## 快速开始

### Server
```bash
# 下载最新版
curl -L https://github.com/0xdevelop/ctl_device/releases/latest/download/ctl_device_linux_amd64 -o ctl_device
chmod +x ctl_device

# 启动（本地开发，无认证）
./ctl_device server

# 生产部署（带 token）
./ctl_device server --token your-secret --config bridge.yaml
```

### Client（IDE 配置）
```bash
# Claude Code（.claude.json）
{
  "mcpServers": {
    "ctl_device": {
      "command": "/path/to/ctl_device",
      "args": ["client", "mcp", "--server", "http://your-server:3711", "--token", "xxx"]
    }
  }
}
```

### OpenClaw 配置
```json
{
  "mcp": {
    "servers": {
      "ctl_device": {
        "command": "ctl_device",
        "args": ["client", "mcp", "--server", "http://localhost:3711"]
      }
    }
  }
}
```
```

## 验收标准

1. 所有集成测试通过
2. `go test -race -timeout 5m ./...` 在 ubuntu/windows/macos 三平台通过
3. GitHub Actions 成功构建 5 个平台二进制
4. Release 包含所有二进制 + sha256 校验
5. README 安装说明可正常跟随执行

## 估时

2 小时
