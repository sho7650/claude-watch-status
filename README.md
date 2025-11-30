# claude-watch-status

Real-time status monitor for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions.

![Demo](./demo.gif)

## Overview

`claude-watch-status` monitors Claude Code activity in real-time by watching the JSONL session logs. It provides visual feedback on what Claude is doing across multiple projects simultaneously.

**Supported Shells**: Fish, Zsh

## Features

- 🔄 **Real-time monitoring** - Watches Claude Code session files for changes
- 📊 **Multi-project support** - Track multiple Claude Code sessions at once
- 🔔 **Desktop notifications** - Get notified when tasks complete (macOS/Linux)
- 🎨 **Color-coded output** - Easy-to-read status indicators
- ⏸️ **Approval detection** - Detects when Claude is waiting for user approval
- 📋 **Dashboard mode** - Show latest status per project in a compact view

## Status Icons

| Icon | Status           | Description                                          |
| ---- | ---------------- | ---------------------------------------------------- |
| 👤   | user input       | User sent a message                                  |
| ⏳   | processing       | Processing tool results                              |
| 🤔   | thinking         | Generating response                                  |
| 🔧   | calling tool     | Invoking a tool                                      |
| 🔧   | running: X       | Executing specific tool (e.g., Bash, Write)          |
| ⏸️   | waiting approval | Waiting for user to approve tool execution           |
| ✅   | completed        | Response complete, waiting for input (estimated)[^1] |
| ⚠️   | max tokens       | Token limit reached                                  |

[^1]: Estimated based on idle time (20+ seconds with text response). See [Limitations](#limitations).

## Requirements

- [Fish shell](https://fishshell.com/) 3.0+ **or** [Zsh](https://www.zsh.org/)
- [fswatch](https://github.com/emcrisostomo/fswatch) - File change monitor
- [jq](https://jqlang.github.io/jq/) - JSON processor
- [terminal-notifier](https://github.com/julienXX/terminal-notifier) (optional, macOS) - Desktop notifications
- [libnotify](https://gitlab.gnome.org/GNOME/libnotify) (optional, Linux) - Desktop notifications

### Installation of Dependencies

```bash
# macOS (Homebrew)
brew install fswatch jq terminal-notifier

# Ubuntu/Debian
sudo apt install fswatch jq libnotify-bin

# Arch Linux
sudo pacman -S fswatch jq libnotify
```

## Installation

### Fish Shell

#### Option 1: Fisher (recommended)

```fish
fisher install sho7650/claude-watch-status
```

#### Option 2: Manual Installation

```fish
# Create functions directory if it doesn't exist
mkdir -p ~/.config/fish/functions

# Download the function
curl -o ~/.config/fish/functions/claude-watch-status.fish \
  https://raw.githubusercontent.com/sho7650/claude-watch-status/main/functions/fish/claude-watch-status.fish
```

#### Option 3: Copy directly

Copy the contents of `functions/fish/claude-watch-status.fish` to your `~/.config/fish/functions/` directory.

### Zsh

Add the following to your `~/.zshrc`:

```zsh
# Option 1: Source directly (replace with your actual path)
source /path/to/claude-watch-status/functions/zsh/claude-watch-status.zsh

# Option 2: Download and source
curl -o ~/.config/zsh/claude-watch-status.zsh \
  https://raw.githubusercontent.com/sho7650/claude-watch-status/main/functions/zsh/claude-watch-status.zsh
source ~/.config/zsh/claude-watch-status.zsh
```

Then reload your shell:

```bash
source ~/.zshrc
```

## Usage

```bash
# Start monitoring (stream mode - default)
claude-watch-status

# Start monitoring (dashboard mode)
claude-watch-status -d
claude-watch-status --dashboard

# Show help
claude-watch-status -h
claude-watch-status --help

# Stop monitoring
# Press Ctrl+C
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
[another-proj] ✅ [10:17:13] completed
[new-project ] ⏳ [10:20:19] processing
```

## How It Works

Claude Code stores session transcripts as JSONL files in `~/.claude/projects/`. This tool:

1. Uses `fswatch` to monitor these files for changes
2. Parses the latest entry in each session file
3. Determines the current state based on:
   - `type`: "user", "assistant", or "summary"
   - `stop_reason`: "end_turn", "tool_use", or null
   - `content[0].type`: "text" or "tool_use"
4. Displays color-coded status with timestamps
5. Runs a background process to detect idle states (approval waiting, completion)

### JSONL State Detection Logic

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

Idle Detection (20+ seconds):
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

### Adjusting Idle Timeout

The default idle timeout is 20 seconds. To change it, edit the function and modify the idle check condition (look for `idle -ge 20` in Fish or `$idle -ge 20` in Zsh).

### Disabling Notifications

Remove or comment out the notification lines in the function, or simply don't install `terminal-notifier` (macOS) or `libnotify` (Linux).

## Limitations

### Estimated Completion Detection

The JSONL session format does not record `stop_reason: "end_turn"` after streaming completes - all assistant messages remain with `stop_reason: null`. Therefore, completion status is **estimated** based on:

- Last entry is `assistant` type
- `stop_reason` is `null`
- `content[0].type` is `text` (not `tool_use`)
- Idle for 20+ seconds

This means completion detection has a ~20 second delay. For more accurate real-time detection, Claude Code Hooks would be required.

### Single Instance Only

Running multiple instances of `claude-watch-status` simultaneously (e.g., Fish and Zsh, or stream and dashboard mode) is **not recommended**. When multiple `fswatch` processes monitor the same directory, file system events may be distributed inconsistently between them, causing some events to be missed by one or both instances.

If you need to switch between shells or modes, stop the current instance first (Ctrl+C).

## Troubleshooting

### "fswatch not found"

Install fswatch using your package manager (see Requirements).

### "jq not found"

Install jq using your package manager (see Requirements).

### No output appears

1. Make sure Claude Code is running and has active sessions
2. Check that `~/.claude/projects/` exists and contains `.jsonl` files
3. Verify fswatch is working: `fswatch ~/.claude/projects/`

### Notifications not working

- **macOS**: Install `terminal-notifier` and check notification permissions in System Settings
- **Linux**: Install `libnotify` (provides `notify-send` command)

## Project Structure

```
claude-watch-status/
├── functions/
│   ├── fish/
│   │   └── claude-watch-status.fish   # Fish shell implementation
│   └── zsh/
│       └── claude-watch-status.zsh    # Zsh implementation
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
