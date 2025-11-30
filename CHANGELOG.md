# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Estimated completion detection** - Detect completed status based on idle time (20+ seconds) with text response, since JSONL format doesn't reliably record `stop_reason: "end_turn"`

### Changed

- Removed unreachable `stop_reason: "end_turn"` checks from idle detection logic
- Updated documentation to clarify completion detection is estimated

## [0.2.0] - 2024-11-30

### Added

- **Zsh support** - New `functions/zsh/claude-watch-status.zsh` wrapper for Zsh users
- **Dashboard mode** (`-d`, `--dashboard`) - Show latest status per project in a compact, updating view
- **Help option** (`-h`, `--help`) - Show usage information (Zsh)
- **jq dependency check** - Now validates jq is installed before running
- **Linux notification support** - Uses `notify-send` on Linux systems
- **Cross-platform stat command** - Automatically detects macOS vs Linux for file modification time

### Changed

- **Project structure** - Reorganized into `functions/fish/` and `functions/zsh/`
- **Fish script location** - Moved from `functions/` to `functions/fish/`
- **SIGTERM handling** - Now properly handles TERM signal in addition to INT

### Technical

- Refactored Fish script with helper functions (`_parse_state`, `_redraw_dashboard`, etc.)

## [0.1.0] - 2024-11-30

### Added

- Initial release
- Real-time monitoring of Claude Code sessions via JSONL file watching
- Multi-project support
- Status detection based on `stop_reason` and content type
- Desktop notifications via terminal-notifier (macOS)
- Idle detection for completion and approval waiting states
- Color-coded output with timestamps

### Status Icons

- 👤 user input - User sent a message
- ⏳ processing - Processing tool results
- 🤔 thinking - Generating response
- 🔧 calling tool / running: X - Tool execution
- ⏸️ waiting approval - Waiting for user approval
- ✅ completed - Response complete
- ⚠️ max tokens - Token limit reached
