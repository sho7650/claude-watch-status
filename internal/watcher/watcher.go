package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Event represents a file change event
type Event struct {
	Path        string
	ProjectName string
	SessionID   string
}

// Watcher watches for JSONL file changes in the projects directory
type Watcher struct {
	fsWatcher   *fsnotify.Watcher
	projectsDir string
	events      chan Event
	errors      chan error
	done        chan struct{}
	mu          sync.RWMutex
	watching    map[string]bool
}

// New creates a new Watcher for the given projects directory
func New(projectsDir string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsWatcher:   fsWatcher,
		projectsDir: projectsDir,
		events:      make(chan Event, 100),
		errors:      make(chan error, 10),
		done:        make(chan struct{}),
		watching:    make(map[string]bool),
	}

	return w, nil
}

// Start begins watching for file changes
func (w *Watcher) Start() error {
	// Initial scan of existing directories
	if err := w.scanDirectories(); err != nil {
		return err
	}

	// Watch the projects directory for new project folders
	if err := w.fsWatcher.Add(w.projectsDir); err != nil {
		return err
	}

	go w.watchLoop()
	return nil
}

// Events returns the channel of file events
func (w *Watcher) Events() <-chan Event {
	return w.events
}

// Errors returns the channel of errors
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	close(w.done)
	return w.fsWatcher.Close()
}

func (w *Watcher) scanDirectories() error {
	entries, err := os.ReadDir(w.projectsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(w.projectsDir, entry.Name())
			if err := w.watchDirectory(dirPath); err != nil {
				w.errors <- err
			}
		}
	}
	return nil
}

func (w *Watcher) watchDirectory(dirPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watching[dirPath] {
		return nil
	}

	if err := w.fsWatcher.Add(dirPath); err != nil {
		return err
	}
	w.watching[dirPath] = true
	return nil
}

func (w *Watcher) watchLoop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.errors <- err
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Handle new directory creation
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			if err := w.watchDirectory(event.Name); err != nil {
				w.errors <- err
			}
			return
		}
	}

	// Only process JSONL file modifications
	if !strings.HasSuffix(event.Name, ".jsonl") {
		return
	}

	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}

	projectName := extractProjectName(event.Name)
	sessionID := extractSessionID(event.Name)

	w.events <- Event{
		Path:        event.Name,
		ProjectName: projectName,
		SessionID:   sessionID,
	}
}

// extractProjectName extracts the project name from the path
// Path format: ~/.claude/projects/{hash}-{projectname}/{session}.jsonl
func extractProjectName(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(dir)

	// Find the last dash and take everything after it
	if idx := strings.LastIndex(base, "-"); idx != -1 {
		return base[idx+1:]
	}
	return base
}

// extractSessionID extracts the session ID from the filename
func extractSessionID(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".jsonl")
}

// GetLatestJSONL returns the most recently modified JSONL file in a directory
func GetLatestJSONL(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	var latest string
	var latestTime int64

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Unix() > latestTime {
			latestTime = info.ModTime().Unix()
			latest = filepath.Join(dirPath, entry.Name())
		}
	}

	return latest, nil
}
