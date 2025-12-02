function claude-watch-status --description "Watch Claude Code with accurate state detection"
    argparse 'd/dashboard' -- $argv
    or return 1

    set -l projects_dir ~/.claude/projects

    # Dependency checks
    if not command -q fswatch
        echo "fswatch not found. Install with: brew install fswatch"
        return 1
    end

    if not command -q jq
        echo "jq not found. Install with: brew install jq"
        return 1
    end

    if set -q _flag_dashboard
        _claude_watch_dashboard $projects_dir
    else
        _claude_watch_stream $projects_dir
    end
end

# Dashboard mode: show latest status per project
function _claude_watch_dashboard
    set -l projects_dir $argv[1]
    set -l tmp_file /tmp/cws_dashboard_updates.(fish --version | string match -r '\d+' | head -1)_$fish_pid

    # Kill any lingering background processes from previous mode
    pkill -f "fish.*while true.*sleep 5" 2>/dev/null

    # Project state tracking (use temp file for cross-process communication)
    set -l project_names
    set -l project_states

    # Initialize display (header only, no initial scan)
    clear
    echo "Claude Code Status (Ctrl+C to stop)"
    echo "────────────────────────────────────────"

    # Background idle/approval detection
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

                # 5+ seconds idle - check for waiting approval
                if test \$idle -ge 5 -a \$idle -lt 300
                    if not contains \"\$file_key\" \$notified
                        set -l last_entry (tail -1 \"\$latest\" 2>/dev/null)
                        set -l last_type (echo \"\$last_entry\" | jq -r '.type // empty' 2>/dev/null)
                        set -l stop_reason (echo \"\$last_entry\" | jq -r '.message.stop_reason // \"null\"' 2>/dev/null)
                        set -l content_type (echo \"\$last_entry\" | jq -r '.message.content[0].type // empty' 2>/dev/null)

                        if test \"\$last_type\" = \"assistant\"
                            if test \"\$stop_reason\" = \"null\" -a \"\$content_type\" = \"tool_use\"; or test \"\$stop_reason\" = \"tool_use\"
                                echo \"\$project|⏸️ |waiting approval|\"(date +%H:%M:%S) >> $tmp_file
                                set -a notified \"\$file_key\"

                                if command -q terminal-notifier
                                    terminal-notifier -title 'Claude Code' -message \"\$project: waiting approval\" -sound Glass 2>/dev/null &
                                end
                            else if test \"\$stop_reason\" = \"null\" -a \"\$content_type\" = \"text\"
                                echo \"\$project|✅|completed|\"(date +%H:%M:%S) >> $tmp_file
                                set -a notified \"\$file_key\"

                                if command -q terminal-notifier
                                    terminal-notifier -title 'Claude Code' -message \"\$project: completed\" -sound Glass 2>/dev/null &
                                end
                            end
                        end
                    end
                end
            end
        end
    " &
    set -l bg_pid $last_pid

    # Cleanup on exit
    function _dashboard_cleanup --on-signal INT --on-signal TERM
        kill $bg_pid 2>/dev/null
        rm -f $tmp_file
        echo ""
        echo "Stopped."
        exit 0
    end

    # Watch for changes
    fswatch -r $projects_dir --include '\.jsonl$' | while read -l file
        # Check for idle detection updates first
        if test -f $tmp_file
            for line in (cat $tmp_file)
                set -l parts (string split "|" $line)
                set -l proj $parts[1]
                set -l icon $parts[2]
                set -l text $parts[3]
                set -l idle_ts $parts[4]

                # Find or add project
                set -l found false
                for i in (seq (count $project_names))
                    if test "$project_names[$i]" = "$proj"
                        set project_states[$i] "$icon|$text|$idle_ts"
                        set found true
                        break
                    end
                end

                if test "$found" = "false"
                    set -a project_names $proj
                    set -a project_states "$icon|$text|$idle_ts"
                end
            end
            rm -f $tmp_file
            _redraw_dashboard $project_names $project_states
        end

        set -l project (basename (dirname $file) | sed 's/.*-//')
        set -l ts (date +%H:%M:%S)
        set -l last_entry (tail -1 $file 2>/dev/null)
        set -l state_info (_parse_state "$last_entry")

        test "$state_info" = "SKIP" && continue

        # Find or add project
        set -l idx 0
        set -l found false
        for i in (seq (count $project_names))
            if test "$project_names[$i]" = "$project"
                set idx $i
                set found true
                break
            end
        end

        if test "$found" = "false"
            # New project - add to list
            set -a project_names $project
            set -a project_states "$state_info|$ts"
            set idx (count $project_names)
        else
            # Update existing
            set project_states[$idx] "$state_info|$ts"
        end

        # Redraw dashboard
        _redraw_dashboard $project_names $project_states
    end

    kill $bg_pid 2>/dev/null
    rm -f $tmp_file
end

function _redraw_dashboard
    set -l names $argv[1..(math (count $argv) / 2)]
    set -l states $argv[(math (count $argv) / 2 + 1)..-1]

    # Move cursor to line 3 (after header)
    printf "\033[3;1H"

    for i in (seq (count $names))
        set -l project $names[$i]
        set -l state_data (string split "|" $states[$i])
        set -l icon $state_data[1]
        set -l state_text $state_data[2]
        set -l ts $state_data[3]

        printf "[%-12s] %s \033[90m[%s]\033[0m %-20s\033[K\n" $project $icon $ts $state_text
    end

    # Clear any remaining lines
    printf "\033[J"
end

function _parse_state
    set -l entry $argv[1]
    set -l entry_type (echo $entry | jq -r '.type // empty' 2>/dev/null)

    switch "$entry_type"
        case "queue-operation" "summary"
            echo "SKIP"
            return

        case "user"
            set -l content_type (echo $entry | jq -r '.message.content[0].type // empty' 2>/dev/null)
            set -l content_is_string (echo $entry | jq -r 'if .message.content | type == "string" then "yes" else "no" end' 2>/dev/null)

            if test "$content_is_string" = "yes"
                echo "👤|user input"
            else if test "$content_type" = "tool_result"
                echo "⏳|processing"
            else
                echo "👤|user input"
            end

        case "assistant"
            set -l stop_reason (echo $entry | jq -r '.message.stop_reason // "null"' 2>/dev/null)
            set -l content_type (echo $entry | jq -r '.message.content[0].type // empty' 2>/dev/null)

            switch "$stop_reason"
                case "null"
                    if test "$content_type" = "tool_use"
                        echo "🔧|calling tool"
                    else
                        echo "🤔|thinking"
                    end

                case "tool_use"
                    set -l tool_name (echo $entry | jq -r '[.message.content[] | select(.type == "tool_use") | .name] | last' 2>/dev/null)
                    echo "🔧|running: $tool_name"

                case "end_turn"
                    echo "✅|completed"

                case "max_tokens"
                    echo "⚠️|max tokens"

                case '*'
                    echo "🤔|responding"
            end

        case '*'
            echo "SKIP"
    end
end

# Stream mode: original behavior
function _claude_watch_stream
    set -l projects_dir $argv[1]

    echo "Watching Claude Code activity... (Ctrl+C to stop)"
    echo "---"

    # Background idle/approval detection
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

                # 5+ seconds idle
                if test \$idle -ge 5 -a \$idle -lt 300
                    if not contains \"\$file_key\" \$notified
                        set -l last_entry (tail -1 \"\$latest\" 2>/dev/null)
                        set -l last_type (echo \"\$last_entry\" | jq -r '.type // empty' 2>/dev/null)
                        set -l stop_reason (echo \"\$last_entry\" | jq -r '.message.stop_reason // \"null\"' 2>/dev/null)
                        set -l content_type (echo \"\$last_entry\" | jq -r '.message.content[0].type // empty' 2>/dev/null)

                        if test \"\$last_type\" = \"assistant\"
                            set -l msg \"\"
                            set -l icon \"\"

                            if test \"\$stop_reason\" = \"null\" -a \"\$content_type\" = \"tool_use\"
                                set icon \"⏸️ \"
                                set msg \"waiting approval\"
                            else if test \"\$stop_reason\" = \"tool_use\"
                                set icon \"⏸️ \"
                                set msg \"waiting approval\"
                            else if test \"\$stop_reason\" = \"null\" -a \"\$content_type\" = \"text\"
                                set icon \"✅\"
                                set msg \"completed\"
                            end

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

    trap "kill $bg_pid 2>/dev/null; echo ''; echo 'Stopped.'; exit" INT TERM

    # Main monitoring loop
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
