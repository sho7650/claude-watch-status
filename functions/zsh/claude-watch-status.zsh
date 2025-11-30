#!/usr/bin/env zsh
# claude-watch-status.zsh - Watch Claude Code with accurate state detection
# Zsh wrapper for claude-watch-status

# Ensure zsh emulation mode
emulate -L zsh

claude-watch-status() {
    local dashboard_mode=false
    local projects_dir="${CLAUDE_PROJECTS_DIR:-$HOME/.claude/projects}"

    # Parse options
    while [[ $# -gt 0 ]]; do
        case $1 in
            (-d|--dashboard)
                dashboard_mode=true
                shift
                ;;
            (-h|--help)
                _cws_show_help
                return 0
                ;;
            (*)
                echo "Unknown option: $1"
                _cws_show_help
                return 1
                ;;
        esac
    done

    # Dependency checks
    if ! command -v fswatch &>/dev/null; then
        echo "fswatch not found. Install with: brew install fswatch"
        return 1
    fi

    if ! command -v jq &>/dev/null; then
        echo "jq not found. Install with: brew install jq"
        return 1
    fi

    if [[ "$dashboard_mode" == true ]]; then
        _cws_dashboard "$projects_dir"
    else
        _cws_stream "$projects_dir"
    fi
}

_cws_show_help() {
    cat << 'EOF'
Usage: claude-watch-status [OPTIONS]

Watch Claude Code activity in real-time.

Options:
  -d, --dashboard    Show dashboard view (latest status per project)
  -h, --help         Show this help message

Examples:
  claude-watch-status           # Stream mode (default)
  claude-watch-status -d        # Dashboard mode
EOF
}

# Parse JSON entry and return state info (icon|state_text)
_cws_parse_state() {
    local entry="$1"
    local entry_type stop_reason content_type content_is_string tool_name

    entry_type=$(echo "$entry" | jq -r '.type // empty' 2>/dev/null)

    case $entry_type in
        (queue-operation|summary)
            echo "SKIP"
            return
            ;;
        (user)
            content_type=$(echo "$entry" | jq -r '.message.content[0].type // empty' 2>/dev/null)
            content_is_string=$(echo "$entry" | jq -r 'if .message.content | type == "string" then "yes" else "no" end' 2>/dev/null)

            if [[ "$content_is_string" == "yes" ]]; then
                echo "👤|user input"
            elif [[ "$content_type" == "tool_result" ]]; then
                echo "⏳|processing"
            else
                echo "👤|user input"
            fi
            ;;
        (assistant)
            stop_reason=$(echo "$entry" | jq -r '.message.stop_reason // "null"' 2>/dev/null)
            content_type=$(echo "$entry" | jq -r '.message.content[0].type // empty' 2>/dev/null)

            case $stop_reason in
                (null)
                    if [[ "$content_type" == "tool_use" ]]; then
                        echo "🔧|calling tool"
                    else
                        echo "🤔|thinking"
                    fi
                    ;;
                (tool_use)
                    tool_name=$(echo "$entry" | jq -r '[.message.content[] | select(.type == "tool_use") | .name] | last' 2>/dev/null)
                    echo "🔧|running: $tool_name"
                    ;;
                (end_turn)
                    echo "✅|completed"
                    ;;
                (max_tokens)
                    echo "⚠️|max tokens"
                    ;;
                (*)
                    echo "🤔|responding"
                    ;;
            esac
            ;;
        (*)
            echo "SKIP"
            ;;
    esac
}

# Dashboard mode: show latest status per project
_cws_dashboard() {
    local projects_dir="$1"
    local bg_pid

    # Initialize display (header only, no initial scan)
    clear
    echo "Claude Code Status (Ctrl+C to stop)"
    echo "────────────────────────────────────────"

    # Use process substitution to avoid subshell variable scope issues
    typeset -A project_states
    typeset -a project_order

    # Background idle/approval detection for dashboard
    (
        typeset -A notified
        typeset -A last_states

        while true; do
            sleep 5
            now=$(date +%s)

            for project_path in "$projects_dir"/*/; do
                [[ -d "$project_path" ]] || continue

                latest=$(ls -t "$project_path"*.jsonl 2>/dev/null | head -1)
                [[ -n "$latest" ]] || continue

                if [[ "$(uname)" == "Darwin" ]]; then
                    mtime=$(stat -f %m "$latest" 2>/dev/null)
                else
                    mtime=$(stat -c %Y "$latest" 2>/dev/null)
                fi
                [[ -n "$mtime" ]] || continue

                idle=$((now - mtime))
                project=$(basename "$project_path" | sed 's/.*-//')
                file_key="$latest:$mtime"

                # 20+ seconds idle - check for waiting approval
                if [[ $idle -ge 20 && $idle -lt 300 ]]; then
                    if [[ -z "${notified[$file_key]}" ]]; then
                        last_entry=$(tail -1 "$latest" 2>/dev/null)
                        last_type=$(echo "$last_entry" | jq -r '.type // empty' 2>/dev/null)
                        stop_reason=$(echo "$last_entry" | jq -r '.message.stop_reason // "null"' 2>/dev/null)
                        content_type=$(echo "$last_entry" | jq -r '.message.content[0].type // empty' 2>/dev/null)

                        if [[ "$last_type" == "assistant" ]]; then
                            if [[ "$stop_reason" == "null" && "$content_type" == "tool_use" ]] || [[ "$stop_reason" == "tool_use" ]]; then
                                # Signal to update dashboard - write to temp file
                                echo "$project|⏸️ |waiting approval|$(date +%H:%M:%S)" >> /tmp/cws_dashboard_updates.$$
                                notified[$file_key]=1

                                if [[ "$(uname)" == "Darwin" ]] && command -v terminal-notifier &>/dev/null; then
                                    terminal-notifier -title 'Claude Code' -message "$project: waiting approval" -sound Glass 2>/dev/null &
                                elif command -v notify-send &>/dev/null; then
                                    notify-send 'Claude Code' "$project: waiting approval" 2>/dev/null &
                                fi
                            elif [[ "$stop_reason" == "null" && "$content_type" == "text" ]]; then
                                # Estimated completed - text response idle for 20+ seconds
                                echo "$project|✅|completed|$(date +%H:%M:%S)" >> /tmp/cws_dashboard_updates.$$
                                notified[$file_key]=1

                                if [[ "$(uname)" == "Darwin" ]] && command -v terminal-notifier &>/dev/null; then
                                    terminal-notifier -title 'Claude Code' -message "$project: completed" -sound Glass 2>/dev/null &
                                elif command -v notify-send &>/dev/null; then
                                    notify-send 'Claude Code' "$project: completed" 2>/dev/null &
                                fi
                            fi
                        fi
                    fi
                fi
            done
        done
    ) &!

    bg_pid=$!

    # Cleanup on exit
    trap "kill $bg_pid 2>/dev/null; rm -f /tmp/cws_dashboard_updates.$$; echo ''; echo 'Stopped.'; exit 0" INT TERM

    # Function to redraw dashboard
    _redraw() {
        printf "\033[3;1H"
        for p in "${project_order[@]}"; do
            local state_data="${project_states[$p]}"
            local icon="${state_data%%|*}"
            local rest="${state_data#*|}"
            local state_text="${rest%%|*}"
            local p_ts="${rest##*|}"
            printf "[%-12s] %s \033[90m[%s]\033[0m %-20s\033[K\n" "$p" "$icon" "$p_ts" "$state_text"
        done
        printf "\033[J"
    }

    # Main loop with timeout to check for idle updates
    while true; do
        # Check for idle detection updates
        if [[ -f /tmp/cws_dashboard_updates.$$ ]]; then
            while IFS='|' read -r proj icon text ts; do
                if [[ -z "${project_states[$proj]}" ]]; then
                    project_order+=("$proj")
                fi
                project_states[$proj]="$icon|$text|$ts"
            done < /tmp/cws_dashboard_updates.$$
            rm -f /tmp/cws_dashboard_updates.$$
            _redraw
        fi

        # Read from fswatch with timeout
        if read -r -t 1 file < <(fswatch -r "$projects_dir" --include '\.jsonl$' -1); then
            local project=$(basename "$(dirname "$file")" | sed 's/.*-//')
            local ts=$(date +%H:%M:%S)
            local last_entry=$(tail -1 "$file" 2>/dev/null)
            local state_info=$(_cws_parse_state "$last_entry")

            [[ "$state_info" == "SKIP" ]] && continue

            if [[ -z "${project_states[$project]}" ]]; then
                project_order+=("$project")
            fi

            project_states[$project]="$state_info|$ts"
            _redraw
        fi
    done

    kill $bg_pid 2>/dev/null
}

# Stream mode: original behavior showing all events
_cws_stream() {
    local projects_dir="$1"
    local bg_pid

    echo "Watching Claude Code activity... (Ctrl+C to stop)"
    echo "---"

    # Background idle/approval detection (run as separate subshell)
    (
        typeset -A notified

        while true; do
            sleep 5
            now=$(date +%s)

            for project_path in "$projects_dir"/*/; do
                [[ -d "$project_path" ]] || continue

                latest=$(ls -t "$project_path"*.jsonl 2>/dev/null | head -1)
                [[ -n "$latest" ]] || continue

                if [[ "$(uname)" == "Darwin" ]]; then
                    mtime=$(stat -f %m "$latest" 2>/dev/null)
                else
                    mtime=$(stat -c %Y "$latest" 2>/dev/null)
                fi
                [[ -n "$mtime" ]] || continue

                idle=$((now - mtime))
                project=$(basename "$project_path" | sed 's/.*-//')
                file_key="$latest:$mtime"

                # 20+ seconds idle
                if [[ $idle -ge 20 && $idle -lt 300 ]]; then
                    if [[ -z "${notified[$file_key]}" ]]; then
                        last_entry=$(tail -1 "$latest" 2>/dev/null)
                        last_type=$(echo "$last_entry" | jq -r '.type // empty' 2>/dev/null)
                        stop_reason=$(echo "$last_entry" | jq -r '.message.stop_reason // "null"' 2>/dev/null)
                        content_type=$(echo "$last_entry" | jq -r '.message.content[0].type // empty' 2>/dev/null)

                        if [[ "$last_type" == "assistant" ]]; then
                            msg=""
                            icon=""

                            if [[ "$stop_reason" == "null" && "$content_type" == "tool_use" ]]; then
                                icon="⏸️ "
                                msg="waiting approval"
                            elif [[ "$stop_reason" == "tool_use" ]]; then
                                icon="⏸️ "
                                msg="waiting approval"
                            elif [[ "$stop_reason" == "null" && "$content_type" == "text" ]]; then
                                icon="✅"
                                msg="completed"
                            fi

                            if [[ -n "$msg" ]]; then
                                printf "%s \033[90m[%s]\033[0m %-15s \033[36m%s\033[0m\n" "$icon" "$(date +%H:%M:%S)" "$project" "$msg"
                                notified[$file_key]=1

                                # Send notification
                                if [[ "$(uname)" == "Darwin" ]] && command -v terminal-notifier &>/dev/null; then
                                    terminal-notifier -title 'Claude Code' -message "$project: $msg" -sound Glass 2>/dev/null &
                                elif command -v notify-send &>/dev/null; then
                                    notify-send 'Claude Code' "$project: $msg" 2>/dev/null &
                                fi
                            fi
                        fi
                    fi
                fi
            done
        done
    ) &!

    bg_pid=$!

    # Cleanup on exit
    trap "kill $bg_pid 2>/dev/null; echo ''; echo 'Stopped.'; exit 0" INT TERM

    # Main monitoring loop
    fswatch -r "$projects_dir" --include '\.jsonl$' | while read -r file; do
        project=$(basename "$(dirname "$file")" | sed 's/.*-//')
        ts=$(date +%H:%M:%S)
        last_entry=$(tail -1 "$file" 2>/dev/null)
        state_info=$(_cws_parse_state "$last_entry")

        [[ "$state_info" == "SKIP" ]] && continue

        icon="${state_info%%|*}"
        state_text="${state_info#*|}"

        printf "%s \033[90m[%s]\033[0m %-15s \033[36m%s\033[0m\n" "$icon" "$ts" "$project" "$state_text"
    done

    kill $bg_pid 2>/dev/null
}
