package hooks

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Installer handles the installation and removal of CWS hooks
type Installer struct {
	claudeDir    string
	settingsPath string
	backupPath   string
	hooksDir     string
	scriptPath   string
	port         int
}

// NewInstaller creates a new Installer
func NewInstaller(port int) *Installer {
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	return &Installer{
		claudeDir:    claudeDir,
		settingsPath: filepath.Join(claudeDir, "settings.json"),
		backupPath:   filepath.Join(claudeDir, "settings.json.cws-backup"),
		hooksDir:     filepath.Join(claudeDir, "hooks"),
		scriptPath:   filepath.Join(claudeDir, "hooks", "cws-notify.sh"),
		port:         port,
	}
}

// Install installs the CWS hooks configuration
func (i *Installer) Install(opts InstallOptions) error {
	// 1. Check prerequisites
	if err := i.checkPrerequisites(); err != nil {
		return err
	}

	// 2. Check port availability
	if err := checkPortAvailable(i.port); err != nil {
		return fmt.Errorf("cannot install hooks: %w\nMake sure the CWS server is not running, or use a different port with --port", err)
	}

	// 3. Load existing settings
	settings, err := i.loadSettings()
	if err != nil {
		return err
	}

	// 4. Check for existing CWS configuration
	if HasCWSHooks(settings) && !opts.Force {
		return fmt.Errorf("CWS hooks already installed. Use --force to overwrite")
	}

	// 5. Remove existing CWS hooks if force mode
	if opts.Force && HasCWSHooks(settings) {
		settings = RemoveCWSHooks(settings)
	}

	// 6. Create backup
	if err := i.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// 7. Create hook script
	if err := i.createHookScript(); err != nil {
		i.restoreFromBackup()
		return fmt.Errorf("failed to create hook script: %w (restored from backup)", err)
	}

	// 8. Merge CWS hooks into settings
	settings = MergeCWSHooks(settings, i.scriptPath)

	// 9. Save settings
	if err := i.saveSettings(settings); err != nil {
		i.restoreFromBackup()
		return fmt.Errorf("failed to save settings: %w (restored from backup)", err)
	}

	// 10. Verify installation
	if err := i.verifyInstallation(); err != nil {
		i.restoreFromBackup()
		return fmt.Errorf("verification failed: %w (restored from backup)", err)
	}

	return nil
}

// Remove removes the CWS hooks configuration
func (i *Installer) Remove(opts InstallOptions) error {
	// 1. Load existing settings
	settings, err := i.loadSettings()
	if err != nil {
		return err
	}

	// 2. Check if CWS hooks are installed
	if !HasCWSHooks(settings) {
		return fmt.Errorf("CWS hooks are not installed")
	}

	// 3. Remove CWS hooks from settings
	settings = RemoveCWSHooks(settings)

	// 4. Save settings
	if err := i.saveSettings(settings); err != nil {
		return fmt.Errorf("failed to save settings: %w", err)
	}

	// 5. Remove hook script (unless --keep-script)
	if !opts.KeepScript {
		if err := i.removeHookScript(); err != nil {
			// Non-fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to remove hook script: %v\n", err)
		}
	}

	// 6. Remove backup
	os.Remove(i.backupPath)

	return nil
}

// Check checks the current configuration status
func (i *Installer) Check() (*CheckResult, error) {
	result := &CheckResult{
		SettingsPath:   i.settingsPath,
		ScriptPath:     i.scriptPath,
		DaemonEndpoint: fmt.Sprintf("http://127.0.0.1:%d/api/hooks", i.port),
	}

	// Check settings
	settings, err := i.loadSettings()
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}

	// Check for CWS hooks
	result.Installed = HasCWSHooks(settings)

	// Check configured events
	for _, event := range CWSHookEvents {
		if hasCWSHookForEvent(settings, event) {
			result.ConfiguredEvents = append(result.ConfiguredEvents, event)
		} else {
			result.MissingEvents = append(result.MissingEvents, event)
		}
	}

	// Check hook script
	if info, err := os.Stat(i.scriptPath); err == nil {
		result.ScriptExists = true
		result.ScriptExecutable = info.Mode()&0111 != 0
	}

	return result, nil
}

func (i *Installer) checkPrerequisites() error {
	// Check if Claude directory exists
	if _, err := os.Stat(i.claudeDir); os.IsNotExist(err) {
		return fmt.Errorf("Claude Code not installed: %s does not exist", i.claudeDir)
	}
	return nil
}

// checkPortAvailable checks if the specified port is available for use
func checkPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("port %d is already in use", port)
	}
	ln.Close()
	return nil
}

func (i *Installer) loadSettings() (map[string]interface{}, error) {
	data, err := os.ReadFile(i.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty settings if file doesn't exist
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("invalid settings.json: %w", err)
	}

	return settings, nil
}

func (i *Installer) saveSettings(settings map[string]interface{}) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(i.settingsPath, data, 0644)
}

func (i *Installer) createBackup() error {
	// Only backup if settings file exists
	if _, err := os.Stat(i.settingsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(i.settingsPath)
	if err != nil {
		return err
	}

	return os.WriteFile(i.backupPath, data, 0644)
}

func (i *Installer) restoreFromBackup() error {
	if _, err := os.Stat(i.backupPath); os.IsNotExist(err) {
		return nil // No backup to restore
	}

	data, err := os.ReadFile(i.backupPath)
	if err != nil {
		return err
	}

	return os.WriteFile(i.settingsPath, data, 0644)
}

func (i *Installer) createHookScript() error {
	// Create hooks directory
	if err := os.MkdirAll(i.hooksDir, 0755); err != nil {
		return err
	}

	// Generate script content
	script := GenerateHookScript(i.port)

	// Write script
	if err := os.WriteFile(i.scriptPath, []byte(script), 0755); err != nil {
		return err
	}

	return nil
}

func (i *Installer) removeHookScript() error {
	// Remove script
	if err := os.Remove(i.scriptPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Try to remove hooks directory if empty
	entries, err := os.ReadDir(i.hooksDir)
	if err == nil && len(entries) == 0 {
		os.Remove(i.hooksDir)
	}

	return nil
}

func (i *Installer) verifyInstallation() error {
	// Reload and verify settings
	settings, err := i.loadSettings()
	if err != nil {
		return err
	}

	if !HasCWSHooks(settings) {
		return fmt.Errorf("CWS hooks not found in settings after installation")
	}

	// Verify script exists and is executable
	info, err := os.Stat(i.scriptPath)
	if err != nil {
		return fmt.Errorf("hook script not found: %w", err)
	}

	if info.Mode()&0111 == 0 {
		return fmt.Errorf("hook script is not executable")
	}

	return nil
}
