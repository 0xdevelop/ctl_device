#!/bin/bash
# ctl_device 项目 bridge 监控脚本 v2
# 修复：直接读 bridge_trae.md 判断状态（Trae 只改这个文件）
# 修复：LAST_EVENT 用临时文件跨子进程共享，防抖生效

set -euo pipefail

PROJECT_DIR="/home/ubuntu/workspace/ctl_device"
BRIDGE_FILE="$PROJECT_DIR/bridge_trae.md"
STATE_FILE="$PROJECT_DIR/.bridge-state.json"
TASK_DIR="$PROJECT_DIR/plan/tasks"
BRIDGE_TASK_FILE="$HOME/bridge/task.txt"
TRAE_PROMPT_FILE="$HOME/bridge/trae-prompt.txt"
LOG_FILE="/tmp/ctl_device-monitor.log"
LAST_EVENT_FILE="/tmp/ctl_device-last-event"

echo 0 > "$LAST_EVENT_FILE"

TASK_SEQUENCE=("01" "02" "03" "04" "05" "06" "07" "08" "09" "10")
TASK_NAMES=(
    "project-init" "protocol-types" "state-store" "agent-manager"
    "jsonrpc-server" "mcp-server" "recovery" "auth-config"
    "dashboard" "integration-ci"
)

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"; }

# 直接从 bridge_trae.md 读状态（Trae 更新的是这个文件）
get_status() {
    grep -oP '### 状态:\s*\K.*' "$BRIDGE_FILE" 2>/dev/null | head -1 | xargs || echo ""
}

get_task_num() {
    grep -oP '### 任务编号:\s*\K.*' "$BRIDGE_FILE" 2>/dev/null | head -1 | tr -d ' ' || echo ""
}

dispatch_task() {
    local num="$1"
    local task_file_match
    task_file_match=$(ls "$TASK_DIR/${num}"-*.md 2>/dev/null | head -1)
    [ -z "$task_file_match" ] && { log "找不到任务文件: $TASK_DIR/${num}-*.md"; return 1; }

    local idx=$(( 10#$num - 1 ))
    local task_name="${TASK_NAMES[$idx]:-task-$num}"
    log "下发任务 $num ($task_name)"

    # 更新 .bridge-state.json
    jq --arg num "$num" --arg name "$task_name" --arg ts "$(date -Iseconds)" \
        '.task_num=$num | .task_name=$name | .status="🟢 待执行" | .updated_at=$ts | .commit=""' \
        "$STATE_FILE" > "${STATE_FILE}.tmp" && mv "${STATE_FILE}.tmp" "$STATE_FILE"

    # 更新 bridge_trae.md
    local criteria
    criteria=$(python3 -c "
import re, sys
content = open('$task_file_match').read()
m = re.search(r'## 验收标准\n(.*?)(?=\n##|\Z)', content, re.DOTALL)
print(m.group(1).strip() if m else '参见任务文件')
" 2>/dev/null || echo "参见任务文件")

    # 保留协议头和历史，替换当前任务区块
    python3 - "$BRIDGE_FILE" "$num" "$task_name" "$task_file_match" << 'PYEOF'
import sys, re

bridge_path, num, task_name, task_file = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
bridge = open(bridge_path).read()

new_task = f"""## 当前任务

### 任务编号: {num}
### 任务名称: {task_name}
### 状态: 🟢 待执行
### 描述:

参考 `plan/tasks/{num}-{task_name}.md` 完整实现。
完成后：状态改为 ✅ 已完成，写回报，然后 git add -A && git commit && git push yerikokay main

"""
bridge_new = re.sub(r'## 当前任务.*?(?=## 回报|## 历史|\Z)', new_task, bridge, flags=re.DOTALL)
open(bridge_path, 'w').write(bridge_new)
PYEOF

    # 派发给 Trae
    local prompt
    prompt=$(cat "$TRAE_PROMPT_FILE")
    printf '%s\n\n项目路径: %s\n当前任务: %s - %s\n任务详情: plan/tasks/%s\n完成后: git add -A && git commit -m "feat: task%s %s" && git push yerikokay main\n' \
        "$prompt" "$PROJECT_DIR" "$num" "$task_name" "$(basename "$task_file_match")" "$num" "$task_name" \
        > "$BRIDGE_TASK_FILE"

    log "任务 $num 已写入 task.txt"
}

verify_and_advance() {
    local num="$1"
    log "验证任务 $num..."

    cd "$PROJECT_DIR"
    git pull yerikokay main --rebase -q 2>/dev/null || true

    if go build ./... 2>/dev/null && go test -race ./... 2>/dev/null; then
        log "✅ 任务 $num 验证通过（go build + race test）"

        # 更新 progress.md
        sed -i "s/| $num | .* | 🟢 待执行/& /" plan/progress.md 2>/dev/null || true

        local next_idx=$(( 10#$num ))
        if [ "$next_idx" -lt "${#TASK_SEQUENCE[@]}" ]; then
            local next_num="${TASK_SEQUENCE[$next_idx]}"
            sleep 3
            dispatch_task "$next_num"
        else
            log "🎉 所有任务完成！触发发布..."
            bash git_tag.sh 2>&1 | tee -a "$LOG_FILE"
        fi
    else
        log "❌ 任务 $num 验证失败，需要修复"
        go build ./... 2>&1 | tee -a "$LOG_FILE" || true
    fi
}

handle_state_change() {
    local status num
    status=$(get_status)
    num=$(get_task_num)

    [ -z "$status" ] || [ -z "$num" ] && return

    case "$status" in
        *"已完成"*|*"completed"*)
            # 防重复：检查 .bridge-state.json 里是否已经处理过这个任务的完成
            local json_status
            json_status=$(jq -r '.status' "$STATE_FILE" 2>/dev/null || echo "")
            if [[ "$json_status" == *"已完成"* ]]; then
                return  # 已经处理过，跳过
            fi

            log "检测到任务 $num 完成，开始验证"
            # 先更新 JSON 状态防止重复触发
            jq --arg ts "$(date -Iseconds)" '.status="✅ 已完成" | .updated_at=$ts' \
                "$STATE_FILE" > "${STATE_FILE}.tmp" && mv "${STATE_FILE}.tmp" "$STATE_FILE"
            verify_and_advance "$num"
            ;;
        *"阻塞"*|*"blocked"*)
            log "⚠️ 任务 $num 阻塞，等待人工介入"
            ;;
    esac
}

# ===== 启动 =====
log "ctl_device bridge monitor v2 启动"
log "监听文件: $BRIDGE_FILE"

# 启动时检查状态
current_status=$(get_status)
current_num=$(get_task_num)
log "当前: 任务$current_num = [$current_status]"

# 如果已完成但还没推进（比如上次 monitor 崩溃），立刻处理
if [[ "$current_status" == *"已完成"* ]]; then
    json_status=$(jq -r '.status' "$STATE_FILE" 2>/dev/null || echo "")
    if [[ "$json_status" != *"已完成"* ]]; then
        log "发现未处理的完成状态，立即推进..."
        handle_state_change
    fi
fi

# 如果是待执行且 task.txt 为空，重新派发
if [[ "$current_status" == *"待执行"* ]] && [ ! -s "$BRIDGE_TASK_FILE" ]; then
    log "重新派发任务 $current_num 给 Trae..."
    dispatch_task "$current_num"
fi

# 事件驱动：inotifywait 监听 bridge_trae.md
watch_bridge() {
    inotifywait -q -m "$BRIDGE_FILE" \
        -e close_write -e moved_to \
        --format "%e" 2>/dev/null |
    while read -r _event; do
        # 防抖：用文件存 last_event 时间戳（跨子进程共享）
        local now last diff
        now=$(date +%s%3N)
        last=$(cat "$LAST_EVENT_FILE" 2>/dev/null || echo 0)
        diff=$(( now - last ))
        if [ "$diff" -lt 2000 ]; then
            continue
        fi
        echo "$now" > "$LAST_EVENT_FILE"
        sleep 1  # 等文件写完
        handle_state_change
    done
}

# 看门狗：每5分钟强制检查一次（防 inotify 丢事件）
watchdog() {
    while true; do
        sleep 300
        handle_state_change
    done
}

watch_bridge &
WATCH_PID=$!
watchdog &
DOG_PID=$!

trap "kill $WATCH_PID $DOG_PID 2>/dev/null; log 'monitor 退出'" EXIT INT TERM

log "监控就绪（inotifywait 事件驱动 + 5min 看门狗）"
wait
