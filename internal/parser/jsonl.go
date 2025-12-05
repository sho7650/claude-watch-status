package parser

import (
	"encoding/json"
	"strings"
	"time"
)

// ToolTimeout returns the timeout threshold for a specific tool
// Long-running tools like Bash get longer timeouts to reduce false positives
func ToolTimeout(toolName string) time.Duration {
	switch toolName {
	// System tools
	case "Bash", "BashOutput":
		return 1 * time.Minute // Shell commands can run long

	// Agent tools
	case "Task":
		return 3 * time.Minute // Sub-agents can take a while

	// Network tools
	case "WebFetch", "WebSearch":
		return 60 * time.Second // Network operations

	// File I/O tools
	case "Read", "Write", "Edit", "Glob", "Grep":
		return 10 * time.Second // File operations

	// Quick state tools
	case "TodoWrite", "NotebookEdit":
		return 5 * time.Second // Quick operations

	// Planning tools (instant state changes)
	case "ExitPlanMode", "EnterPlanMode":
		return 5 * time.Second // Instant

	// Long-running MCP tools (explicit)
	case "mcp__sequential-thinking__sequentialthinking":
		return 2 * time.Minute // Extended thinking

	default:
		// Handle MCP tool prefixes
		if strings.HasPrefix(toolName, "mcp__playwright__") {
			return 2 * time.Minute // Browser automation
		}
		if strings.HasPrefix(toolName, "mcp__serena__") {
			return 30 * time.Second // Symbol operations
		}
		if strings.HasPrefix(toolName, "mcp__context7__") {
			return 30 * time.Second // Documentation lookup
		}
		if strings.HasPrefix(toolName, "mcp__chrome-devtools__") {
			return 2 * time.Minute // DevTools operations
		}
		if strings.HasPrefix(toolName, "mcp__magic__") {
			return 60 * time.Second // UI component generation
		}
		if strings.HasPrefix(toolName, "mcp__") {
			return 60 * time.Second // Other MCP tools
		}
		return 30 * time.Second // Default fallback
	}
}

// DefaultIdleThreshold is the base threshold for idle detection
const DefaultIdleThreshold = 5 * time.Second

// MaxIdleThreshold prevents indefinite waiting for tool completion
const MaxIdleThreshold = 10 * time.Minute

// EntryType represents the type of JSONL entry
type EntryType string

const (
	EntryTypeUser           EntryType = "user"
	EntryTypeAssistant      EntryType = "assistant"
	EntryTypeSummary        EntryType = "summary"
	EntryTypeQueueOperation EntryType = "queue-operation"
)

// StopReason represents the reason for stopping
type StopReason string

const (
	StopReasonNull      StopReason = "null"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonMaxTokens StopReason = "max_tokens"
)

// ContentType represents the type of content
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
)

// Entry represents a parsed JSONL entry
type Entry struct {
	Type       EntryType `json:"type"`
	Message    *Message  `json:"message,omitempty"`
	UUID       string    `json:"uuid"`
	ParentUUID string    `json:"parentUuid,omitempty"`
	Timestamp  string    `json:"timestamp"`
}

// Message represents the message content
type Message struct {
	StopReason *string   `json:"stop_reason"`
	Content    []Content `json:"content"`
}

// Content represents message content item
type Content struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`          // tool_use id
	Name      string `json:"name,omitempty"`        // for tool_use
	Text      string `json:"text,omitempty"`        // for text
	ToolUseID string `json:"tool_use_id,omitempty"` // for tool_result
}

// State represents the parsed state from a JSONL entry
type State struct {
	Icon        string
	Text        string
	ToolName    string
	Skip        bool
	IsEstimated bool // true if state detection is based on timeout heuristics
}

// ParseEntry parses a single JSONL line into an Entry
func ParseEntry(line string) (*Entry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	var entry Entry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ParseState determines the state from a JSONL entry
func ParseState(entry *Entry) State {
	if entry == nil {
		return State{Skip: true}
	}

	switch entry.Type {
	case EntryTypeQueueOperation, EntryTypeSummary:
		return State{Skip: true}

	case EntryTypeUser:
		return parseUserState(entry)

	case EntryTypeAssistant:
		return parseAssistantState(entry)

	default:
		return State{Skip: true}
	}
}

func parseUserState(entry *Entry) State {
	if entry.Message == nil || len(entry.Message.Content) == 0 {
		return State{Icon: "👤", Text: "user input"}
	}

	contentType := entry.Message.Content[0].Type
	if contentType == string(ContentTypeToolResult) {
		return State{Icon: "⏳", Text: "processing"}
	}
	return State{Icon: "👤", Text: "user input"}
}

func parseAssistantState(entry *Entry) State {
	if entry.Message == nil {
		return State{Icon: "🤔", Text: "responding"}
	}

	stopReason := getStopReason(entry.Message.StopReason)
	contentType := getContentType(entry.Message.Content)

	switch stopReason {
	case StopReasonNull:
		if contentType == ContentTypeToolUse {
			return State{Icon: "🔧", Text: "calling tool"}
		}
		return State{Icon: "🤔", Text: "thinking"}

	case StopReasonToolUse:
		toolName := getLastToolName(entry.Message.Content)
		return State{Icon: "🔧", Text: "running: " + toolName, ToolName: toolName}

	case StopReasonEndTurn:
		return State{Icon: "✅", Text: "completed"}

	case StopReasonMaxTokens:
		return State{Icon: "⚠️", Text: "max tokens"}

	default:
		return State{Icon: "🤔", Text: "responding"}
	}
}

func getStopReason(sr *string) StopReason {
	if sr == nil {
		return StopReasonNull
	}
	return StopReason(*sr)
}

func getContentType(content []Content) ContentType {
	if len(content) == 0 {
		return ContentTypeText
	}
	return ContentType(content[0].Type)
}

func getLastToolName(content []Content) string {
	toolName := "unknown"
	for _, c := range content {
		if c.Type == string(ContentTypeToolUse) && c.Name != "" {
			toolName = c.Name
		}
	}
	return toolName
}


// GetToolUseIDs returns all tool_use IDs from content
func GetToolUseIDs(content []Content) []string {
	var ids []string
	for _, c := range content {
		if c.Type == string(ContentTypeToolUse) && c.ID != "" {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// GetToolResultIDs returns all tool_result IDs from content
func GetToolResultIDs(content []Content) []string {
	var ids []string
	for _, c := range content {
		if c.Type == string(ContentTypeToolResult) && c.ToolUseID != "" {
			ids = append(ids, c.ToolUseID)
		}
	}
	return ids
}

// HasPendingToolUse checks if an assistant entry has tool_use that needs result
func HasPendingToolUse(entry *Entry) bool {
	if entry == nil || entry.Type != EntryTypeAssistant || entry.Message == nil {
		return false
	}
	stopReason := getStopReason(entry.Message.StopReason)
	return stopReason == StopReasonToolUse || 
		(stopReason == StopReasonNull && getContentType(entry.Message.Content) == ContentTypeToolUse)
}

// IsIdleWaitingApproval checks if the entry indicates waiting for approval
func IsIdleWaitingApproval(entry *Entry) bool {
	if entry == nil || entry.Type != EntryTypeAssistant || entry.Message == nil {
		return false
	}

	stopReason := getStopReason(entry.Message.StopReason)
	contentType := getContentType(entry.Message.Content)

	// tool_use with stop_reason null or "tool_use" means waiting approval
	if stopReason == StopReasonNull && contentType == ContentTypeToolUse {
		return true
	}
	if stopReason == StopReasonToolUse {
		return true
	}
	return false
}

// IsIdleCompleted checks if the entry indicates estimated completion
func IsIdleCompleted(entry *Entry) bool {
	if entry == nil || entry.Type != EntryTypeAssistant || entry.Message == nil {
		return false
	}

	stopReason := getStopReason(entry.Message.StopReason)
	contentType := getContentType(entry.Message.Content)

	// text with stop_reason null means likely completed (estimated)
	return stopReason == StopReasonNull && contentType == ContentTypeText
}
