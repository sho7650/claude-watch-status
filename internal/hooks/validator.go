package hooks

import (
	"encoding/json"
	"fmt"
	"os"
)

// ValidateSettingsFile validates that a settings file is valid JSON
func ValidateSettingsFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read settings file: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return nil
}

// ValidateHookScript validates that the hook script exists and is executable
func ValidateHookScript(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("hook script does not exist")
		}
		return fmt.Errorf("cannot stat hook script: %w", err)
	}

	if info.Mode()&0111 == 0 {
		return fmt.Errorf("hook script is not executable")
	}

	return nil
}

// ValidateCWSConfiguration validates the complete CWS configuration
func ValidateCWSConfiguration(settingsPath, scriptPath string) []error {
	var errors []error

	// Validate settings file
	if err := ValidateSettingsFile(settingsPath); err != nil {
		errors = append(errors, fmt.Errorf("settings: %w", err))
	} else {
		// Check for CWS hooks in settings
		data, _ := os.ReadFile(settingsPath)
		var settings map[string]interface{}
		json.Unmarshal(data, &settings)

		if !HasCWSHooks(settings) {
			errors = append(errors, fmt.Errorf("settings: CWS hooks not found"))
		}

		// Check for all required events
		for _, event := range CWSHookEvents {
			if !hasCWSHookForEvent(settings, event) {
				errors = append(errors, fmt.Errorf("settings: missing hook for event %s", event))
			}
		}
	}

	// Validate hook script
	if err := ValidateHookScript(scriptPath); err != nil {
		errors = append(errors, fmt.Errorf("script: %w", err))
	}

	return errors
}
