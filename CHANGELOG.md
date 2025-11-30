# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
