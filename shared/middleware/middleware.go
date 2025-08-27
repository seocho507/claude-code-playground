package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"shared/config"
)

// CORSConfig contains CORS middleware configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

// CORS returns a CORS middleware with configurable options
func CORS(cfg CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Set Access-Control-Allow-Origin
		if len(cfg.AllowedOrigins) == 0 || (len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*") {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			for _, allowedOrigin := range cfg.AllowedOrigins {
				if allowedOrigin == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		// Set other CORS headers
		if len(cfg.AllowedMethods) > 0 {
			c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		}
		
		if len(cfg.AllowedHeaders) > 0 {
			c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		}
		
		if len(cfg.ExposeHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
		}
		
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		
		if cfg.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// DefaultCORS returns a CORS middleware with sensible defaults
func DefaultCORS() gin.HandlerFunc {
	return CORS(CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	})
}

// Logger returns a structured logging middleware
func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %s %d %s %s %s\n",
			param.TimeStamp.Format("2006-01-02 15:04:05"),
			param.ClientIP,
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// Recovery returns a recovery middleware with custom error handling
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		log.Printf("Panic recovered: %v", recovered)
		
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		c.Abort()
	})
}

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// Timeout adds request timeout middleware
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// JWTAuth provides JWT authentication middleware
type JWTAuth struct {
	AccessSecret  string
	RefreshSecret string
	Algorithm     string
}

// NewJWTAuth creates a new JWT authentication middleware
func NewJWTAuth(jwtConfig config.JWTConfig) *JWTAuth {
	algorithm := jwtConfig.Algorithm
	if algorithm == "" {
		algorithm = "HS256"
	}
	
	return &JWTAuth{
		AccessSecret:  jwtConfig.AccessSecret,
		RefreshSecret: jwtConfig.RefreshSecret,
		Algorithm:     algorithm,
	}
}

// AuthRequired returns middleware that requires valid JWT authentication
func (j *JWTAuth) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":     "Authorization header required",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// Extract Bearer token
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":     "Invalid authorization header format",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Verify signing method
			if token.Method.Alg() != j.Algorithm {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(j.AccessSecret), nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":     "Invalid token: " + err.Error(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":     "Token is not valid",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// Extract claims
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("user_id", claims["user_id"])
			c.Set("username", claims["username"])
			c.Set("email", claims["email"])
			c.Set("roles", claims["roles"])
		}

		c.Next()
	}
}

// AdminRequired returns middleware that requires admin role
func (j *JWTAuth) AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":     "No roles found in token",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// Check if user has admin role
		roleList, ok := roles.([]interface{})
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":     "Invalid roles format",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		hasAdminRole := false
		for _, role := range roleList {
			if roleStr, ok := role.(string); ok && (roleStr == "admin" || roleStr == "administrator") {
				hasAdminRole = true
				break
			}
		}

		if !hasAdminRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":     "Admin role required",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimit provides basic in-memory rate limiting
func RateLimit(requests int, duration time.Duration) gin.HandlerFunc {
	type client struct {
		requests int
		resetTime time.Time
	}
	
	clients := make(map[string]*client)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()
		
		if clientData, exists := clients[clientIP]; exists {
			if now.Before(clientData.resetTime) {
				if clientData.requests >= requests {
					c.JSON(http.StatusTooManyRequests, gin.H{
						"error":     "Rate limit exceeded",
						"timestamp": time.Now().UTC().Format(time.RFC3339),
						"retry_after": clientData.resetTime.Sub(now).Seconds(),
					})
					c.Abort()
					return
				}
				clientData.requests++
			} else {
				clientData.requests = 1
				clientData.resetTime = now.Add(duration)
			}
		} else {
			clients[clientIP] = &client{
				requests: 1,
				resetTime: now.Add(duration),
			}
		}
		
		c.Next()
	}
}

// SecurityHeaders adds common security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}