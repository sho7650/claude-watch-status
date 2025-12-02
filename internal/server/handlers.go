package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sho7650/claude-watch-status/internal/state"
)

// StatusResponse represents the API response for status
type StatusResponse struct {
	Projects []state.ProjectStatus `json:"projects"`
}

// handleGetStatus returns the current status of all projects
func (s *Server) handleGetStatus(c echo.Context) error {
	statuses := s.manager.GetAll()
	return c.JSON(http.StatusOK, StatusResponse{Projects: statuses})
}

// handleHealth returns server health status
func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// handleSSE handles Server-Sent Events for real-time updates
func (s *Server) handleSSE(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")

	// Subscribe to status events
	eventCh := s.manager.Subscribe()
	defer s.manager.Unsubscribe(eventCh)

	// Send initial state
	statuses := s.manager.GetAll()
	initialData, _ := json.Marshal(StatusResponse{Projects: statuses})
	fmt.Fprintf(c.Response(), "event: init\ndata: %s\n\n", initialData)
	c.Response().Flush()

	// Stream updates
	for {
		select {
		case <-c.Request().Context().Done():
			return nil

		case event, ok := <-eventCh:
			if !ok {
				return nil
			}

			data, err := json.Marshal(event.Project)
			if err != nil {
				continue
			}

			fmt.Fprintf(c.Response(), "event: update\ndata: %s\n\n", data)
			c.Response().Flush()
		}
	}
}

// HookEventRequest represents the incoming hook event from Claude Code
type HookEventRequest struct {
	SessionID     string                 `json:"session_id"`
	HookEventName string                 `json:"hook_event_name"`
	ToolName      string                 `json:"tool_name,omitempty"`
	ToolInput     map[string]interface{} `json:"tool_input,omitempty"`
	ToolResult    *ToolResult            `json:"tool_result,omitempty"`
	CWD           string                 `json:"cwd"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleHooksEvent handles incoming hook events from Claude Code
func (s *Server) handleHooksEvent(c echo.Context) error {
	var req HookEventRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Extract project name from CWD
	projectName := extractProjectNameFromCWD(req.CWD)

	// Convert hook event to state
	icon, stateText := convertHookEventToState(req.HookEventName, req.ToolName)

	// Update state manager
	event := state.HookEvent{
		SessionID:     req.SessionID,
		HookEventName: req.HookEventName,
		ToolName:      req.ToolName,
		CWD:           req.CWD,
		ProjectName:   projectName,
		Icon:          icon,
		State:         stateText,
	}

	s.manager.UpdateFromHook(event)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// extractProjectNameFromCWD extracts project name from the working directory
func extractProjectNameFromCWD(cwd string) string {
	// Try to extract meaningful project name from path
	// e.g., /Users/user/projects/myproject -> myproject
	base := filepath.Base(cwd)
	if base == "" || base == "." || base == "/" {
		return "unknown"
	}
	return base
}

// convertHookEventToState converts hook event to icon and state text
func convertHookEventToState(hookEvent, toolName string) (icon, stateText string) {
	switch strings.ToLower(hookEvent) {
	case "sessionstart":
		return "👤", "session started"
	case "sessionend":
		return "💤", "session ended"
	case "pretooluse":
		// PreToolUse fires AFTER approval, so tool is now running
		if toolName != "" {
			return "🔧", "running: " + toolName
		}
		return "🔧", "running tool"
	case "posttooluse":
		return "⏳", "processing"
	case "stop":
		return "✅", "completed"
	default:
		return "🔄", hookEvent
	}
}
