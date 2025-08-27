package services

import (
	"auth-service/internal/config"
	"auth-service/internal/models"
	"auth-service/internal/repositories"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	// Existing Auth functionality
	Register(req *models.RegisterRequest) (*models.AuthResponse, error)
	Login(req *models.LoginRequest, ipAddress, userAgent string) (*models.AuthResponse, error)
	RefreshToken(req *models.RefreshTokenRequest) (*models.RefreshResponse, error)
	VerifyToken(token string) (*models.VerifyTokenResponse, error)
	Logout(userID uuid.UUID, token string) error
	ChangePassword(userID uuid.UUID, req *models.ChangePasswordRequest) error
	DeleteAccount(userID uuid.UUID) error
	GetProfile(userID uuid.UUID) (*models.UserInfo, error)
	UpdateProfile(userID uuid.UUID, req *models.UpdateProfileRequest) (*models.UserInfo, error)
	ForgotPassword(req *models.ForgotPasswordRequest) error
	ResetPassword(req *models.ResetPasswordRequest) error
	
	// Extended User Service functionality (from refactoring plan Task 1.2)
	GetUserPreferences(userID uuid.UUID) (*models.UserPreference, error)
	CreateUserPreferences(userID uuid.UUID, req *models.CreatePreferencesRequest) (*models.UserPreference, error)
	UpdateUserPreferences(userID uuid.UUID, req *UpdatePreferencesRequest) (*models.UserPreference, error)
	
	// Activity and notification management
	LogUserActivity(userID uuid.UUID, action, description string, metadata map[string]interface{}) error
	GetUserActivities(userID uuid.UUID, limit, offset int) ([]models.UserActivity, error)
	
	GetUserNotifications(userID uuid.UUID) ([]models.UserNotification, error)
	MarkNotificationAsRead(userID, notificationID uuid.UUID) error
	CreateNotification(userID uuid.UUID, req *CreateNotificationRequest) error
}

// Request types for extended User Service functionality
type UpdatePreferencesRequest struct {
	EmailNotifications *bool   `json:"email_notifications,omitempty"`
	PushNotifications  *bool   `json:"push_notifications,omitempty"`
	TwoFactorEnabled   *bool   `json:"two_factor_enabled,omitempty"`
	Theme              string  `json:"theme,omitempty"`
	PrivacyLevel       string  `json:"privacy_level,omitempty"`
	MarketingEmails    *bool   `json:"marketing_emails,omitempty"`
}

type CreateNotificationRequest struct {
	Type       string `json:"type" validate:"required"`
	Title      string `json:"title" validate:"required"`
	Message    string `json:"message"`
	ActionURL  string `json:"action_url,omitempty"`
	ActionText string `json:"action_text,omitempty"`
	ExpiresAt  *int64 `json:"expires_at,omitempty"` // Unix timestamp
}

type authService struct {
	userRepo    repositories.UserRepository
	sessionRepo repositories.SessionRepository
	jwtService  JWTService
}

func NewAuthService(userRepo repositories.UserRepository, sessionRepo repositories.SessionRepository, jwtConfig config.JWTConfig) AuthService {
	return &authService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtService:  NewJWTService(jwtConfig),
	}
}

func (s *authService) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	// Check if email is already taken
	emailTaken, err := s.userRepo.IsEmailTaken(req.Email)
	if err != nil {
		return nil, err
	}
	if emailTaken {
		return nil, errors.New("email already exists")
	}

	// Check if username is already taken
	usernameTaken, err := s.userRepo.IsUsernameTaken(req.Username)
	if err != nil {
		return nil, err
	}
	if usernameTaken {
		return nil, errors.New("username already exists")
	}

	// Hash password
	hashedPassword, err := s.hashPassword(req.Password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	// Create user
	user := &models.User{
		Email:        strings.ToLower(req.Email),
		Username:     req.Username,
		PasswordHash: hashedPassword,
		Role:         models.RoleUser,
		IsActive:     true,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// Generate tokens
	return s.jwtService.GenerateTokenPair(user)
}

func (s *authService) Login(req *models.LoginRequest, ipAddress, userAgent string) (*models.AuthResponse, error) {
	// Record login attempt
	loginAttempt := &models.LoginAttempt{
		Email:     req.Email,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   false,
	}

	// Get user by email
	user, err := s.userRepo.GetByEmail(strings.ToLower(req.Email))
	if err != nil {
		s.userRepo.CreateLoginAttempt(loginAttempt)
		return nil, errors.New("invalid credentials")
	}

	// Check if user can attempt login
	if !user.CanAttemptLogin() {
		s.userRepo.CreateLoginAttempt(loginAttempt)
		if user.IsLocked() {
			return nil, errors.New("account is temporarily locked")
		}
		return nil, errors.New("account is inactive")
	}

	// Verify password
	if !s.verifyPassword(req.Password, user.PasswordHash) {
		user.IncrementFailedAttempts()
		s.userRepo.Update(user)
		s.userRepo.CreateLoginAttempt(loginAttempt)
		return nil, errors.New("invalid credentials")
	}

	// Reset failed attempts on successful login
	if user.FailedLoginAttempts > 0 {
		user.ResetFailedAttempts()
		s.userRepo.Update(user)
	}

	// Update last login
	s.userRepo.UpdateLastLogin(user.ID, ipAddress)

	// Record successful login attempt
	loginAttempt.Success = true
	s.userRepo.CreateLoginAttempt(loginAttempt)

	// Generate tokens
	authResponse, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Store refresh token in Redis
	refreshTokenHash := s.jwtService.HashToken(authResponse.RefreshToken)
	if err := s.sessionRepo.StoreRefreshToken(user.ID, refreshTokenHash, 7*24*time.Hour); err != nil {
		return nil, err
	}

	// Create session record
	session := &models.Session{
		UserID:          user.ID,
		AccessTokenHash: s.jwtService.HashToken(authResponse.AccessToken),
		RefreshToken:    refreshTokenHash,
		ExpiresAt:       time.Now().Add(15 * time.Minute),
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
		DeviceInfo:      `{}`, // Set empty JSON object for JSONB column
		IsActive:        true,
	}

	if err := s.sessionRepo.CreateSession(session); err != nil {
		return nil, err
	}

	return authResponse, nil
}

func (s *authService) RefreshToken(req *models.RefreshTokenRequest) (*models.RefreshResponse, error) {
	// Validate refresh token
	claims, err := s.jwtService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Check if refresh token is blacklisted
	tokenHash := s.jwtService.HashToken(req.RefreshToken)
	isBlacklisted, err := s.sessionRepo.IsTokenBlacklisted(tokenHash)
	if err != nil {
		return nil, err
	}
	if isBlacklisted {
		return nil, errors.New("refresh token is blacklisted")
	}

	// Verify refresh token in Redis
	userIDStr, err := s.sessionRepo.GetRefreshTokenData(tokenHash)
	if err != nil {
		return nil, errors.New("refresh token not found")
	}

	if userIDStr != claims.UserID {
		return nil, errors.New("invalid refresh token")
	}

	// Parse user ID to UUID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	// Get user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if !user.IsActive {
		return nil, errors.New("user account is inactive")
	}

	// Generate new access token
	newAccessToken, err := s.jwtService.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	// Optionally generate new refresh token (rotation)
	newRefreshToken, err := s.jwtService.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	// Store new refresh token in Redis
	newRefreshTokenHash := s.jwtService.HashToken(newRefreshToken)
	if err := s.sessionRepo.StoreRefreshToken(user.ID, newRefreshTokenHash, 7*24*time.Hour); err != nil {
		return nil, err
	}

	// Invalidate old refresh token
	s.sessionRepo.DeleteRefreshToken(tokenHash)

	return &models.RefreshResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(15 * time.Minute.Seconds()),
	}, nil
}

func (s *authService) VerifyToken(token string) (*models.VerifyTokenResponse, error) {
	// Validate token
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return &models.VerifyTokenResponse{Valid: false}, nil
	}

	// Check if token is blacklisted
	tokenHash := s.jwtService.HashToken(token)
	isBlacklisted, err := s.sessionRepo.IsTokenBlacklisted(tokenHash)
	if err != nil {
		return &models.VerifyTokenResponse{Valid: false}, nil
	}
	if isBlacklisted {
		return &models.VerifyTokenResponse{Valid: false}, nil
	}

	// Verify user still exists and is active
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return &models.VerifyTokenResponse{Valid: false}, nil
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return &models.VerifyTokenResponse{Valid: false}, nil
	}

	if !user.IsActive {
		return &models.VerifyTokenResponse{Valid: false}, nil
	}

	return &models.VerifyTokenResponse{
		Valid:  true,
		UserID: claims.UserID,
		Role:   models.UserRole(claims.Role),
		Email:  claims.Email,
	}, nil
}

func (s *authService) Logout(userID uuid.UUID, token string) error {
	// Blacklist the access token
	tokenHash := s.jwtService.HashToken(token)
	if err := s.sessionRepo.BlacklistToken(tokenHash, 15*time.Minute); err != nil {
		return err
	}

	// Revoke all sessions for the user
	return s.sessionRepo.RevokeAllUserSessions(userID)
}

func (s *authService) ChangePassword(userID uuid.UUID, req *models.ChangePasswordRequest) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	// Verify current password
	if !s.verifyPassword(req.CurrentPassword, user.PasswordHash) {
		return errors.New("invalid current password")
	}

	// Hash new password
	newPasswordHash, err := s.hashPassword(req.NewPassword)
	if err != nil {
		return errors.New("failed to hash new password")
	}

	// Update password
	user.PasswordHash = newPasswordHash
	return s.userRepo.Update(user)
}

func (s *authService) DeleteAccount(userID uuid.UUID) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	// Soft delete the user account by setting deleted_at timestamp
	return s.userRepo.Delete(user.ID)
}

func (s *authService) GetProfile(userID uuid.UUID) (*models.UserInfo, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	return &models.UserInfo{
		ID:            user.ID.String(),
		Email:         user.Email,
		Username:      user.Username,
		Role:          user.Role,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		Avatar:        user.Avatar,
		LastLoginAt:   user.LastLoginAt,
		CreatedAt:     user.CreatedAt,
	}, nil
}

func (s *authService) UpdateProfile(userID uuid.UUID, req *models.UpdateProfileRequest) (*models.UserInfo, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Update fields if provided
	if req.Username != "" {
		// Check if username is taken by another user
		existingUser, _ := s.userRepo.GetByUsername(req.Username)
		if existingUser != nil && existingUser.ID != userID {
			return nil, errors.New("username already taken")
		}
		user.Username = req.Username
	}

	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	// Save changes
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	return &models.UserInfo{
		ID:            user.ID.String(),
		Email:         user.Email,
		Username:      user.Username,
		Role:          user.Role,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		Avatar:        user.Avatar,
		LastLoginAt:   user.LastLoginAt,
		CreatedAt:     user.CreatedAt,
	}, nil
}

func (s *authService) ForgotPassword(req *models.ForgotPasswordRequest) error {
	// This is a simplified implementation
	// In production, you would send an email with a reset link
	user, err := s.userRepo.GetByEmail(strings.ToLower(req.Email))
	if err != nil {
		// Don't reveal if email exists or not
		return nil
	}

	// Generate reset token
	resetToken, err := generateRandomToken(32)
	if err != nil {
		return err
	}

	// Create password reset record (simplified - not implemented in this example)
	_ = user
	_ = resetToken

	// TODO: Send email with reset link
	return nil
}

func (s *authService) ResetPassword(req *models.ResetPasswordRequest) error {
	// This is a simplified implementation
	// In production, you would verify the reset token and update the password
	_ = req
	return errors.New("not implemented")
}

// Helper functions
func (s *authService) hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *authService) verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Extended User Service functionality - Stub implementations for TDD
// These will be properly implemented after tests are written

func (s *authService) GetUserPreferences(userID uuid.UUID) (*models.UserPreference, error) {
	// Get user preferences from repository
	prefs, err := s.userRepo.GetUserPreferences(userID)
	if err != nil {
		if err == repositories.ErrUserPreferencesNotFound {
			return nil, errors.New("user preferences not found")
		}
		return nil, err
	}
	return prefs, nil
}

func (s *authService) UpdateUserPreferences(userID uuid.UUID, req *UpdatePreferencesRequest) (*models.UserPreference, error) {
	// Try to get existing preferences
	prefs, err := s.userRepo.GetUserPreferences(userID)
	if err != nil {
		if err == repositories.ErrUserPreferencesNotFound {
			// Create new preferences with defaults
			prefs = &models.UserPreference{
				UserID:             userID,
				EmailNotifications: true,
				PushNotifications:  true,
				TwoFactorEnabled:   false,
				Theme:              "light",
				PrivacyLevel:       "normal",
				MarketingEmails:    false,
			}
		} else {
			return nil, err
		}
	}
	
	// Update fields if provided in request
	if req.EmailNotifications != nil {
		prefs.EmailNotifications = *req.EmailNotifications
	}
	if req.PushNotifications != nil {
		prefs.PushNotifications = *req.PushNotifications
	}
	if req.TwoFactorEnabled != nil {
		prefs.TwoFactorEnabled = *req.TwoFactorEnabled
	}
	if req.Theme != "" {
		prefs.Theme = req.Theme
	}
	if req.PrivacyLevel != "" {
		prefs.PrivacyLevel = req.PrivacyLevel
	}
	if req.MarketingEmails != nil {
		prefs.MarketingEmails = *req.MarketingEmails
	}
	
	// Save preferences
	if prefs.ID == uuid.Nil {
		// Create new preferences
		err = s.userRepo.CreateUserPreferences(prefs)
	} else {
		// Update existing preferences
		err = s.userRepo.UpdateUserPreferences(prefs)
	}
	
	if err != nil {
		return nil, err
	}
	
	return prefs, nil
}

func (s *authService) CreateUserPreferences(userID uuid.UUID, req *models.CreatePreferencesRequest) (*models.UserPreference, error) {
	// Check if preferences already exist for this user
	_, err := s.userRepo.GetUserPreferences(userID)
	if err == nil {
		// Preferences already exist, return error
		return nil, errors.New("user preferences already exist")
	}
	if err != repositories.ErrUserPreferencesNotFound {
		// Some other error occurred
		return nil, err
	}
	
	// Create new preferences with default values, then override with request values
	prefs := &models.UserPreference{
		ID:                 uuid.New(),
		UserID:             userID,
		EmailNotifications: true,  // Default value
		PushNotifications:  true,  // Default value
		TwoFactorEnabled:   false, // Default value
		Theme:              "light", // Default value
		Language:           "en",    // Default value  
		PrivacyLevel:       "normal", // Default value
		MarketingEmails:    false,    // Default value
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	
	// Override with request values if provided
	if req.EmailNotifications != nil {
		prefs.EmailNotifications = *req.EmailNotifications
	}
	if req.PushNotifications != nil {
		prefs.PushNotifications = *req.PushNotifications
	}
	if req.TwoFactorEnabled != nil {
		prefs.TwoFactorEnabled = *req.TwoFactorEnabled
	}
	if req.Theme != "" {
		prefs.Theme = req.Theme
	}
	if req.Language != "" {
		prefs.Language = req.Language
	}
	if req.PrivacyLevel != "" {
		prefs.PrivacyLevel = req.PrivacyLevel
	}
	if req.MarketingEmails != nil {
		prefs.MarketingEmails = *req.MarketingEmails
	}
	
	// Create preferences in database
	err = s.userRepo.CreateUserPreferences(prefs)
	if err != nil {
		return nil, err
	}
	
	return prefs, nil
}

func (s *authService) LogUserActivity(userID uuid.UUID, action, description string, metadata map[string]interface{}) error {
	// Convert metadata to JSON string
	metadataJSON := "{}"
	if len(metadata) > 0 {
		if jsonBytes, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(jsonBytes)
		}
	}
	
	// Create user activity record
	activity := &models.UserActivity{
		ID:          uuid.New(),
		UserID:      userID,
		Action:      action,
		Description: description,
		Metadata:    metadataJSON,
		CreatedAt:   time.Now(),
	}
	
	// Save to repository
	return s.userRepo.CreateUserActivity(activity)
}

func (s *authService) GetUserActivities(userID uuid.UUID, limit, offset int) ([]models.UserActivity, error) {
	// Get user activities from repository
	return s.userRepo.GetUserActivities(userID, limit, offset)
}

func (s *authService) GetUserNotifications(userID uuid.UUID) ([]models.UserNotification, error) {
	// Get user notifications from repository
	return s.userRepo.GetUserNotifications(userID)
}

func (s *authService) MarkNotificationAsRead(userID, notificationID uuid.UUID) error {
	// Mark notification as read via repository
	return s.userRepo.MarkNotificationAsRead(userID, notificationID)
}

func (s *authService) CreateNotification(userID uuid.UUID, req *CreateNotificationRequest) error {
	// Create notification record
	notification := &models.UserNotification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      req.Type,
		Title:     req.Title,
		Message:   req.Message,
		IsRead:    false,
		CreatedAt: time.Now(),
	}
	
	// Optional fields
	if req.ActionURL != "" {
		notification.ActionURL = req.ActionURL
	}
	if req.ActionText != "" {
		notification.ActionText = req.ActionText
	}
	if req.ExpiresAt != nil {
		expiresAt := time.Unix(*req.ExpiresAt, 0)
		notification.ExpiresAt = &expiresAt
	}
	
	// Save to repository
	return s.userRepo.CreateUserNotification(notification)
}