# claude-watch-status

Real-time status monitor for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions.

![Demo](./demo.gif)

## Overview

`claude-watch-status` is a Fish shell function that monitors Claude Code activity in real-time by watching the JSONL session logs. It provides visual feedback on what Claude is doing across multiple projects simultaneously.

## Features

- 🔄 **Real-time monitoring** - Watches Claude Code session files for changes
- 📊 **Multi-project support** - Track multiple Claude Code sessions at once
- 🔔 **Desktop notifications** - Get notified when tasks complete (macOS)
- 🎨 **Color-coded output** - Easy-to-read status indicators
- ⏸️ **Approval detection** - Detects when Claude is waiting for user approval

## Status Icons

| Icon | Status           | Description                                 |
| ---- | ---------------- | ------------------------------------------- |
| 👤   | user input       | User sent a message                         |
| ⏳   | processing       | Processing tool results                     |
| 🤔   | thinking         | Generating response                         |
| 🔧   | calling tool     | Invoking a tool                             |
| 🔧   | running: X       | Executing specific tool (e.g., Bash, Write) |
| ⏸️   | waiting approval | Waiting for user to approve tool execution  |
| ✅   | completed        | Response complete, waiting for input        |
| ⚠️   | max tokens       | Token limit reached                         |

## Requirements

- [Fish shell](https://fishshell.com/) 3.0+
- [fswatch](https://github.com/emcrisostomo/fswatch) - File change monitor
- [jq](https://jqlang.github.io/jq/) - JSON processor
- [terminal-notifier](https://github.com/julienXX/terminal-notifier) (optional) - macOS notifications

### Installation of Dependencies

```bash
# macOS (Homebrew)
brew install fish fswatch jq terminal-notifier

# Ubuntu/Debian
sudo apt install fish fswatch jq

# Arch Linux
sudo pacman -S fish fswatch jq
```

## Installation

### Option 1: Fisher (recommended)

```fish
fisher install your-username/claude-watch-status
```

### Option 2: Manual Installation

```fish
# Create functions directory if it doesn't exist
mkdir -p ~/.config/fish/functions

# Download the function
curl -o ~/.config/fish/functions/claude-watch-status.fish \
  https://raw.githubusercontent.com/sho7650/claude-watch-status/main/functions/claude-watch-status.fish
```

### Option 3: Copy directly

Copy the contents of `functions/claude-watch-status.fish` to your `~/.config/fish/functions/` directory.

## Usage

```fish
# Start monitoring
claude-watch-status

# Stop monitoring
# Press Ctrl+C
```

### Example Output

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
  └─ stop_reason: "end_turn"         → ✅ completed
  └─ stop_reason: "max_tokens"       → ⚠️ max tokens

Idle Detection (15+ seconds):
  └─ stop_reason: "end_turn"         → ✅ completed (with notification)
  └─ stop_reason: null + tool_use    → ⏸️ waiting approval
  └─ stop_reason: "tool_use"         → ⏸️ waiting approval
```

## Configuration

### Adjusting Idle Timeout

The default idle timeout is 15 seconds. To change it, edit the function and modify:

```fish
# In the background process
if test \$idle -ge 15 -a \$idle -lt 300
```

### Disabling Notifications

Remove or comment out the `terminal-notifier` lines in the function.

## Troubleshooting

### "fswatch not found"

Install fswatch using your package manager (see Requirements).

### No output appears

1. Make sure Claude Code is running and has active sessions
2. Check that `~/.claude/projects/` exists and contains `.jsonl` files
3. Verify fswatch is working: `fswatch ~/.claude/projects/`

### Notifications not working

- macOS only: Install `terminal-notifier`
- Check notification permissions in System Settings

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
