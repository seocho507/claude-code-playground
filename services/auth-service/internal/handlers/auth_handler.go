package handlers

import (
	"auth-service/internal/models"
	"auth-service/internal/services"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sharedMiddleware "shared/middleware"
)

// AuthHandler handles HTTP authentication requests with comprehensive business logic integration
type AuthHandler struct {
	authService   services.AuthService   // Business logic for authentication operations
	oauth2Service services.OAuth2Service // OAuth2 integration for external providers
}

// NewAuthHandler creates AuthHandler instance with configured service dependencies
//
// Purpose: Constructs AuthHandler with required service layer dependencies
// Parameters:
//   - authService (services.AuthService): Business logic service for authentication operations
//     - Handles user registration, login, token management, profile operations
//     - Implements rate limiting, password policies, session management
//   - oauth2Service (services.OAuth2Service): OAuth2 integration service
//     - Manages external provider authentication (Google, GitHub, Facebook)
//     - Handles OAuth flows, token exchange, user profile mapping
// Returns:
//   - *AuthHandler: Configured handler instance ready for HTTP request processing
// Dependencies: Services must be properly initialized with repository and configuration
// Usage: Called during application initialization to wire HTTP layer with business logic
func NewAuthHandler(authService services.AuthService, oauth2Service services.OAuth2Service) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		oauth2Service: oauth2Service,
	}
}

// Register handles HTTP POST requests for user registration with comprehensive validation
//
// Endpoint: POST /api/v1/auth/register
// Purpose: Creates new user accounts with email/password authentication
// Authentication: None required (public endpoint)
// Request Body: RegisterRequest JSON containing user registration data
//   - email (string, required): Valid email address format
//   - password (string, required): Minimum 6 characters
//   - username (string, required): Unique username, minimum 3 characters
//   - first_name (string, optional): User's first name
//   - last_name (string, optional): User's last name
// Response: AuthResponse JSON with JWT tokens and user profile
//   - access_token: JWT token for API authentication (15 minutes)
//   - refresh_token: JWT token for token renewal (7 days)
//   - token_type: "Bearer" for Authorization header usage
//   - expires_in: Access token expiration in seconds
//   - user: User profile object with public fields only
// HTTP Status Codes:
//   - 201 Created: User registered successfully with tokens returned
//   - 400 Bad Request: Invalid JSON format or validation failures
//   - 409 Conflict: Email or username already exists
//   - 500 Internal Server Error: Database or service failures
// Validation Performed:
//   - JSON structure validation using Gin binding
//   - Email format validation
//   - Password strength checking (configurable policies)
//   - Username uniqueness verification
//   - Email uniqueness verification
// Side Effects:
//   - Creates user record in PostgreSQL database
//   - Generates and stores JWT session in database
//   - Logs registration attempt for audit trail
// Security Features:
//   - Password bcrypt hashing with configurable cost
//   - Automatic session creation with secure tokens
//   - Input sanitization and validation
// Rate Limiting: None applied (registration is typically unrestricted)
// Usage Example:
//   curl -X POST /api/v1/auth/register \
//     -H "Content-Type: application/json" \
//     -d '{"email":"user@example.com","password":"securepass","username":"newuser"}'
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	response, err := h.authService.Register(&req)
	if err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
		}
		
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Registration failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	response, err := h.authService.Login(&req, ipAddress, userAgent)
	if err != nil {
		statusCode := http.StatusUnauthorized
		if strings.Contains(err.Error(), "locked") {
			statusCode = http.StatusTooManyRequests
		}
		
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Login failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	response, err := h.authService.RefreshToken(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "Token refresh failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// VerifyToken handles token verification (for other services)
func (h *AuthHandler) VerifyToken(c *gin.Context) {
	var req models.VerifyTokenRequest
	
	// Try to get token from Authorization header first (for Traefik ForwardAuth)
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) == 2 && bearerToken[0] == "Bearer" {
			req.Token = bearerToken[1]
		}
	}
	
	// If no token in header, try JSON body
	if req.Token == "" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid request",
				Message: "Token required in Authorization header or request body",
			})
			return
		}
	}

	response, err := h.authService.VerifyToken(req.Token)
	if err != nil {
		// For ForwardAuth: return 401 for invalid tokens (not 500)
		c.Header("X-Auth-Status", "failed")
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "Token verification failed",
			Message: err.Error(),
		})
		return
	}

	// For ForwardAuth: return appropriate status based on token validity
	if !response.Valid {
		c.Header("X-Auth-Status", "invalid")
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid token",
		})
		return
	}

	// For ForwardAuth, set response headers for downstream services
	c.Header("X-User-ID", response.UserID)
	c.Header("X-User-Role", string(response.Role))
	c.Header("X-User-Email", response.Email)
	c.Header("X-Auth-Status", "authenticated")

	// For ForwardAuth, return 200 OK (Traefik needs 200 to proceed)
	c.JSON(http.StatusOK, gin.H{
		"valid":  true,
		"user_id": response.UserID,
		"email":   response.Email,
		"role":    response.Role,
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *gin.Context) {
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Token required",
		})
		return
	}

	// Get user ID from token verification
	verifyResponse, err := h.authService.VerifyToken(token.(string))
	if err != nil || !verifyResponse.Valid {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid token",
		})
		return
	}

	userID, err := uuid.Parse(verifyResponse.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	if err := h.authService.Logout(userID, token.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Logout failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Logged out successfully",
	})
}

// GetProfile returns user profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	profile, err := h.authService.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Profile not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateProfile updates user profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	profile, err := h.authService.UpdateProfile(userID, &req)
	if err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(err.Error(), "already taken") {
			statusCode = http.StatusConflict
		}
		
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Profile update failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// ChangePassword handles password change
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.authService.ChangePassword(userID, &req); err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(err.Error(), "invalid current password") {
			statusCode = http.StatusUnauthorized
		}
		
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Password change failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Password changed successfully",
	})
}

// ForgotPassword - Forgot Password API
// @Summary Request password reset
// @Description Send password reset email to user
// @Tags Password Recovery
// @Accept json
// @Produce json
// @Router /api/v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req models.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.authService.ForgotPassword(&req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to process request",
			Message: err.Error(),
		})
		return
	}

	// Always return success for security reasons
	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "If the email exists, a password reset link has been sent",
	})
}

// ResetPassword - Reset Password API
// @Summary Reset password with token
// @Description Complete password reset using token from email
// @Tags Password Recovery
// @Accept json
// @Produce json
// @Router /api/v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.authService.ResetPassword(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Password reset failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Password reset successfully",
	})
}

// DeleteAccount handles account deletion
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	if err := h.authService.DeleteAccount(userID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Account deletion failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Account deleted successfully",
	})
}

// OAuth2 handlers

// OAuthLogin - OAuth2 Login API
// @Summary Start OAuth2 authentication
// @Description Redirect to OAuth2 provider for authentication
// @Tags OAuth2
// @Param provider path string true "OAuth provider (google, github, facebook)"
// @Produce json
// @Router /api/v1/auth/oauth/{provider} [get]
func (h *AuthHandler) OAuthLogin(c *gin.Context) {
	provider := c.Param("provider")
	
	// Generate state parameter for CSRF protection
	state, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to generate state",
		})
		return
	}

	// Store state in session/cache for validation (simplified)
	// In production, you would store this in Redis with expiration

	authURL, err := h.oauth2Service.GetAuthURL(provider, state)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "OAuth login failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

// OAuthCallback - OAuth2 Callback API
// @Summary Handle OAuth2 callback
// @Description Process OAuth2 provider callback and create session
// @Tags OAuth2
// @Param provider path string true "OAuth provider (google, github, facebook)"
// @Param code query string true "Authorization code"
// @Param state query string true "State parameter"
// @Produce json
// @Router /api/v1/auth/oauth/{provider}/callback [get]
func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Missing code or state parameter",
		})
		return
	}

	// Validate state parameter (simplified)
	// In production, you would validate against stored state

	// Get user info from OAuth provider
	oauthUser, err := h.oauth2Service.HandleCallback(provider, code, state)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "OAuth callback failed",
			Message: err.Error(),
		})
		return
	}

	// TODO: Handle OAuth user (create account or login existing user)
	// This is a simplified response
	c.JSON(http.StatusOK, gin.H{
		"message": "OAuth login successful",
		"user":    oauthUser,
	})
}

// GetMe handles HTTP GET requests for basic auth user information
//
// Endpoint: GET /api/v1/auth/me  
// Purpose: Returns basic authentication-related user information only
// Authentication: JWT token required (protected endpoint)
// Response: Basic user data (id, email, username, is_active, email_verified)
// Note: This endpoint only returns auth-related data, not profile information
func (h *AuthHandler) GetMe(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	user, err := h.authService.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "User not found",
		})
		return
	}

	// Return only basic auth-related information
	response := gin.H{
		"id":             user.ID,
		"email":          user.Email,
		"username":       user.Username,
		"is_active":      user.IsActive,
		"email_verified": user.EmailVerified,
		"created_at":     user.CreatedAt,
		"last_login_at":  user.LastLoginAt,
	}

	c.JSON(http.StatusOK, response)
}

// Helper functions


// GetUserPreferences handles user preferences retrieval
func (h *AuthHandler) GetUserPreferences(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	preferences, err := h.authService.GetUserPreferences(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Preferences not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, preferences)
}

// UpdateUserPreferences - Update User Preferences API
// @Summary Update user preferences
// @Description Modify existing user preferences
// @Tags User Preferences
// @Security Bearer
// @Accept json
// @Produce json
// @Router /api/v1/auth/preferences [put]
func (h *AuthHandler) UpdateUserPreferences(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	var req models.UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Convert models.UpdatePreferencesRequest to services.UpdatePreferencesRequest
	serviceReq := &services.UpdatePreferencesRequest{
		EmailNotifications: req.EmailNotifications,
		PushNotifications:  req.PushNotifications,
		TwoFactorEnabled:   req.TwoFactorEnabled,
		Theme:              req.Theme,
		PrivacyLevel:       req.PrivacyLevel,
		MarketingEmails:    req.MarketingEmails,
	}

	preferences, err := h.authService.UpdateUserPreferences(userID, serviceReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Failed to update preferences",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, preferences)
}

// CreateUserPreferences - Create User Preferences API
// @Summary Create user preferences
// @Description Initialize preferences for new user
// @Tags User Preferences
// @Security Bearer
// @Accept json
// @Produce json
// @Router /api/v1/auth/preferences [post]
func (h *AuthHandler) CreateUserPreferences(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	var req models.CreatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	preferences, err := h.authService.CreateUserPreferences(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Failed to create preferences",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, preferences)
}

// GetUserActivities - Get User Activities API
// @Summary Get user activity history
// @Description Retrieve paginated list of user activities
// @Tags User Activities
// @Security Bearer
// @Produce json
// @Router /api/v1/auth/activities [get]
func (h *AuthHandler) GetUserActivities(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	// Parse query parameters for pagination
	limit := 50  // default
	offset := 0  // default
	
	if limitParam := c.Query("limit"); limitParam != "" {
		if parsedLimit, err := strconv.Atoi(limitParam); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	
	if offsetParam := c.Query("offset"); offsetParam != "" {
		if parsedOffset, err := strconv.Atoi(offsetParam); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	activities, err := h.authService.GetUserActivities(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get activities",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, activities)
}

// GetUserNotifications - Get User Notifications API
// @Summary Get user notifications
// @Description Retrieve list of notifications for user
// @Tags Notifications
// @Security Bearer
// @Produce json
// @Router /api/v1/auth/notifications [get]
func (h *AuthHandler) GetUserNotifications(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	notifications, err := h.authService.GetUserNotifications(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get notifications",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// MarkNotificationAsRead - Mark Notification Read API
// @Summary Mark notification as read
// @Description Update notification status to read
// @Tags Notifications
// @Security Bearer
// @Produce json
// @Router /api/v1/auth/notifications/{id}/read [put]
func (h *AuthHandler) MarkNotificationAsRead(c *gin.Context) {
	userIDStr := sharedMiddleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Message: "User ID must be a valid UUID",
		})
		return
	}

	notificationIDStr := c.Param("notificationId")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid notification ID",
			Message: "Notification ID must be a valid UUID",
		})
		return
	}

	if err := h.authService.MarkNotificationAsRead(userID, notificationID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to mark notification as read",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Notification marked as read",
	})
}

func generateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}