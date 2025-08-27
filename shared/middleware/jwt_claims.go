package middleware

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the claims in our JWT tokens
type JWTClaims struct {
	UserID    string   `json:"user_id"`
	Email     string   `json:"email"`
	Username  string   `json:"username"`
	Role      string   `json:"role"`
	Roles     []string `json:"roles,omitempty"`
	Type      string   `json:"type"` // "access" or "refresh"
	SessionID string   `json:"session_id,omitempty"`

	// Standard JWT claims
	Issuer    string `json:"iss,omitempty"`
	Subject   string `json:"sub,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	NotBefore int64  `json:"nbf,omitempty"`
}

// UserInfo represents basic user information extracted from JWT
type UserInfo struct {
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Roles    []string `json:"roles,omitempty"`
}

// Valid validates the JWT claims according to JWT standards
func (c JWTClaims) Valid() error {
	now := time.Now().Unix()

	// Check if token has expired
	if c.ExpiresAt > 0 && now > c.ExpiresAt {
		return errors.New("token has expired")
	}

	// Check if token is used before valid time
	if c.NotBefore > 0 && now < c.NotBefore {
		return errors.New("token used before valid time")
	}

	// Check if token is issued in the future
	if c.IssuedAt > 0 && now < c.IssuedAt {
		return errors.New("token issued in the future")
	}

	return nil
}

// GetExpirationTime implements jwt.Claims interface
func (c JWTClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	if c.ExpiresAt == 0 {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)), nil
}

// GetIssuedAt implements jwt.Claims interface
func (c JWTClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	if c.IssuedAt == 0 {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

// GetNotBefore implements jwt.Claims interface
func (c JWTClaims) GetNotBefore() (*jwt.NumericDate, error) {
	if c.NotBefore == 0 {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.NotBefore, 0)), nil
}

// GetIssuer implements jwt.Claims interface
func (c JWTClaims) GetIssuer() (string, error) {
	return c.Issuer, nil
}

// GetSubject implements jwt.Claims interface
func (c JWTClaims) GetSubject() (string, error) {
	return c.Subject, nil
}

// GetAudience implements jwt.Claims interface
func (c JWTClaims) GetAudience() (jwt.ClaimStrings, error) {
	// We don't use audience claims, return empty
	return jwt.ClaimStrings{}, nil
}

// IsExpired checks if the token is expired
func (c JWTClaims) IsExpired() bool {
	if c.ExpiresAt == 0 {
		return false // No expiration time means never expires
	}
	return time.Now().Unix() > c.ExpiresAt
}

// HasRole checks if the user has a specific role
func (c JWTClaims) HasRole(role string) bool {
	if role == "" {
		return false
	}

	// Check main role field
	if c.Role == role {
		return true
	}

	// Check roles array
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}

	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (c JWTClaims) HasAnyRole(roles ...string) bool {
	if len(roles) == 0 {
		return false
	}

	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}

	return false
}

// IsAccessToken checks if this is an access token
func (c JWTClaims) IsAccessToken() bool {
	return c.Type == "access"
}

// IsRefreshToken checks if this is a refresh token
func (c JWTClaims) IsRefreshToken() bool {
	return c.Type == "refresh"
}

// ToMap converts claims to jwt.MapClaims for token generation
func (c JWTClaims) ToMap() jwt.MapClaims {
	claims := jwt.MapClaims{
		"user_id":  c.UserID,
		"email":    c.Email,
		"username": c.Username,
		"role":     c.Role,
		"type":     c.Type,
	}

	if len(c.Roles) > 0 {
		claims["roles"] = c.Roles
	}

	if c.SessionID != "" {
		claims["session_id"] = c.SessionID
	}

	if c.Issuer != "" {
		claims["iss"] = c.Issuer
	}

	if c.Subject != "" {
		claims["sub"] = c.Subject
	}

	if c.ExpiresAt > 0 {
		claims["exp"] = c.ExpiresAt
	}

	if c.IssuedAt > 0 {
		claims["iat"] = c.IssuedAt
	}

	if c.NotBefore > 0 {
		claims["nbf"] = c.NotBefore
	}

	return claims
}

// FromMap populates claims from jwt.MapClaims
func (c *JWTClaims) FromMap(claims jwt.MapClaims) error {
	if userID, ok := claims["user_id"]; ok {
		if str, ok := userID.(string); ok {
			c.UserID = str
		}
	}

	if email, ok := claims["email"]; ok {
		if str, ok := email.(string); ok {
			c.Email = str
		}
	}

	if username, ok := claims["username"]; ok {
		if str, ok := username.(string); ok {
			c.Username = str
		}
	}

	if role, ok := claims["role"]; ok {
		if str, ok := role.(string); ok {
			c.Role = str
		}
	}

	if roles, ok := claims["roles"]; ok {
		if roleSlice, ok := roles.([]interface{}); ok {
			c.Roles = make([]string, len(roleSlice))
			for i, role := range roleSlice {
				if str, ok := role.(string); ok {
					c.Roles[i] = str
				}
			}
		}
	}

	if tokenType, ok := claims["type"]; ok {
		if str, ok := tokenType.(string); ok {
			c.Type = str
		}
	}

	if sessionID, ok := claims["session_id"]; ok {
		if str, ok := sessionID.(string); ok {
			c.SessionID = str
		}
	}

	if issuer, ok := claims["iss"]; ok {
		if str, ok := issuer.(string); ok {
			c.Issuer = str
		}
	}

	if subject, ok := claims["sub"]; ok {
		if str, ok := subject.(string); ok {
			c.Subject = str
		}
	}

	if exp, ok := claims["exp"]; ok {
		if num, ok := exp.(float64); ok {
			c.ExpiresAt = int64(num)
		}
	}

	if iat, ok := claims["iat"]; ok {
		if num, ok := iat.(float64); ok {
			c.IssuedAt = int64(num)
		}
	}

	if nbf, ok := claims["nbf"]; ok {
		if num, ok := nbf.(float64); ok {
			c.NotBefore = int64(num)
		}
	}

	return nil
}

// ToUserInfo converts claims to UserInfo
func (c JWTClaims) ToUserInfo() *UserInfo {
	return &UserInfo{
		UserID:   c.UserID,
		Email:    c.Email,
		Username: c.Username,
		Role:     c.Role,
		Roles:    c.Roles,
	}
}

// String returns a string representation of the claims for logging
func (c JWTClaims) String() string {
	return fmt.Sprintf("JWTClaims{UserID:%s, Email:%s, Role:%s, Type:%s, ExpiresAt:%d}",
		c.UserID, c.Email, c.Role, c.Type, c.ExpiresAt)
}