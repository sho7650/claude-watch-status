package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sho7650/claude-watch-status/internal/cli"
	"github.com/sho7650/claude-watch-status/internal/config"
	"github.com/sho7650/claude-watch-status/internal/server"
	"github.com/sho7650/claude-watch-status/internal/state"
	"github.com/sho7650/claude-watch-status/internal/watcher"
)

var (
	version     = "dev"
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
	serveCmd.Flags().IntVarP(&serverPort, "port", "p", 8787, "Server port")
	rootCmd.AddCommand(serveCmd)

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
