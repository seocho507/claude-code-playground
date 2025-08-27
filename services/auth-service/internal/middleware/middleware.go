package middleware

import (
	"auth-service/internal/config"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORS middleware with configuration support
func CORS(cfg *config.CORSConfig) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Check if origin is in allowed origins list, or allow all if empty
		if len(cfg.AllowedOrigins) == 0 || contains(cfg.AllowedOrigins, "*") {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else if contains(cfg.AllowedOrigins, origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}
		
		// Set credentials based on configuration
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		
		// Set allowed methods
		if len(cfg.AllowedMethods) > 0 {
			c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		
		// Set allowed headers
		if len(cfg.AllowedHeaders) > 0 {
			c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		
		// Set exposed headers
		if len(cfg.ExposedHeaders) > 0 {
			c.Writer.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
		}
		
		// Set max age
		if cfg.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Logger middleware
func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return ""
	})
}

// Recovery middleware
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		log.Printf("Panic recovered: %v", recovered)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
	})
}

// JWT Authentication Middleware
// IMPORTANT: JWT authentication middleware has been moved to shared/middleware package
// Use the following in your main.go or route setup:
//
// import sharedMiddleware "shared/middleware"
//
// jwtMiddleware := sharedMiddleware.NewJWTMiddleware(jwtSecret)
// protected.Use(jwtMiddleware.AuthRequired())
//
// Available functions in shared middleware:
// - AuthRequired(): Validates JWT and sets user context
// - OptionalAuth(): Optional JWT validation
// - GetUserIDFromContext(c): Extract user ID from context
// - GetUserFromContext(c): Extract full user info
// - IsAuthenticated(c): Check if user is authenticated
// - HasRole(c, role): Check if user has specific role

// PrometheusHandler returns a simple metrics endpoint
func PrometheusHandler() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// This is a simplified metrics endpoint
		// In production, you would use the Prometheus client library
		metrics := `# HELP auth_service_requests_total Total number of requests
# TYPE auth_service_requests_total counter
auth_service_requests_total 0

# HELP auth_service_request_duration_seconds Request duration
# TYPE auth_service_request_duration_seconds histogram
auth_service_request_duration_seconds_bucket{le="0.1"} 0
auth_service_request_duration_seconds_bucket{le="0.5"} 0
auth_service_request_duration_seconds_bucket{le="1.0"} 0
auth_service_request_duration_seconds_bucket{le="+Inf"} 0
auth_service_request_duration_seconds_sum 0
auth_service_request_duration_seconds_count 0

# HELP auth_service_active_sessions Active user sessions
# TYPE auth_service_active_sessions gauge
auth_service_active_sessions 0
`
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, metrics)
	})
}

// RateLimit middleware (simplified version)
func RateLimit() gin.HandlerFunc {
	// This is a simplified rate limiter
	// In production, you would use a proper rate limiting library
	return gin.HandlerFunc(func(c *gin.Context) {
		// Allow all requests for now
		c.Next()
	})
}

// RequestID middleware
func RequestID() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	})
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "000"
}