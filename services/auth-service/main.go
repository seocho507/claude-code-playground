package main

import (
	"auth-service/internal/config"
	"auth-service/internal/database"
	"auth-service/internal/handlers"
	localMiddleware "auth-service/internal/middleware"
	"auth-service/internal/repositories"
	"auth-service/internal/services"
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	sharedDB "shared/database"
	sharedMiddleware "shared/middleware"
)

// main initializes and starts the Auth Service application with complete dependency setup
func main() {
	// Parse command line flags for environment selection
	var environment = flag.String("env", "prod", "Environment to run in (local, prod)")
	flag.Parse()

	// Set Gin framework mode based on environment for appropriate logging and debugging
	if *environment == "local" {
		gin.SetMode(gin.DebugMode)
		log.Println("🚀 Starting Auth Service in LOCAL mode")
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.Println("🚀 Starting Auth Service in PRODUCTION mode")
	}

	// Load environment-specific configuration from TOML file
	cfg, err := config.Load(*environment)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize PostgreSQL database connection with retry logic
	ctx := context.Background()
	dbConfig := sharedDB.ConnectionConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Name:            cfg.Database.Name,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.Database.ConnMaxLifetime) * time.Second,
		Timezone:        "UTC",
	}
	retryConfig := sharedDB.DefaultRetryConfig()
	db, err := sharedDB.ConnectWithRetry(ctx, dbConfig, retryConfig)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto-migrate database schema to ensure tables exist and are up to date
	if err := database.Migrate(db); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Initialize Redis client with retry logic for session management and token blacklisting
	redisClient := database.ConnectRedis(cfg.Redis)

	// Initialize data access layer repositories with database connections
	userRepo := repositories.NewUserRepository(db)
	sessionRepo := repositories.NewSessionRepository(db, redisClient)

	// Initialize business logic services with repositories and configuration
	authService := services.NewAuthService(userRepo, sessionRepo, cfg.JWT)
	// oauth2Service := services.NewOAuth2Service(cfg.OAuth2) // Temporarily disabled

	// Initialize HTTP handlers with service dependencies
	authHandler := handlers.NewAuthHandler(authService, nil) // Pass nil for OAuth2Service temporarily

	// Setup HTTP router with middleware and route definitions
	router := setupRouter(authHandler, cfg)
	
	log.Println("✅ Rate limiting handled by Traefik Gateway")

	// HTTP Server with proper configuration
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("✅ Auth Service starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down Auth Service...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}

	// Close Redis connection
	if redisClient != nil {
		redisClient.Close()
	}

	log.Println("✅ Auth Service stopped")
}

// setupRouter configures HTTP router with comprehensive middleware and API route definitions
func setupRouter(authHandler *handlers.AuthHandler, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	// Initialize JWT middleware with secret from config
	jwtMiddleware := sharedMiddleware.NewJWTMiddleware(cfg.JWT.AccessSecret)

	// Apply global middleware for all routes
	router.Use(localMiddleware.CORS(&cfg.CORS)) // Cross-origin request handling
	router.Use(localMiddleware.Logger())       // HTTP request logging for monitoring
	router.Use(localMiddleware.Recovery())     // Panic recovery to prevent server crashes

	// Health check endpoint for load balancers and monitoring systems
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"service":   "auth-service",
			"timestamp": "2024-01-01T00:00:00Z",
		})
	})

	// Prometheus metrics endpoint for application monitoring
	router.GET("/metrics", localMiddleware.PrometheusHandler())

	// API version 1 route group
	v1 := router.Group("/api/v1")
	{
		// Authentication route group
		auth := v1.Group("/auth")
		{
			// Public authentication endpoints (no JWT required)
			auth.POST("/register", authHandler.Register)           // User registration
			auth.POST("/login", authHandler.Login)                 // User authentication
			auth.POST("/refresh", authHandler.RefreshToken)        // Token refresh
			auth.POST("/forgot-password", authHandler.ForgotPassword) // Password reset request
			auth.POST("/reset-password", authHandler.ResetPassword)   // Password reset execution

			// OAuth2 integration endpoints for external provider authentication
			auth.GET("/oauth/:provider", authHandler.OAuthLogin)         // OAuth login initiation
			auth.GET("/oauth/:provider/callback", authHandler.OAuthCallback) // OAuth callback handling

			// Protected endpoints requiring valid JWT authentication
			protected := auth.Group("/")
			protected.Use(jwtMiddleware.AuthRequired()) // JWT validation middleware
			{
				// Existing auth endpoints
				protected.GET("/me", authHandler.GetMe)                     // Basic auth info only
				protected.POST("/logout", authHandler.Logout)               // Session termination
				protected.POST("/change-password", authHandler.ChangePassword) // Password change
				protected.DELETE("/account", authHandler.DeleteAccount)     // Account deletion

				// NEW: Unified User Service endpoints (Task 4.1 - API Integration)
				// These endpoints moved from User Service (/api/v1/users/*) to Auth Service (/api/v1/auth/*)
				protected.GET("/profile", authHandler.GetProfile)                    // Previously /api/v1/users/profile
				protected.PUT("/profile", authHandler.UpdateProfile)                // Previously /api/v1/users/profile

				protected.GET("/preferences", authHandler.GetUserPreferences)       // Previously /api/v1/users/preferences  
				protected.POST("/preferences", authHandler.CreateUserPreferences)   // Create new preferences
				protected.PUT("/preferences", authHandler.UpdateUserPreferences)    // Previously /api/v1/users/preferences

				protected.GET("/activities", authHandler.GetUserActivities)         // Previously /api/v1/users/activities

				protected.GET("/notifications", authHandler.GetUserNotifications)   // Previously /api/v1/users/notifications
				protected.PUT("/notifications/:notificationId/read", authHandler.MarkNotificationAsRead) // New unified endpoint
			}
		}

		// Token verification endpoint for API Gateway ForwardAuth integration
		v1.POST("/verify", authHandler.VerifyToken)
	}

	return router
}