package hooks

import (
	"encoding/json"
	"strings"
)

// MergeCWSHooks merges CWS hooks into existing settings
func MergeCWSHooks(settings map[string]interface{}, scriptPath string) map[string]interface{} {
	result := deepCopy(settings)

	// Initialize hooks map if not present
	if result["hooks"] == nil {
		result["hooks"] = make(map[string]interface{})
	}

	hooks := result["hooks"].(map[string]interface{})

	// Add CWS hooks for each event
	for _, event := range CWSHookEvents {
		cwsEntry := createCWSHookEntry(event, scriptPath)

		if hooks[event] == nil {
			// Event doesn't exist - create new array
			hooks[event] = []interface{}{cwsEntry}
		} else {
			// Event exists - append to array
			existingEntries, ok := hooks[event].([]interface{})
			if !ok {
				// Handle case where it might be a different type
				hooks[event] = []interface{}{cwsEntry}
				continue
			}
			hooks[event] = append(existingEntries, cwsEntry)
		}
	}

	return result
}

// RemoveCWSHooks removes all CWS-managed hooks from settings
func RemoveCWSHooks(settings map[string]interface{}) map[string]interface{} {
	result := deepCopy(settings)

	hooks, ok := result["hooks"].(map[string]interface{})
	if !ok {
		return result
	}

	// Process each event
	for event, entries := range hooks {
		entryList, ok := entries.([]interface{})
		if !ok {
			continue
		}

		filtered := make([]interface{}, 0)

		for _, entry := range entryList {
			if !isCWSManagedEntry(entry) {
				filtered = append(filtered, entry)
			}
		}

		if len(filtered) > 0 {
			hooks[event] = filtered
		} else {
			delete(hooks, event)
		}
	}

	// Remove hooks object if empty
	if len(hooks) == 0 {
		delete(result, "hooks")
	}

	return result
}

// HasCWSHooks checks if settings contain any CWS-managed hooks
func HasCWSHooks(settings map[string]interface{}) bool {
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return false
	}

	for _, entries := range hooks {
		entryList, ok := entries.([]interface{})
		if !ok {
			continue
		}

		for _, entry := range entryList {
			if isCWSManagedEntry(entry) {
				return true
			}
		}
	}

	return false
}

// hasCWSHookForEvent checks if a specific event has CWS hooks
func hasCWSHookForEvent(settings map[string]interface{}, event string) bool {
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return false
	}

	entries, ok := hooks[event].([]interface{})
	if !ok {
		return false
	}

	for _, entry := range entries {
		if isCWSManagedEntry(entry) {
			return true
		}
	}

	return false
}

// createCWSHookEntry creates a hook entry for a given event
func createCWSHookEntry(event, scriptPath string) map[string]interface{} {
	hookConfig := map[string]interface{}{
		"type":    "command",
		"command": scriptPath + "  " + CWSMarker,
	}

	entry := map[string]interface{}{
		"hooks": []interface{}{hookConfig},
	}

	// Add matcher for PreToolUse and PostToolUse
	if event == "PreToolUse" || event == "PostToolUse" {
		entry["matcher"] = "*"
	}

	return entry
}

// isCWSManagedEntry checks if a hook entry is managed by CWS
func isCWSManagedEntry(entry interface{}) bool {
	entryMap, ok := entry.(map[string]interface{})
	if !ok {
		return false
	}

	hooksList, ok := entryMap["hooks"].([]interface{})
	if !ok {
		return false
	}

	for _, hook := range hooksList {
		hookMap, ok := hook.(map[string]interface{})
		if !ok {
			continue
		}

		cmd, ok := hookMap["command"].(string)
		if ok && strings.Contains(cmd, CWSMarker) {
			return true
		}
	}

	return false
}

// deepCopy creates a deep copy of a map
func deepCopy(src map[string]interface{}) map[string]interface{} {
	data, _ := json.Marshal(src)
	var dst map[string]interface{}
	json.Unmarshal(data, &dst)
	if dst == nil {
		dst = make(map[string]interface{})
	}
	return dst
}
