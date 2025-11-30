function claude-watch-status --description "Watch Claude Code with accurate state detection"
    set -l projects_dir ~/.claude/projects
    
    if not command -q fswatch
        echo "fswatch not found. Install with: brew install fswatch"
        return 1
    end
    
    echo "Watching Claude Code activity... (Ctrl+C to stop)"
    echo "---"
    
    # バックグラウンドでアイドル/承認待ち検出
    fish -c "
        set -l notified
        
        while true
            sleep 5
            set -l now (date +%s)
            
            for project_dir in $projects_dir/*/
                test -d \"\$project_dir\" || continue
                
                set -l latest (ls -t \"\$project_dir\"*.jsonl 2>/dev/null | head -1)
                test -n \"\$latest\" || continue
                
                set -l mtime (stat -f %m \"\$latest\" 2>/dev/null)
                test -n \"\$mtime\" || continue
                
                set -l idle (math \$now - \$mtime)
                set -l project (basename \"\$project_dir\" | sed 's/.*-//')
                set -l file_key \"\$latest:\$mtime\"
                
                # 15秒以上アイドル
                if test \$idle -ge 15 -a \$idle -lt 300
                    if not contains \"\$file_key\" \$notified
                        set -l last_entry (tail -1 \"\$latest\" 2>/dev/null)
                        set -l last_type (echo \"\$last_entry\" | jq -r '.type // empty' 2>/dev/null)
                        set -l stop_reason (echo \"\$last_entry\" | jq -r '.message.stop_reason // \"null\"' 2>/dev/null)
                        set -l content_type (echo \"\$last_entry\" | jq -r '.message.content[0].type // empty' 2>/dev/null)
                        
                        if test \"\$last_type\" = \"assistant\"
                            set -l msg \"\"
                            set -l icon \"\"
                            
                            # end_turn のみを completed とする
                            if test \"\$stop_reason\" = \"end_turn\"
                                set icon \"✅\"
                                set msg \"completed\"
                            else if test \"\$stop_reason\" = \"null\" -a \"\$content_type\" = \"tool_use\"
                                # ツール呼び出し後15秒以上 → 承認待ち
                                set icon \"⏸️ \"
                                set msg \"waiting approval\"
                            else if test \"\$stop_reason\" = \"tool_use\"
                                # tool_use で止まっている → 承認待ちまたは停止
                                set icon \"⏸️ \"
                                set msg \"waiting approval\"
                            end
                            # stop_reason: null + text の場合は通知しない（まだ続く可能性）
                            
                            if test -n \"\$msg\"
                                printf \"%s \\033[90m[%s]\\033[0m %-15s \\033[36m%s\\033[0m\\n\" \"\$icon\" (date +%H:%M:%S) \"\$project\" \"\$msg\"
                                set -a notified \"\$file_key\"
                                
                                if command -q terminal-notifier
                                    terminal-notifier -title 'Claude Code' -message \"\$project: \$msg\" -sound Glass 2>/dev/null &
                                end
                            end
                        end
                    end
                end
            end
        end
    " &
    set -l bg_pid $last_pid
    
    trap "kill $bg_pid 2>/dev/null; echo ''; echo 'Stopped.'; exit" INT
    
    # メイン監視ループ
    fswatch -r $projects_dir --include '\.jsonl$' | while read -l file
        set -l project_dir (basename (dirname $file))
        set -l project (echo $project_dir | sed 's/.*-//')
        
        set -l ts (date +%H:%M:%S)
        
        set -l last_entry (tail -1 $file 2>/dev/null)
        set -l entry_type (echo $last_entry | jq -r '.type // empty' 2>/dev/null)
        
        set -l state_icon "❓"
        set -l state_text "unknown"
        
        switch "$entry_type"
            case "queue-operation" "summary"
                continue
                
            case "user"
                set -l content_type (echo $last_entry | jq -r '.message.content[0].type // empty' 2>/dev/null)
                set -l content_is_string (echo $last_entry | jq -r 'if .message.content | type == "string" then "yes" else "no" end' 2>/dev/null)
                
                if test "$content_is_string" = "yes"
                    set state_icon "👤"
                    set state_text "user input"
                else if test "$content_type" = "tool_result"
                    set state_icon "⏳"
                    set state_text "processing"
                else
                    set state_icon "👤"
                    set state_text "user input"
                end
                
            case "assistant"
                set -l stop_reason (echo $last_entry | jq -r '.message.stop_reason // "null"' 2>/dev/null)
                set -l content_type (echo $last_entry | jq -r '.message.content[0].type // empty' 2>/dev/null)
                
                switch "$stop_reason"
                    case "null"
                        if test "$content_type" = "tool_use"
                            set state_icon "🔧"
                            set state_text "calling tool"
                        else
                            set state_icon "🤔"
                            set state_text "thinking"
                        end
                        
                    case "tool_use"
                        set -l tool_name (echo $last_entry | jq -r '[.message.content[] | select(.type == "tool_use") | .name] | last' 2>/dev/null)
                        set state_icon "🔧"
                        set state_text "running: $tool_name"
                        
                    case "end_turn"
                        # これが唯一の「完了」
                        set state_icon "✅"
                        set state_text "completed"
                        
                    case "max_tokens"
                        set state_icon "⚠️"
                        set state_text "max tokens"
                        
                    case '*'
                        set state_icon "🤔"
                        set state_text "responding"
                end
                
            case '*'
                continue
        end
        
        printf "%s %s[%s]%s %-15s %s%s%s\n" \
            $state_icon \
            (set_color brblack) $ts (set_color normal) \
            $project \
            (set_color cyan) $state_text (set_color normal)
    end
    
    kill $bg_pid 2>/dev/null
end