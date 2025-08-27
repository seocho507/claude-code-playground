package models

import "time"

// Request DTOs
type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Username    string `json:"username" binding:"required,min=3,max=30"`
	Password    string `json:"password" binding:"required,min=8"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

type UpdateProfileRequest struct {
	Username    string `json:"username,omitempty" binding:"omitempty,min=3,max=30"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

// Extended profile requests for the unified system
type UpdateExtendedProfileRequest struct {
	Bio         string     `json:"bio,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Gender      string     `json:"gender,omitempty"`
	Country     string     `json:"country,omitempty"`
	City        string     `json:"city,omitempty"`
	Timezone    string     `json:"timezone,omitempty"`
	Language    string     `json:"language,omitempty"`
	Website     string     `json:"website,omitempty"`
	LinkedIn    string     `json:"linkedin,omitempty"`
	Twitter     string     `json:"twitter,omitempty"`
	GitHub      string     `json:"github,omitempty"`
	IsPublic    *bool      `json:"is_public,omitempty"`
}

type CreatePreferencesRequest struct {
	EmailNotifications  *bool  `json:"email_notifications,omitempty"`
	PushNotifications   *bool  `json:"push_notifications,omitempty"`
	TwoFactorEnabled    *bool  `json:"two_factor_enabled,omitempty"`
	Theme               string `json:"theme,omitempty"`
	Language            string `json:"language,omitempty"`
	PrivacyLevel        string `json:"privacy_level,omitempty"`
	MarketingEmails     *bool  `json:"marketing_emails,omitempty"`
}

type UpdatePreferencesRequest struct {
	EmailNotifications  *bool  `json:"email_notifications,omitempty"`
	PushNotifications   *bool  `json:"push_notifications,omitempty"`
	TwoFactorEnabled    *bool  `json:"two_factor_enabled,omitempty"`
	Theme               string `json:"theme,omitempty"`
	Language            string `json:"language,omitempty"`
	PrivacyLevel        string `json:"privacy_level,omitempty"`
	MarketingEmails     *bool  `json:"marketing_emails,omitempty"`
}

type VerifyTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// Response DTOs
type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	User         UserInfo  `json:"user"`
}

type RefreshResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
}

type UserInfo struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Username      string     `json:"username"`
	FirstName     string     `json:"first_name,omitempty"`
	LastName      string     `json:"last_name,omitempty"`
	PhoneNumber   string     `json:"phone_number,omitempty"`
	Role          UserRole   `json:"role"`
	IsActive      bool       `json:"is_active"`
	EmailVerified bool       `json:"email_verified"`
	IsVerified    bool       `json:"is_verified"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	Avatar        string     `json:"avatar,omitempty"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type VerifyTokenResponse struct {
	Valid  bool     `json:"valid"`
	UserID string   `json:"user_id,omitempty"`
	Role   UserRole `json:"role,omitempty"`
	Email  string   `json:"email,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JWT Claims
type JWTClaims struct {
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Username string   `json:"username"`
	Role     UserRole `json:"role"`
	Type     string   `json:"type"` // "access" or "refresh"
	Issuer   string   `json:"iss"`
	Subject  string   `json:"sub"`
	IssuedAt int64    `json:"iat"`
	ExpiresAt int64   `json:"exp"`
}

// OAuth2 User Info
type OAuth2UserInfo struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Provider string `json:"provider"`
}

// OAuthUser represents OAuth user data
type OAuthUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Provider string `json:"provider"`
}

// CreateNotificationRequest represents a request to create a notification
type CreateNotificationRequest struct {
	Type       string     `json:"type" binding:"required"`
	Title      string     `json:"title" binding:"required"`
	Message    string     `json:"message" binding:"required"`
	ActionURL  string     `json:"action_url,omitempty"`
	ActionText string     `json:"action_text,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}