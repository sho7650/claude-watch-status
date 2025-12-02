package hooks

// CWSMarker is the identifier used to mark CWS-managed hook entries
const CWSMarker = "# cws-managed"

// DefaultPort is the default port for the CWS daemon
const DefaultPort = 10087

// CWSHookEvents are the events that CWS registers hooks for
var CWSHookEvents = []string{
	"PreToolUse",
	"PostToolUse",
	"Stop",
	"SessionStart",
	"SessionEnd",
}

// HookEntry represents a hook entry in settings.json
type HookEntry struct {
	Matcher string       `json:"matcher,omitempty"`
	Hooks   []HookConfig `json:"hooks"`
}

// HookConfig represents a single hook configuration
type HookConfig struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// Settings represents the Claude Code settings.json structure
type Settings struct {
	Hooks   map[string][]HookEntry `json:"hooks,omitempty"`
	Env     map[string]interface{} `json:"env,omitempty"`
	Schema  string                 `json:"$schema,omitempty"`
	Other   map[string]interface{} `json:"-"` // Catch-all for unknown fields
}

// InstallOptions contains options for the init command
type InstallOptions struct {
	Port       int
	Force      bool
	Yes        bool
	KeepScript bool
}

// CheckResult represents the result of a configuration check
type CheckResult struct {
	Installed       bool
	SettingsPath    string
	ScriptPath      string
	ScriptExists    bool
	ScriptExecutable bool
	ConfiguredEvents []string
	MissingEvents    []string
	DaemonEndpoint   string
}
