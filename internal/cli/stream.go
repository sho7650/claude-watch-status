package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sho7650/claude-watch-status/internal/notifier"
	"github.com/sho7650/claude-watch-status/internal/state"
	"github.com/sho7650/claude-watch-status/internal/watcher"
)

// StreamMode runs the CLI in stream mode
type StreamMode struct {
	projectsDir string
	notifier    *notifier.Notifier
	manager     *state.Manager
	notified    map[string]bool // Track notified files to prevent duplicates
}

// NewStreamMode creates a new StreamMode
func NewStreamMode(projectsDir string) *StreamMode {
	return &StreamMode{
		projectsDir: projectsDir,
		notifier:    notifier.New(),
		manager:     state.NewManager(),
		notified:    make(map[string]bool),
	}
}

// Run starts the stream mode
func (s *StreamMode) Run() error {
	fmt.Println("Watching Claude Code activity... (Ctrl+C to stop)")
	fmt.Println("---")

	w, err := watcher.New(s.projectsDir)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := w.Start(); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}
	defer w.Stop()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start idle detection goroutine
	idleTicker := time.NewTicker(5 * time.Second)
	defer idleTicker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println()
			fmt.Println("Stopped.")
			return nil

		case event := <-w.Events():
			s.handleEvent(event)

		case err := <-w.Errors():
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		case <-idleTicker.C:
			s.checkIdleProjects()
		}
	}
}

func (s *StreamMode) handleEvent(event watcher.Event) {
	status, err := s.manager.Update(event.ProjectName, event.SessionID, event.Path)
	if err != nil || status == nil {
		return
	}

	s.printStatus(status)
}

func (s *StreamMode) printStatus(status *state.ProjectStatus) {
	ts := status.UpdatedAt.Format("15:04:05")
	// Format: icon [timestamp] project     state
	fmt.Printf("%s \033[90m[%s]\033[0m %-15s \033[36m%s\033[0m\n",
		status.Icon, ts, status.Name, status.State)
}

func (s *StreamMode) checkIdleProjects() {
	events := s.manager.CheckIdleProjects(5 * time.Second)

	for _, event := range events {
		// Create a unique key for this idle event
		key := fmt.Sprintf("%s:%s:%s", event.Project.FilePath, event.Project.FileTime, event.Type)
		if s.notified[key] {
			continue
		}
		s.notified[key] = true

		// Update the manager's state
		s.manager.MarkIdle(event.Project.Name, event.Project.Icon, event.Project.State, event.Project.IsEstimated)

		// Print the status
		s.printStatus(&event.Project)

		// Send notification
		switch event.Type {
		case "idle_approval":
			s.notifier.NotifyWaitingApproval(event.Project.Name)
		case "idle_completed":
			s.notifier.NotifyCompleted(event.Project.Name)
		}
	}
}
