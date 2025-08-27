package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware handles JWT authentication
type JWTMiddleware struct {
	secret string
}

// NewJWTMiddleware creates a new JWT middleware instance
func NewJWTMiddleware(secret string) *JWTMiddleware {
	return &JWTMiddleware{
		secret: secret,
	}
}

// AuthRequired is middleware that requires valid JWT authentication
// Returns 401 if token is missing or invalid
func (m *JWTMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"error":   "Unauthorized",
				"message": "No token provided",
			})
			return
		}

		claims, err := m.validateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid token",
				"details": err.Error(),
			})
			return
		}

		// Set user information in context
		m.setUserContext(c, claims)
		c.Next()
	}
}

// OptionalAuth is middleware that optionally validates JWT authentication
// Continues processing even if token is missing or invalid
func (m *JWTMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.Next()
			return
		}

		claims, err := m.validateToken(token)
		if err != nil {
			// Log error but continue processing
			// Could add logging here
			c.Next()
			return
		}

		// Set user information in context
		m.setUserContext(c, claims)
		c.Next()
	}
}

// extractToken extracts JWT token from Authorization header
func extractToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if !strings.HasPrefix(bearer, "Bearer ") {
		return ""
	}

	token := strings.TrimPrefix(bearer, "Bearer ")
	if token == "" {
		return ""
	}

	return token
}

// validateToken validates JWT token and returns claims
func (m *JWTMiddleware) validateToken(tokenString string) (*JWTClaims, error) {
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract and validate claims
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Convert to our custom claims
	claims := &JWTClaims{}
	if err := claims.FromMap(mapClaims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Validate claims (expiration, etc.)
	if err := claims.Valid(); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	return claims, nil
}

// setUserContext sets user information in Gin context
func (m *JWTMiddleware) setUserContext(c *gin.Context, claims *JWTClaims) {
	c.Set("user_id", claims.UserID)
	c.Set("user_email", claims.Email)
	c.Set("user_username", claims.Username)
	c.Set("user_role", claims.Role)
	c.Set("user_roles", claims.Roles)
	c.Set("claims", claims)
}

// GetUserFromContext extracts user information from Gin context
// Returns nil if no user is authenticated
func GetUserFromContext(c *gin.Context) *UserInfo {
	claimsInterface, exists := c.Get("claims")
	if !exists {
		return nil
	}

	claims, ok := claimsInterface.(*JWTClaims)
	if !ok {
		return nil
	}

	return claims.ToUserInfo()
}

// GetUserIDFromContext extracts user ID from context
func GetUserIDFromContext(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		if str, ok := userID.(string); ok {
			return str
		}
	}
	return ""
}

// GetUserRoleFromContext extracts user role from context
func GetUserRoleFromContext(c *gin.Context) string {
	if role, exists := c.Get("user_role"); exists {
		if str, ok := role.(string); ok {
			return str
		}
	}
	return ""
}

// GetUserRolesFromContext extracts user roles from context
func GetUserRolesFromContext(c *gin.Context) []string {
	if roles, exists := c.Get("user_roles"); exists {
		if roleSlice, ok := roles.([]string); ok {
			return roleSlice
		}
	}
	return nil
}

// IsAuthenticated checks if the current request is authenticated
func IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}

// HasRole checks if the authenticated user has a specific role
func HasRole(c *gin.Context, role string) bool {
	userRole := GetUserRoleFromContext(c)
	if userRole == role {
		return true
	}

	userRoles := GetUserRolesFromContext(c)
	for _, r := range userRoles {
		if r == role {
			return true
		}
	}

	return false
}

// HasAnyRole checks if the authenticated user has any of the specified roles
func HasAnyRole(c *gin.Context, roles ...string) bool {
	for _, role := range roles {
		if HasRole(c, role) {
			return true
		}
	}
	return false
}

// GetClaimsFromContext extracts JWT claims from context
func GetClaimsFromContext(c *gin.Context) *JWTClaims {
	if claimsInterface, exists := c.Get("claims"); exists {
		if claims, ok := claimsInterface.(*JWTClaims); ok {
			return claims
		}
	}
	return nil
}