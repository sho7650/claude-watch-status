package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sho7650/claude-watch-status/internal/cli"
	"github.com/sho7650/claude-watch-status/internal/config"
	"github.com/sho7650/claude-watch-status/internal/hooks"
	"github.com/sho7650/claude-watch-status/internal/server"
	"github.com/sho7650/claude-watch-status/internal/state"
	"github.com/sho7650/claude-watch-status/internal/watcher"
	"github.com/spf13/cobra"
)

var (
	version       = "0.2.0"
	dashboardMode bool
	serverPort    int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "claude-watch-status",
		Short: "Real-time status monitor for Claude Code sessions",
		Long: `claude-watch-status monitors Claude Code activity in real-time by watching
the JSONL session logs. It provides visual feedback on what Claude is doing
across multiple projects simultaneously.`,
		RunE: runWatch,
	}

	rootCmd.Flags().BoolVarP(&dashboardMode, "dashboard", "d", false, "Show dashboard view (latest status per project)")

	// Serve subcommand
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Web UI server",
		Long:  `Start the HTTP server with Web UI and API endpoints for real-time status monitoring.`,
		RunE:  runServe,
	}
	serveCmd.Flags().IntVarP(&serverPort, "port", "p", 10087, "Server port")
	rootCmd.AddCommand(serveCmd)

	// Init subcommand
	var initPort int
	var initForce, initYes, initCheck, initRemove, initKeepScript bool

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Install Claude Code hooks for real-time status detection",
		Long: `Install hooks into ~/.claude/settings.json to enable
real-time status detection without polling delays.

This command:
  - Creates a backup of your current settings
  - Adds CWS hooks to your Claude Code configuration
  - Creates the hook notification script

Existing hooks and settings are preserved.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(initPort, initForce, initYes, initCheck, initRemove, initKeepScript)
		},
	}
	initCmd.Flags().IntVarP(&initPort, "port", "p", 10087, "Daemon port")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing CWS configuration")
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "Skip confirmation prompts")
	initCmd.Flags().BoolVar(&initCheck, "check", false, "Check current configuration status")
	initCmd.Flags().BoolVar(&initRemove, "remove", false, "Remove CWS hooks configuration")
	initCmd.Flags().BoolVar(&initKeepScript, "keep-script", false, "Keep hook script when removing")
	rootCmd.AddCommand(initCmd)

	// Version subcommand
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("claude-watch-status %s\n", version)
		},
	}
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runWatch(cmd *cobra.Command, args []string) error {
	projectsDir := config.GetProjectsDir()

	// Check if projects directory exists
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return fmt.Errorf("projects directory not found: %s\nMake sure Claude Code is installed and has been used at least once", projectsDir)
	}

	if dashboardMode {
		dashboard := cli.NewDashboardMode(projectsDir)
		return dashboard.Run()
	}

	stream := cli.NewStreamMode(projectsDir)
	return stream.Run()
}

func runServe(cmd *cobra.Command, args []string) error {
	projectsDir := config.GetProjectsDir()

	// Check if projects directory exists
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return fmt.Errorf("projects directory not found: %s\nMake sure Claude Code is installed and has been used at least once", projectsDir)
	}

	// Create state manager
	manager := state.NewManager()

	// Create and start watcher
	w, err := watcher.New(projectsDir)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := w.Start(); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}
	defer w.Stop()

	// Process watcher events in background
	go func() {
		for event := range w.Events() {
			manager.Update(event.ProjectName, event.SessionID, event.Path)
		}
	}()

	// Create and start server
	srv := server.New(serverPort, manager)
	return srv.Start()
}

func runInit(port int, force, yes, check, remove, keepScript bool) error {
	installer := hooks.NewInstaller(port)

	// Check mode
	if check {
		return runInitCheck(installer)
	}

	// Remove mode
	if remove {
		return runInitRemove(installer, yes, keepScript)
	}

	// Install mode
	return runInitInstall(installer, force, yes)
}

func runInitCheck(installer *hooks.Installer) error {
	result, err := installer.Check()
	if err != nil {
		return err
	}

	fmt.Println("Claude Watch Status - Hooks Configuration Check")
	fmt.Println()
	fmt.Printf("Settings file: %s\n", result.SettingsPath)

	if result.Installed {
		fmt.Println("Status: ✅ Installed")
	} else {
		fmt.Println("Status: ❌ Not installed")
	}

	fmt.Println()
	fmt.Println("Configured events:")
	for _, event := range result.ConfiguredEvents {
		fmt.Printf("  ✅ %s\n", event)
	}
	for _, event := range result.MissingEvents {
		fmt.Printf("  ❌ %s\n", event)
	}

	fmt.Println()
	fmt.Printf("Hook script: %s\n", result.ScriptPath)
	if result.ScriptExists {
		if result.ScriptExecutable {
			fmt.Println("Status: ✅ Exists (executable)")
		} else {
			fmt.Println("Status: ⚠️  Exists (not executable)")
		}
	} else {
		fmt.Println("Status: ❌ Not found")
	}

	fmt.Println()
	fmt.Printf("Daemon endpoint: %s\n", result.DaemonEndpoint)

	return nil
}

func runInitRemove(installer *hooks.Installer, yes, keepScript bool) error {
	// Confirmation prompt
	if !yes {
		fmt.Print("Remove CWS hooks configuration? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	opts := hooks.InstallOptions{
		KeepScript: keepScript,
	}

	if err := installer.Remove(opts); err != nil {
		return err
	}

	fmt.Println("✅ CWS hooks configuration removed successfully.")
	return nil
}

func runInitInstall(installer *hooks.Installer, force, yes bool) error {
	// Check current status
	result, err := installer.Check()
	if err != nil {
		return err
	}

	if result.Installed && !force {
		fmt.Println("CWS hooks are already installed.")
		fmt.Println("Use --force to overwrite, or --check to view current configuration.")
		return nil
	}

	// Confirmation prompt
	if !yes {
		if result.Installed {
			fmt.Print("Overwrite existing CWS hooks configuration? [y/N] ")
		} else {
			fmt.Print("Install CWS hooks configuration? [y/N] ")
		}
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	opts := hooks.InstallOptions{
		Force: force,
	}

	if err := installer.Install(opts); err != nil {
		return err
	}

	fmt.Println("✅ CWS hooks installed successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start the daemon: claude-watch-status serve")
	fmt.Println("  2. Reload hooks in Claude Code (may require restart)")
	fmt.Println()
	fmt.Println("The daemon must be running to receive hook events.")

	return nil
}
