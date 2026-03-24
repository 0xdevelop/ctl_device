# 任务 08 - 认证 + 配置文件

## 状态: 🟢 待执行

## 描述

实现 token 认证、TLS 支持、YAML 配置文件加载，让 ctl_device 可以安全部署到公网。

## 详细要求

### internal/auth/auth.go

```go
type Authenticator struct {
    token string  // 空字符串 = 不认证（本地开发）
}

func (a *Authenticator) Validate(r *http.Request) error {
    // 支持两种方式：
    // 1. Header: Authorization: Bearer <token>
    // 2. JSON body: {"auth": {"token": "xxx"}, ...}
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler
```

### TLS 配置

```go
type TLSConfig struct {
    Enabled  bool   `yaml:"enabled"`
    CertFile string `yaml:"cert_file"`
    KeyFile  string `yaml:"key_file"`
    AutoTLS  bool   `yaml:"auto_tls"`   // 使用 autocert（Let's Encrypt）
    Domain   string `yaml:"domain"`      // auto_tls 时需要
}
```

server 启动时：
- `tls.enabled=false`：普通 HTTP（局域网用）
- `tls.enabled=true, auto_tls=false`：加载 cert_file/key_file
- `tls.enabled=true, auto_tls=true`：使用 `golang.org/x/crypto/acme/autocert`

### config/server_config.go（独立于现有 config.go）

```yaml
# bridge.yaml 示例
server:
  mcp_port: 3710
  jsonrpc_port: 3711
  dashboard_port: 3712
  bind: "0.0.0.0"          # 或 "127.0.0.1"（仅本地）
  token: "your-secret"      # 空 = 不认证
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
    auto_tls: false
    domain: ""
  state_dir: "~/.config/ctl_device"
  snapshot_interval_seconds: 30
  heartbeat_timeout_seconds: 45

notify:
  channel: "openclaw-weixin"  # weixin/telegram/discord/slack/webhook/none
  target: ""
  webhook_url: ""

projects:
  - name: scrypt-wallet
    dir: /home/ubuntu/workspace/scrypt-wallet
    tech: go
    test_cmd: "go test ./..."
    executor: "any"
    timeout_minutes: 120
```

```go
type ServerConfig struct {
    Server   ServerSection   `yaml:"server"`
    Notify   NotifySection   `yaml:"notify"`
    Projects []ProjectConfig `yaml:"projects"`
}

func LoadConfig(path string) (*ServerConfig, error)
func DefaultConfig() *ServerConfig
```

### client config（~/.config/ctl_device/client.yaml）

```yaml
server: "http://localhost:3711"
token: ""
agent_id: "my-macbook"
role: "executor"
capabilities: ["go", "python"]
```

```go
type ClientConfig struct {
    Server       string   `yaml:"server"`
    Token        string   `yaml:"token"`
    AgentID      string   `yaml:"agent_id"`
    Role         string   `yaml:"role"`
    Capabilities []string `yaml:"capabilities"`
}

func LoadClientConfig() (*ClientConfig, error)
// 按顺序查找：--config flag → ./client.yaml → ~/.config/ctl_device/client.yaml
```

### 命令行 flag 优先级

```
CLI flags > 环境变量 > 配置文件 > 默认值
```

环境变量支持：
```
CTL_DEVICE_SERVER=http://...
CTL_DEVICE_TOKEN=xxx
CTL_DEVICE_AGENT_ID=xxx
```

## 验收标准

1. `go test ./internal/auth/...` 通过
2. `go test ./config/...` 通过
3. 测试覆盖：
   - 有 token 时：正确 token 通过，错误 token 返回 401
   - 无 token（空字符串）：所有请求通过
   - YAML 加载：完整配置、缺省字段使用默认值
4. `ctl_device server --config bridge.yaml` 正常启动
5. `ctl_device server --token mytoken` 覆盖配置文件

## 估时

1.5 小时
