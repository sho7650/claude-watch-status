package cli

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/sho7650/claude-watch-status/internal/notifier"
	"github.com/sho7650/claude-watch-status/internal/state"
	"github.com/sho7650/claude-watch-status/internal/watcher"
)

// DashboardMode runs the CLI in dashboard mode
type DashboardMode struct {
	projectsDir string
	notifier    *notifier.Notifier
	manager     *state.Manager
	notified    map[string]bool
}

// NewDashboardMode creates a new DashboardMode
func NewDashboardMode(projectsDir string) *DashboardMode {
	return &DashboardMode{
		projectsDir: projectsDir,
		notifier:    notifier.New(),
		manager:     state.NewManager(),
		notified:    make(map[string]bool),
	}
}

// Run starts the dashboard mode
func (d *DashboardMode) Run() error {
	// Clear screen and print header
	fmt.Print("\033[2J\033[H") // Clear screen and move to top-left
	fmt.Println("Claude Code Status (Ctrl+C to stop)")
	fmt.Println("────────────────────────────────────────")

	w, err := watcher.New(d.projectsDir)
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
			d.handleEvent(event)

		case err := <-w.Errors():
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		case <-idleTicker.C:
			d.checkIdleProjects()
		}
	}
}

func (d *DashboardMode) handleEvent(event watcher.Event) {
	status, err := d.manager.Update(event.ProjectName, event.SessionID, event.Path)
	if err != nil || status == nil {
		return
	}

	d.redraw()
}

func (d *DashboardMode) redraw() {
	statuses := d.manager.GetAll()

	// Sort by project name for consistent ordering
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	// Move cursor to line 3 (after header)
	fmt.Print("\033[3;1H")

	for _, status := range statuses {
		ts := status.UpdatedAt.Format("15:04:05")
		// Format: [project     ] icon [timestamp] state
		fmt.Printf("[%-12s] %s \033[90m[%s]\033[0m %-20s\033[K\n",
			status.Name, status.Icon, ts, status.State)
	}

	// Clear any remaining lines
	fmt.Print("\033[J")
}

func (d *DashboardMode) checkIdleProjects() {
	events := d.manager.CheckIdleProjects(5 * time.Second)

	for _, event := range events {
		// Create a unique key for this idle event
		key := fmt.Sprintf("%s:%s:%s", event.Project.FilePath, event.Project.FileTime, event.Type)
		if d.notified[key] {
			continue
		}
		d.notified[key] = true

		// Update the manager's state
		d.manager.MarkIdle(event.Project.Name, event.Project.Icon, event.Project.State)

		// Send notification
		switch event.Type {
		case "idle_approval":
			d.notifier.NotifyWaitingApproval(event.Project.Name)
		case "idle_completed":
			d.notifier.NotifyCompleted(event.Project.Name)
		}
	}

	// Always redraw to update timestamps
	if len(d.manager.GetAll()) > 0 {
		d.redraw()
	}
}
