package services

import (
	"auth-service/internal/config"
	"auth-service/internal/models"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"shared/middleware"
)

// parseDuration parses duration string and returns time.Duration
func parseDuration(durationStr string) time.Duration {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		log.Printf("Error parsing duration '%s': %v, using default 15m", durationStr, err)
		return 15 * time.Minute
	}
	return duration
}

type JWTService interface {
	GenerateTokenPair(user *models.User) (*models.AuthResponse, error)
	GenerateAccessToken(user *models.User) (string, error)
	GenerateRefreshToken(user *models.User) (string, error)
	ValidateToken(tokenString string) (*middleware.JWTClaims, error)
	ValidateRefreshToken(tokenString string) (*middleware.JWTClaims, error)
	HashToken(token string) string
	GetTokenClaims(tokenString string) (*middleware.JWTClaims, error)
}

type jwtService struct {
	config config.JWTConfig
}

func NewJWTService(cfg config.JWTConfig) JWTService {
	return &jwtService{config: cfg}
}

func (s *jwtService) GenerateTokenPair(user *models.User) (*models.AuthResponse, error) {
	// Generate access token
	accessToken, err := s.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshToken, err := s.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(parseDuration(s.config.AccessExpiry).Seconds()),
		User: models.UserInfo{
			ID:            user.ID.String(),
			Email:         user.Email,
			Username:      user.Username,
			Role:          user.Role,
			IsActive:      user.IsActive,
			EmailVerified: user.EmailVerified,
			Avatar:        user.Avatar,
			LastLoginAt:   user.LastLoginAt,
			CreatedAt:     user.CreatedAt,
		},
	}, nil
}

func (s *jwtService) GenerateAccessToken(user *models.User) (string, error) {
	now := time.Now()
	claims := &middleware.JWTClaims{
		UserID:    user.ID.String(),
		Email:     user.Email,
		Username:  user.Username,
		Role:      string(user.Role),
		Type:      "access",
		Issuer:    s.config.Issuer,
		Subject:   user.ID.String(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(parseDuration(s.config.AccessExpiry)).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  claims.UserID,
		"email":    claims.Email,
		"username": claims.Username,
		"role":     claims.Role,
		"type":     claims.Type,
		"iss":      claims.Issuer,
		"sub":      claims.Subject,
		"iat":      claims.IssuedAt,
		"exp":      claims.ExpiresAt,
	})

	return token.SignedString([]byte(s.config.AccessSecret))
}

func (s *jwtService) GenerateRefreshToken(user *models.User) (string, error) {
	now := time.Now()
	claims := &middleware.JWTClaims{
		UserID:    user.ID.String(),
		Email:     user.Email,
		Username:  user.Username,
		Role:      string(user.Role),
		Type:      "refresh",
		Issuer:    s.config.Issuer,
		Subject:   user.ID.String(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(parseDuration(s.config.RefreshExpiry)).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  claims.UserID,
		"email":    claims.Email,
		"username": claims.Username,
		"role":     claims.Role,
		"type":     claims.Type,
		"iss":      claims.Issuer,
		"sub":      claims.Subject,
		"iat":      claims.IssuedAt,
		"exp":      claims.ExpiresAt,
	})

	return token.SignedString([]byte(s.config.RefreshSecret))
}

func (s *jwtService) ValidateToken(tokenString string) (*middleware.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(s.config.AccessSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Verify token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "access" {
		return nil, errors.New("invalid token type")
	}

	return s.mapClaimsToJWTClaims(claims)
}

func (s *jwtService) ValidateRefreshToken(tokenString string) (*middleware.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(s.config.RefreshSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Verify token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, errors.New("invalid token type")
	}

	return s.mapClaimsToJWTClaims(claims)
}

func (s *jwtService) HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *jwtService) GetTokenClaims(tokenString string) (*middleware.JWTClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return s.mapClaimsToJWTClaims(claims)
}

func (s *jwtService) mapClaimsToJWTClaims(claims jwt.MapClaims) (*middleware.JWTClaims, error) {
	userID, ok := claims["user_id"].(string)
	if !ok {
		return nil, errors.New("invalid user_id claim")
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, errors.New("invalid email claim")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return nil, errors.New("invalid username claim")
	}

	roleStr, ok := claims["role"].(string)
	if !ok {
		return nil, errors.New("invalid role claim")
	}

	tokenType, ok := claims["type"].(string)
	if !ok {
		return nil, errors.New("invalid type claim")
	}

	issuer, ok := claims["iss"].(string)
	if !ok {
		return nil, errors.New("invalid issuer claim")
	}

	subject, ok := claims["sub"].(string)
	if !ok {
		return nil, errors.New("invalid subject claim")
	}

	issuedAt, ok := claims["iat"].(float64)
	if !ok {
		return nil, errors.New("invalid issued at claim")
	}

	expiresAt, ok := claims["exp"].(float64)
	if !ok {
		return nil, errors.New("invalid expires at claim")
	}

	return &middleware.JWTClaims{
		UserID:    userID,
		Email:     email,
		Username:  username,
		Role:      roleStr,
		Type:      tokenType,
		Issuer:    issuer,
		Subject:   subject,
		IssuedAt:  int64(issuedAt),
		ExpiresAt: int64(expiresAt),
	}, nil
}