package notifier

import (
	"runtime"

	"github.com/gen2brain/beeep"
)

// Notifier handles desktop notifications
type Notifier struct {
	enabled bool
}

// New creates a new Notifier
func New() *Notifier {
	return &Notifier{
		enabled: true,
	}
}

// SetEnabled enables or disables notifications
func (n *Notifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

// Notify sends a desktop notification
func (n *Notifier) Notify(title, message string) error {
	if !n.enabled {
		return nil
	}

	// Use beeep for cross-platform notifications
	return beeep.Notify(title, message, "")
}

// NotifyWithSound sends a desktop notification with sound (if supported)
func (n *Notifier) NotifyWithSound(title, message string) error {
	if !n.enabled {
		return nil
	}

	// beeep.Alert includes sound on supported platforms
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return beeep.Alert(title, message, "")
	}
	return beeep.Notify(title, message, "")
}

// NotifyWaitingApproval sends a notification for waiting approval status
func (n *Notifier) NotifyWaitingApproval(projectName string) error {
	return n.NotifyWithSound("Claude Code", projectName+": waiting approval")
}

// NotifyCompleted sends a notification for completed status
func (n *Notifier) NotifyCompleted(projectName string) error {
	return n.NotifyWithSound("Claude Code", projectName+": completed")
}

// NotifySessionStart sends a notification for session start
func (n *Notifier) NotifySessionStart(projectName string) error {
	return n.Notify("Claude Code", projectName+": session started")
}

// NotifySessionEnd sends a notification for session end
func (n *Notifier) NotifySessionEnd(projectName string) error {
	return n.Notify("Claude Code", projectName+": session ended")
}
