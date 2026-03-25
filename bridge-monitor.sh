#!/bin/bash
# ctl_device 项目 bridge 监控脚本
# 监听 bridge_trae.md 状态变化，自动推进任务

set -euo pipefail

PROJECT_DIR="/home/ubuntu/workspace/ctl_device"
BRIDGE_FILE="$PROJECT_DIR/bridge_trae.md"
STATE_FILE="$PROJECT_DIR/.bridge-state.json"
TASK_DIR="$PROJECT_DIR/plan/tasks"
BRIDGE_TASK_FILE="$HOME/bridge/task.txt"
TRAE_PROMPT_FILE="$HOME/bridge/trae-prompt.txt"
LOG_FILE="/tmp/ctl_device-monitor.log"

TASK_SEQUENCE=("01" "02" "03" "04" "05" "06" "07" "08" "09" "10")
TASK_NAMES=(
    "project-init"
    "protocol-types"
    "state-store"
    "agent-manager"
    "jsonrpc-server"
    "mcp-server"
    "recovery"
    "auth-config"
    "dashboard"
    "integration-ci"
)

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"; }

get_status() {
    if [ -f "$STATE_FILE" ]; then
        jq -r '.status' "$STATE_FILE" 2>/dev/null || echo ""
    else
        grep -oP '### 状态:\s*\K.*' "$BRIDGE_FILE" 2>/dev/null | head -1 | xargs || echo ""
    fi
}

get_task_num() {
    if [ -f "$STATE_FILE" ]; then
        jq -r '.task_num' "$STATE_FILE" 2>/dev/null || echo ""
    else
        grep -oP '### 任务编号:\s*\K.*' "$BRIDGE_FILE" 2>/dev/null | head -1 | tr -d ' ' || echo ""
    fi
}

dispatch_task() {
    local num="$1"
    local task_file="$TASK_DIR/${num}-*.md"
    local task_file_match
    task_file_match=$(ls $task_file 2>/dev/null | head -1)

    if [ -z "$task_file_match" ]; then
        log "找不到任务文件: $task_file"
        return 1
    fi

    log "下发任务 $num: $task_file_match"

    # 写入 bridge_trae.md（保留协议头，更新当前任务）
    local task_content
    task_content=$(cat "$task_file_match")

    # 更新 .bridge-state.json
    local idx=$(( 10#$num - 1 ))
    local task_name="${TASK_NAMES[$idx]:-task-$num}"
    jq --arg num "$num" --arg name "$task_name" --arg ts "$(date -Iseconds)" \
        '.task_num=$num | .task_name=$name | .status="🟢 待执行" | .updated_at=$ts | .commit=""' \
        "$STATE_FILE" > "${STATE_FILE}.tmp" && mv "${STATE_FILE}.tmp" "$STATE_FILE"

    # 更新 bridge_trae.md 当前任务区块
    python3 - <<PYEOF
import re, sys

bridge = open("$BRIDGE_FILE").read()
task_content = open("$task_file_match").read()

new_task = f"""## 当前任务

### 任务编号: $num
### 任务名称: $task_name
### 状态: 🟢 待执行
### 描述:

参考 \`plan/tasks/${num}-*.md\` 完整实现。完成后：
- 状态改为 ✅ 已完成，在 ## 回报 写结果
- git add -A && git commit -m "feat: task$num $task_name"
- git push yerikokay main

### 验收标准:
"""

# 提取验收标准
criteria = re.search(r'## 验收标准\n(.*?)(?=\n##|\Z)', task_content, re.DOTALL)
if criteria:
    new_task += criteria.group(1).strip()

# 替换当前任务区块
bridge_new = re.sub(r'## 当前任务.*?(?=## 回报|## 历史|\Z)', new_task + '\n\n', bridge, flags=re.DOTALL)
open("$BRIDGE_FILE", 'w').write(bridge_new)
print("bridge_trae.md updated")
PYEOF

    # 派发给 Trae（写入 task.txt 触发 watcher）
    local prompt
    prompt=$(cat "$TRAE_PROMPT_FILE")
    echo "$prompt

项目路径: $PROJECT_DIR
当前任务: $num - $task_name
任务详情文件: plan/tasks/$(basename $task_file_match)" > "$BRIDGE_TASK_FILE"

    log "任务 $num 已写入 task.txt，等待 trae-watcher 发送"
}

verify_and_advance() {
    local num="$1"
    log "验证任务 $num 完成..."

    cd "$PROJECT_DIR"

    # 拉取最新代码
    git fetch yerikokay main 2>/dev/null || true
    git pull yerikokay main 2>/dev/null || true

    # 运行测试
    if go build ./... 2>/dev/null && go test ./... 2>/dev/null; then
        log "✅ 任务 $num 验证通过"

        # 更新 progress.md
        sed -i "s/| $num | .* | 🟢 待执行/& /" "$PROJECT_DIR/plan/progress.md" 2>/dev/null || true
        sed -i "s/🟢 待执行\(.*任务 $num\)/✅ 已完成\1/" "$PROJECT_DIR/plan/progress.md" 2>/dev/null || true

        # 下发下一个任务
        local next_idx=$(( 10#$num ))
        if [ $next_idx -lt ${#TASK_SEQUENCE[@]} ]; then
            local next_num="${TASK_SEQUENCE[$next_idx]}"
            log "下发下一任务: $next_num"
            sleep 2
            dispatch_task "$next_num"
        else
            log "🎉 所有任务完成！"
            # 触发 git_tag.sh 发布
            cd "$PROJECT_DIR" && bash git_tag.sh 2>&1 | tee -a "$LOG_FILE"
        fi
    else
        log "❌ 任务 $num 验证失败，go build/test 未通过"
    fi
}

handle_state_change() {
    local status
    status=$(get_status)
    local num
    num=$(get_task_num)

    [ -z "$status" ] && return
    [ -z "$num" ] && return

    log "状态变更: 任务$num = $status"

    case "$status" in
        *"已完成"*|*"completed"*)
            verify_and_advance "$num"
            ;;
        *"阻塞"*|*"blocked"*)
            log "⚠️ 任务 $num 阻塞，需要人工介入"
            ;;
    esac
}

# ===== 主逻辑 =====
log "ctl_device bridge monitor 启动"
log "监听: $BRIDGE_FILE 和 $STATE_FILE"

# 先检查当前状态，看是否需要恢复
current_status=$(get_status)
current_num=$(get_task_num)
log "当前状态: 任务$current_num = $current_status"

# 如果是待执行且 task.txt 为空，重新派发一次（恢复用）
if [[ "$current_status" == *"待执行"* ]]; then
    if [ ! -s "$BRIDGE_TASK_FILE" ]; then
        log "检测到待执行任务，重新派发给 Trae..."
        dispatch_task "$current_num"
    fi
fi

# 事件驱动：监听文件变更
LAST_EVENT=0
watch_files() {
    inotifywait -q -m "$PROJECT_DIR" \
        -e close_write -e moved_to \
        --include "(bridge_trae\.md|\.bridge-state\.json)" \
        --format "%e %f" 2>/dev/null |
    while read event file; do
        local now
        now=$(date +%s%3N)
        local diff=$(( now - LAST_EVENT ))
        if [ $diff -lt 1000 ]; then continue; fi
        LAST_EVENT=$now
        sleep 1  # 等文件写完
        handle_state_change
    done
}

# 看门狗：每5分钟强制检查一次
watchdog() {
    while true; do
        sleep 300
        handle_state_change
    done
}

watch_files &
WATCH_PID=$!
watchdog &
DOG_PID=$!

trap "kill $WATCH_PID $DOG_PID 2>/dev/null; log '监控退出'" EXIT

log "监控就绪，等待 Trae 更新状态..."
wait
