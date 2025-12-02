package state

import (
	"bufio"
	"os"
	"sync"
	"time"

	"github.com/sho7650/claude-watch-status/internal/parser"
)

// ProjectStatus represents the current status of a project
type ProjectStatus struct {
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	State     string    `json:"state"`
	Detail    string    `json:"detail,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	SessionID string    `json:"session_id,omitempty"`
	Source    string    `json:"source"` // "hooks" or "jsonl"
	FilePath  string    `json:"-"`
	FileTime  time.Time `json:"-"`
}

// StatusEvent represents a status change event
type StatusEvent struct {
	Project ProjectStatus
	Type    string // "update", "idle_approval", "idle_completed"
}

// Manager manages the state of all projects
type Manager struct {
	projects  map[string]*ProjectStatus
	mu        sync.RWMutex
	listeners []chan StatusEvent
	listMu    sync.RWMutex
}

// NewManager creates a new state manager
func NewManager() *Manager {
	return &Manager{
		projects:  make(map[string]*ProjectStatus),
		listeners: make([]chan StatusEvent, 0),
	}
}

// Update updates the status for a project from a JSONL file change
func (m *Manager) Update(projectName, sessionID, filePath string) (*ProjectStatus, error) {
	entry, err := readLastEntry(filePath)
	if err != nil {
		return nil, err
	}

	state := parser.ParseState(entry)
	if state.Skip {
		return nil, nil
	}

	// Get file modification time
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	status := &ProjectStatus{
		Name:      projectName,
		Icon:      state.Icon,
		State:     state.Text,
		Detail:    state.ToolName,
		UpdatedAt: time.Now(),
		SessionID: sessionID,
		Source:    "jsonl",
		FilePath:  filePath,
		FileTime:  info.ModTime(),
	}
	m.projects[projectName] = status
	m.mu.Unlock()

	m.notify(StatusEvent{Project: *status, Type: "update"})
	return status, nil
}

// UpdateFromHook updates the status from a hooks event
func (m *Manager) UpdateFromHook(event HookEvent) *ProjectStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := &ProjectStatus{
		Name:      event.ProjectName,
		Icon:      event.Icon,
		State:     event.State,
		Detail:    event.ToolName,
		UpdatedAt: time.Now(),
		SessionID: event.SessionID,
		Source:    "hooks",
	}
	m.projects[event.ProjectName] = status

	m.notify(StatusEvent{Project: *status, Type: "update"})
	return status
}

// HookEvent represents an event from Claude Code hooks
type HookEvent struct {
	SessionID     string `json:"session_id"`
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name,omitempty"`
	CWD           string `json:"cwd"`
	ProjectName   string `json:"-"`
	Icon          string `json:"-"`
	State         string `json:"-"`
}

// Get returns the status for a specific project
func (m *Manager) Get(projectName string) *ProjectStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if status, ok := m.projects[projectName]; ok {
		return status
	}
	return nil
}

// GetAll returns all project statuses
func (m *Manager) GetAll() []ProjectStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]ProjectStatus, 0, len(m.projects))
	for _, status := range m.projects {
		statuses = append(statuses, *status)
	}
	return statuses
}

// Subscribe creates a new subscription channel for status events
func (m *Manager) Subscribe() chan StatusEvent {
	ch := make(chan StatusEvent, 100)
	m.listMu.Lock()
	m.listeners = append(m.listeners, ch)
	m.listMu.Unlock()
	return ch
}

// Unsubscribe removes a subscription channel
func (m *Manager) Unsubscribe(ch chan StatusEvent) {
	m.listMu.Lock()
	defer m.listMu.Unlock()

	for i, listener := range m.listeners {
		if listener == ch {
			m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
			close(ch)
			return
		}
	}
}

func (m *Manager) notify(event StatusEvent) {
	m.listMu.RLock()
	defer m.listMu.RUnlock()

	for _, ch := range m.listeners {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// CheckIdleProjects checks for projects that have been idle and may need notification
func (m *Manager) CheckIdleProjects(idleThreshold time.Duration) []StatusEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []StatusEvent
	now := time.Now()

	for _, status := range m.projects {
		if status.Source == "hooks" {
			// Hooks-based status is already accurate
			continue
		}

		idle := now.Sub(status.FileTime)
		if idle < idleThreshold || idle > 5*time.Minute {
			continue
		}

		// Re-read the file to check current state
		entry, err := readLastEntry(status.FilePath)
		if err != nil {
			continue
		}

		if parser.IsIdleWaitingApproval(entry) {
			events = append(events, StatusEvent{
				Project: ProjectStatus{
					Name:      status.Name,
					Icon:      "⏸️",
					State:     "waiting approval",
					UpdatedAt: now,
					SessionID: status.SessionID,
					Source:    "jsonl",
				},
				Type: "idle_approval",
			})
		} else if parser.IsIdleCompleted(entry) {
			events = append(events, StatusEvent{
				Project: ProjectStatus{
					Name:      status.Name,
					Icon:      "✅",
					State:     "completed",
					UpdatedAt: now,
					SessionID: status.SessionID,
					Source:    "jsonl",
				},
				Type: "idle_completed",
			})
		}
	}

	return events
}

// MarkIdle updates a project's status to an idle state
func (m *Manager) MarkIdle(projectName string, icon, state string) {
	m.mu.Lock()
	if status, ok := m.projects[projectName]; ok {
		status.Icon = icon
		status.State = state
		status.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
}

// readLastEntry reads the last line of a JSONL file and parses it
func readLastEntry(filePath string) (*parser.Entry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lastLine string
	scanner := bufio.NewScanner(file)
	// Use a larger buffer for potentially long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lastLine = line
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return parser.ParseEntry(lastLine)
}
