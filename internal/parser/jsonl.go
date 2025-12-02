package parser

import (
	"encoding/json"
	"strings"
)

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
	Type    EntryType `json:"type"`
	Message *Message  `json:"message,omitempty"`
}

// Message represents the message content
type Message struct {
	StopReason *string   `json:"stop_reason"`
	Content    []Content `json:"content"`
}

// Content represents message content item
type Content struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"` // for tool_use
	Text string `json:"text,omitempty"` // for text
}

// State represents the parsed state from a JSONL entry
type State struct {
	Icon     string
	Text     string
	ToolName string
	Skip     bool
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
