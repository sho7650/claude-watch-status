package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sho7650/claude-watch-status/internal/state"
)

//go:embed static
var staticFS embed.FS

// Server represents the HTTP server
type Server struct {
	echo    *echo.Echo
	port    int
	manager *state.Manager
}

// New creates a new Server
func New(port int, manager *state.Manager) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	s := &Server{
		echo:    e,
		port:    port,
		manager: manager,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// API routes
	api := s.echo.Group("/api")
	api.GET("/status", s.handleGetStatus)
	api.GET("/status/stream", s.handleSSE)
	api.POST("/hooks", s.handleHooksEvent)

	// Health check
	s.echo.GET("/health", s.handleHealth)

	// Static files (Web UI)
	staticContent, err := fs.Sub(staticFS, "static")
	if err == nil {
		s.echo.GET("/*", echo.WrapHandler(http.FileServer(http.FS(staticContent))))
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting server on http://127.0.0.1%s\n", addr)
	return s.echo.Start(addr)
}

// Stop gracefully stops the server
func (s *Server) Stop() error {
	return s.echo.Close()
}

// GetManager returns the state manager
func (s *Server) GetManager() *state.Manager {
	return s.manager
}
