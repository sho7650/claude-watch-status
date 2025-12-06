# claude-watch-status

Real-time status monitor for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions.

![Demo](./demo.gif)

## Overview

`claude-watch-status` monitors Claude Code activity in real-time by watching the JSONL session logs. It provides visual feedback on what Claude is doing across multiple projects simultaneously.

## Features

- **Real-time monitoring** - Watches Claude Code session files for changes
- **Multi-project support** - Track multiple Claude Code sessions at once
- **Desktop notifications** - Get notified when tasks complete (macOS/Linux)
- **Web UI** - Browser-based dashboard with real-time updates
- **Hooks integration** - Optional Claude Code hooks for faster detection
- **Tool-specific timeouts** - Intelligent detection based on tool type
- **Uncertainty indicators** - Shows when detection is estimated vs confirmed

## Status Icons

| Icon | Status | Description |
|------|--------|-------------|
| 👤 | user input | User sent a message |
| ⏳ | processing | Processing tool results |
| 🤔 | thinking | Generating response |
| 🔧 | calling tool | Invoking a tool |
| 🔧 | running: X | Executing specific tool (e.g., Bash, Write) |
| ⏸️ | waiting approval | Waiting for user to approve tool execution |
| ⏸️❓ | waiting approval | Estimated waiting (tool may still be running) |
| ✅ | completed | Response complete, waiting for input |
| ✅❓ | completed | Estimated completion (based on idle time)[^1] |
| ⚠️ | max tokens | Token limit reached |

[^1]: The ❓ indicator shows when state detection is based on timeout heuristics rather than definitive signals.

## Installation

### Using Go

```bash
go install github.com/sho7650/claude-watch-status/cmd/claude-watch-status@latest
```

### From Source

```bash
git clone https://github.com/sho7650/claude-watch-status.git
cd claude-watch-status
go build -o claude-watch-status ./cmd/claude-watch-status
```

### Using Homebrew (macOS)

```bash
# Coming soon
# brew install sho7650/tap/claude-watch-status
```

## Usage

### CLI Modes

```bash
# Stream mode (default) - shows all events chronologically
claude-watch-status

# Dashboard mode - compact view with latest status per project
claude-watch-status -d
claude-watch-status --dashboard

# Web UI mode - browser-based dashboard
claude-watch-status serve
claude-watch-status serve -p 8080  # custom port

# Show help
claude-watch-status --help

# Show version
claude-watch-status version
```

### Stream Mode (Default)

Shows all events in chronological order:

```
Watching Claude Code activity... (Ctrl+C to stop)
---
👤 [14:23:01] myproject       user input
🔧 [14:23:02] myproject       running: Bash
⏳ [14:23:03] myproject       processing
🤔 [14:23:05] myproject       thinking
✅ [14:23:08] myproject       completed
👤 [14:23:15] another-proj    user input
🔧 [14:23:16] another-proj    calling tool
⏸️  [14:23:32] another-proj    waiting approval
```

### Dashboard Mode (`-d`)

Shows the latest status per project, updating in place:

```
Claude Code Status (Ctrl+C to stop)
────────────────────────────────────────
[myproject   ] 🤔 [10:15:43] thinking
[another-proj] ✅❓ [10:17:13] completed
[new-project ] ⏳ [10:20:19] processing
```

### Web UI Mode (`serve`)

Start the web server and open http://localhost:10087 in your browser:

```bash
claude-watch-status serve
```

Features:
- Real-time updates via Server-Sent Events (SSE)
- Clean, responsive interface
- Works across local network

## Hooks Integration (Optional)

For faster and more accurate detection, install Claude Code hooks:

```bash
# Install hooks
claude-watch-status init

# Check installation status
claude-watch-status init --check

# Remove hooks
claude-watch-status init --remove
```

When hooks are installed:
1. Start the daemon: `claude-watch-status serve`
2. Claude Code will notify the daemon of state changes in real-time
3. No polling delays for tool execution detection

## How It Works

### JSONL Parsing

Claude Code stores session transcripts as JSONL files in `~/.claude/projects/`. This tool:

1. Monitors these files for changes using fsnotify
2. Parses the latest entry in each session file
3. Determines the current state based on:
   - `type`: "user", "assistant", or "summary"
   - `stop_reason`: "end_turn", "tool_use", or null
   - `content[0].type`: "text" or "tool_use"
4. Applies tool-specific timeouts for idle detection
5. Displays status with uncertainty indicators when detection is estimated

### Tool-Specific Timeouts

Different tools have different expected execution times. The system uses intelligent timeouts to reduce false positives:

| Tool Category | Timeout | Examples |
|---------------|---------|----------|
| Quick operations | 5 sec | TodoWrite, ExitPlanMode |
| File I/O | 10 sec | Read, Write, Edit, Glob, Grep |
| System commands | 10 sec | Bash, BashOutput |
| Symbol operations | 30 sec | mcp__serena__* |
| Network | 60 sec | WebFetch, WebSearch |
| Browser automation | 2 min | mcp__playwright__*, mcp__chrome-devtools__* |
| Extended thinking | 2 min | mcp__sequential-thinking__* |
| Sub-agents | 3 min | Task |

### State Detection Logic

```
Entry Type: "user"
  └─ content[0].type: "tool_result" → ⏳ processing
  └─ content[0].type: "text"        → 👤 user input

Entry Type: "assistant"
  └─ stop_reason: null
      └─ content[0].type: "tool_use" → 🔧 calling tool
      └─ content[0].type: "text"     → 🤔 thinking
  └─ stop_reason: "tool_use"         → 🔧 running: [tool_name]
  └─ stop_reason: "max_tokens"       → ⚠️ max tokens

Idle Detection (tool-specific timeout):
  └─ stop_reason: null + tool_use    → ⏸️ waiting approval
  └─ stop_reason: "tool_use"         → ⏸️ waiting approval
  └─ stop_reason: null + text        → ✅ completed (estimated)
```

> **Note**: The JSONL format does not reliably record `stop_reason: "end_turn"` after streaming completes. Completion status is estimated based on idle time with text content.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_PROJECTS_DIR` | `~/.claude/projects` | Directory containing Claude Code session files |

### Server Configuration

The web server runs on port 10087 by default. Use `-p` to specify a different port:

```bash
claude-watch-status serve -p 8080
```

## Limitations

### Estimated Detection

Some states cannot be detected definitively from JSONL:

1. **Completion**: `stop_reason: "end_turn"` is never recorded in JSONL files
2. **Waiting approval vs Running**: Both appear as `stop_reason: "tool_use"`

The ❓ indicator shows when detection is based on timeout heuristics. This is expected behavior, not a bug.

### Single Instance

Running multiple instances simultaneously is not recommended. File system events may be distributed inconsistently between watchers.

## Shell Functions (Legacy)

The original Fish/Zsh shell functions are still available in `functions/` but are no longer maintained. The Go implementation is recommended for all users.

## Project Structure

```
claude-watch-status/
├── cmd/
│   └── claude-watch-status/
│       └── main.go              # CLI entry point
├── internal/
│   ├── cli/                     # Stream and dashboard modes
│   ├── config/                  # Configuration handling
│   ├── hooks/                   # Claude Code hooks integration
│   ├── notifier/                # Desktop notifications
│   ├── parser/                  # JSONL parsing and state detection
│   ├── server/                  # Web UI server
│   ├── state/                   # State management
│   └── watcher/                 # File system watcher
├── functions/                   # Legacy shell functions
│   ├── fish/
│   └── zsh/
├── docs/                        # Additional documentation
├── go.mod
├── go.sum
├── README.md
├── CHANGELOG.md
└── LICENSE
```

## Related Tools

- [claude-code-log](https://github.com/anthropics/claude-code-log) - TUI viewer for Claude Code sessions
- [ccusage](https://github.com/ryoppippi/ccusage) - Token usage analyzer

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Anthropic](https://www.anthropic.com/) for Claude Code
- The Claude Code community for reverse-engineering the JSONL format

---

**Note**: This tool relies on Claude Code's internal JSONL format which is not officially documented and may change between versions.
