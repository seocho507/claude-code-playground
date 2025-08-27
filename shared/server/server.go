package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"shared/config"
	"shared/middleware"
)

// Server represents an HTTP server with graceful shutdown capabilities
type Server struct {
	httpServer *http.Server
	config     config.ServerConfig
	router     *gin.Engine
}

// Options contains server configuration options
type Options struct {
	ServiceName   string
	Version       string
	Config        config.ServerConfig
	Router        *gin.Engine
	Middleware    []gin.HandlerFunc
	EnableCORS    bool
	EnableLogging bool
	EnableRecovery bool
	EnableSecurity bool
	CustomSetup   func(*gin.Engine) // Custom router setup function
}

// New creates a new server instance with the provided options
func New(opts Options) *Server {
	// Set Gin mode based on configuration
	if opts.Config.Host == "0.0.0.0" && opts.Config.Port != "8080" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	var router *gin.Engine
	if opts.Router != nil {
		router = opts.Router
	} else {
		router = gin.New()
	}

	// Apply default middleware
	if opts.EnableLogging {
		router.Use(middleware.Logger())
	}
	
	if opts.EnableRecovery {
		router.Use(middleware.Recovery())
	}
	
	if opts.EnableCORS {
		router.Use(middleware.DefaultCORS())
	}
	
	if opts.EnableSecurity {
		router.Use(middleware.SecurityHeaders())
	}

	// Apply custom middleware
	for _, mw := range opts.Middleware {
		router.Use(mw)
	}

	// Add standard routes
	setupStandardRoutes(router, opts.ServiceName, opts.Version)

	// Apply custom setup
	if opts.CustomSetup != nil {
		opts.CustomSetup(router)
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%s", opts.Config.Port),
		Handler:        router,
		ReadTimeout:    opts.Config.ReadTimeout,
		WriteTimeout:   opts.Config.WriteTimeout,
		IdleTimeout:    opts.Config.IdleTimeout,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	return &Server{
		httpServer: httpServer,
		config:     opts.Config,
		router:     router,
	}
}

// setupStandardRoutes adds standard health and monitoring routes
func setupStandardRoutes(router *gin.Engine, serviceName, version string) {
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   serviceName,
			"version":   version,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"uptime":    time.Since(startTime).String(),
		})
	})

	// Readiness probe
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"service":   serviceName,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Liveness probe
	router.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"service":   serviceName,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Version endpoint
	router.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": serviceName,
			"version": version,
			"build_time": buildTime,
			"git_commit": gitCommit,
		})
	})
}

var (
	startTime time.Time
	buildTime string
	gitCommit string
)

func init() {
	startTime = time.Now()
	buildTime = os.Getenv("BUILD_TIME")
	gitCommit = os.Getenv("GIT_COMMIT")
	if buildTime == "" {
		buildTime = "unknown"
	}
	if gitCommit == "" {
		gitCommit = "unknown"
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("🚀 Starting server on %s", s.httpServer.Addr)
	
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	
	return nil
}

// StartWithGracefulShutdown starts the server with graceful shutdown support
func (s *Server) StartWithGracefulShutdown() error {
	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Starting server on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("✅ Server stopped gracefully")
	return nil
}

// Stop stops the server gracefully
func (s *Server) Stop(ctx context.Context) error {
	log.Println("🛑 Stopping server...")
	
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}
	
	log.Println("✅ Server stopped")
	return nil
}

// GetRouter returns the underlying Gin router
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// AddRoute adds a route to the server
func (s *Server) AddRoute(method, path string, handler gin.HandlerFunc) {
	s.router.Handle(method, path, handler)
}

// AddMiddleware adds middleware to the server
func (s *Server) AddMiddleware(middleware gin.HandlerFunc) {
	s.router.Use(middleware)
}

// Builder provides a fluent interface for server configuration
type Builder struct {
	opts Options
}

// NewBuilder creates a new server builder
func NewBuilder(serviceName string) *Builder {
	return &Builder{
		opts: Options{
			ServiceName:    serviceName,
			Version:        "1.0.0",
			EnableCORS:     true,
			EnableLogging:  true,
			EnableRecovery: true,
			EnableSecurity: true,
		},
	}
}

// WithVersion sets the service version
func (b *Builder) WithVersion(version string) *Builder {
	b.opts.Version = version
	return b
}

// WithConfig sets the server configuration
func (b *Builder) WithConfig(config config.ServerConfig) *Builder {
	b.opts.Config = config
	return b
}

// WithRouter sets a custom router
func (b *Builder) WithRouter(router *gin.Engine) *Builder {
	b.opts.Router = router
	return b
}

// WithMiddleware adds middleware
func (b *Builder) WithMiddleware(middleware ...gin.HandlerFunc) *Builder {
	b.opts.Middleware = append(b.opts.Middleware, middleware...)
	return b
}

// WithCORS enables or disables CORS
func (b *Builder) WithCORS(enabled bool) *Builder {
	b.opts.EnableCORS = enabled
	return b
}

// WithLogging enables or disables logging
func (b *Builder) WithLogging(enabled bool) *Builder {
	b.opts.EnableLogging = enabled
	return b
}

// WithRecovery enables or disables recovery middleware
func (b *Builder) WithRecovery(enabled bool) *Builder {
	b.opts.EnableRecovery = enabled
	return b
}

// WithSecurity enables or disables security headers
func (b *Builder) WithSecurity(enabled bool) *Builder {
	b.opts.EnableSecurity = enabled
	return b
}

// WithCustomSetup sets a custom router setup function
func (b *Builder) WithCustomSetup(setup func(*gin.Engine)) *Builder {
	b.opts.CustomSetup = setup
	return b
}

// Build creates the server instance
func (b *Builder) Build() *Server {
	return New(b.opts)
}

// DefaultServer creates a server with sensible defaults
func DefaultServer(serviceName string, config config.ServerConfig) *Server {
	return NewBuilder(serviceName).
		WithConfig(config).
		Build()
}