package repositories

import (
	"auth-service/internal/models"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrUserPreferencesNotFound = errors.New("user preferences not found")
)

// allowedProfileFields defines which fields can be updated via UpdateProfile
// This prevents updating sensitive fields like password_hash, is_active, etc.
var allowedProfileFields = map[string]bool{
	"first_name":    true,
	"last_name":     true,
	"bio":           true,
	"phone_number":  true,
	"avatar_url":    true,
	"date_of_birth": true,
	"gender":        true,
	"country":       true,
	"city":          true,
	"timezone":      true,
	"language":      true,
	"website":       true,
	"linkedin":      true,
	"twitter":       true,
	"github":        true,
}

// Constants for GetUserActivities
const (
	// DefaultActivityLimit is the default number of activities to return
	DefaultActivityLimit = 100
	// MaxActivityLimit is the maximum number of activities to return to prevent DoS
	MaxActivityLimit = 1000
)

// Constants for CreateUserActivity
const (
	// MaxActionLength is the maximum length for activity action field
	MaxActionLength = 100
	// DefaultMetadata is the default JSON metadata for activities
	DefaultMetadata = "{}"
)

// Constants for CreateUserNotification
const (
	// MaxNotificationTypeLength is the maximum length for notification type field
	MaxNotificationTypeLength = 50
	// MaxNotificationTitleLength is the maximum length for notification title field
	MaxNotificationTitleLength = 200
)

type UserRepository interface {
	Create(user *models.User) error
	GetByID(id uuid.UUID) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByOAuthID(provider, oauthID string) (*models.User, error)
	Update(user *models.User) error
	Delete(userID uuid.UUID) error
	UpdateLastLogin(userID uuid.UUID, ipAddress string) error
	IncrementFailedAttempts(userID uuid.UUID) error
	ResetFailedAttempts(userID uuid.UUID) error
	CreateLoginAttempt(attempt *models.LoginAttempt) error
	IsEmailTaken(email string) (bool, error)
	IsUsernameTaken(username string) (bool, error)
	
	// Extended User Service functionality - User Preferences
	GetUserPreferences(userID uuid.UUID) (*models.UserPreference, error)
	CreateUserPreferences(prefs *models.UserPreference) error
	UpdateUserPreferences(prefs *models.UserPreference) error
	
	// Extended User Service functionality - Profile Management
	UpdateProfile(userID uuid.UUID, fields map[string]interface{}) error
	
	// Extended User Service functionality - User Activities
	GetUserActivities(userID uuid.UUID, limit, offset int) ([]models.UserActivity, error)
	CreateUserActivity(activity *models.UserActivity) error
	
	// Extended User Service functionality - User Notifications
	GetUserNotifications(userID uuid.UUID) ([]models.UserNotification, error)
	CreateUserNotification(notification *models.UserNotification) error
	MarkNotificationAsRead(userID, notificationID uuid.UUID) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) GetByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.Where("id = ? AND is_active = ?", id, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ? AND is_active = ?", email, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ? AND is_active = ?", username, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByOAuthID(provider, oauthID string) (*models.User, error) {
	var user models.User
	var err error

	switch provider {
	case "google":
		err = r.db.Where("google_id = ? AND is_active = ?", oauthID, true).First(&user).Error
	case "github":
		err = r.db.Where("git_hub_id = ? AND is_active = ?", oauthID, true).First(&user).Error
	case "facebook":
		err = r.db.Where("facebook_id = ? AND is_active = ?", oauthID, true).First(&user).Error
	default:
		return nil, errors.New("unsupported OAuth provider")
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) Delete(userID uuid.UUID) error {
	// Soft delete by setting deleted_at timestamp
	return r.db.Delete(&models.User{}, userID).Error
}

func (r *userRepository) UpdateLastLogin(userID uuid.UUID, ipAddress string) error {
	now := time.Now()
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"last_login_at": now,
			"last_login_ip": ipAddress,
		}).Error
}

func (r *userRepository) IncrementFailedAttempts(userID uuid.UUID) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		UpdateColumn("failed_login_attempts", gorm.Expr("failed_login_attempts + 1")).Error
}

func (r *userRepository) ResetFailedAttempts(userID uuid.UUID) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"failed_login_attempts": 0,
			"locked_until":         nil,
		}).Error
}

func (r *userRepository) CreateLoginAttempt(attempt *models.LoginAttempt) error {
	return r.db.Create(attempt).Error
}

func (r *userRepository) IsEmailTaken(email string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) IsUsernameTaken(username string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// Extended User Service functionality implementations

func (r *userRepository) GetUserPreferences(userID uuid.UUID) (*models.UserPreference, error) {
	var prefs models.UserPreference
	err := r.db.Where("user_id = ?", userID).First(&prefs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserPreferencesNotFound
		}
		return nil, err
	}
	return &prefs, nil
}

func (r *userRepository) CreateUserPreferences(prefs *models.UserPreference) error {
	return r.db.Create(prefs).Error
}

func (r *userRepository) UpdateUserPreferences(prefs *models.UserPreference) error {
	return r.db.Save(prefs).Error
}

// Extended User Service functionality implementations - Profile Management

// validateProfileUpdateFields validates the input fields map
func validateProfileUpdateFields(fields map[string]interface{}) error {
	if fields == nil {
		return errors.New("fields map cannot be nil")
	}
	
	if len(fields) == 0 {
		return errors.New("no fields to update")
	}
	
	return nil
}

// filterAllowedFields filters the input fields to only include allowed profile fields
func filterAllowedFields(fields map[string]interface{}) (map[string]interface{}, error) {
	updateFields := make(map[string]interface{})
	for key, value := range fields {
		if allowedProfileFields[key] {
			updateFields[key] = value
		}
	}
	
	if len(updateFields) == 0 {
		return nil, errors.New("no valid fields to update")
	}
	
	return updateFields, nil
}

func (r *userRepository) UpdateProfile(userID uuid.UUID, fields map[string]interface{}) error {
	// Validate input
	if err := validateProfileUpdateFields(fields); err != nil {
		return err
	}
	
	// Filter to only allowed fields
	updateFields, err := filterAllowedFields(fields)
	if err != nil {
		return err
	}
	
	// Check if user exists
	var user models.User
	err = r.db.Where("id = ? AND is_active = ?", userID, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}
	
	// Update the user with filtered fields
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(updateFields).Error
}

// Extended User Service functionality implementations - User Activities

// validateActivityPaginationParams validates pagination parameters for GetUserActivities
func validateActivityPaginationParams(limit, offset int) error {
	if limit < 0 {
		return errors.New("limit cannot be negative")
	}
	
	if offset < 0 {
		return errors.New("offset cannot be negative")
	}
	
	return nil
}

// normalizeActivityLimit normalizes the limit parameter to safe bounds
func normalizeActivityLimit(limit int) int {
	// Default limit if zero or too large to prevent potential DoS
	if limit == 0 || limit > MaxActivityLimit {
		return DefaultActivityLimit
	}
	
	return limit
}

func (r *userRepository) GetUserActivities(userID uuid.UUID, limit, offset int) ([]models.UserActivity, error) {
	// Validate input parameters
	if err := validateActivityPaginationParams(limit, offset); err != nil {
		return nil, err
	}
	
	// Normalize limit to safe bounds
	limit = normalizeActivityLimit(limit)
	
	var activities []models.UserActivity
	
	// Retrieve activities for the user, ordered by most recent first
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&activities).Error
		
	if err != nil {
		return nil, err
	}
	
	return activities, nil
}

// validateUserActivity validates the input UserActivity
func validateUserActivity(activity *models.UserActivity) error {
	if activity == nil {
		return errors.New("activity cannot be nil")
	}
	
	if activity.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}
	
	if activity.Action == "" {
		return errors.New("action is required")
	}
	
	// Validate action length (based on database schema)
	if len(activity.Action) > MaxActionLength {
		return errors.New("action exceeds maximum length of 100 characters")
	}
	
	return nil
}

// setActivityDefaults sets default values for UserActivity if not provided
func setActivityDefaults(activity *models.UserActivity) {
	// Set defaults if not provided
	if activity.ID == uuid.Nil {
		activity.ID = uuid.New()
	}
	
	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now()
	}
	
	// Ensure Metadata is valid JSON (empty object if not set)
	if activity.Metadata == "" {
		activity.Metadata = DefaultMetadata
	}
}

func (r *userRepository) CreateUserActivity(activity *models.UserActivity) error {
	// Validate input
	if err := validateUserActivity(activity); err != nil {
		return err
	}
	
	// Set defaults
	setActivityDefaults(activity)
	
	// Create the activity in the database
	return r.db.Create(activity).Error
}

// Extended User Service functionality implementations - User Notifications

// validateUserID validates that the user ID is not empty
func validateUserID(userID uuid.UUID) error {
	if userID == uuid.Nil {
		return errors.New("user_id is required")
	}
	return nil
}

func (r *userRepository) GetUserNotifications(userID uuid.UUID) ([]models.UserNotification, error) {
	// Validate input
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	
	var notifications []models.UserNotification
	
	// Retrieve notifications for the user, ordered by most recent first
	// Filter out expired notifications where expires_at is not null and < now
	err := r.db.Where("user_id = ? AND (expires_at IS NULL OR expires_at > ?)", userID, time.Now().UTC()).
		Order("created_at DESC").
		Find(&notifications).Error
		
	if err != nil {
		return nil, err
	}
	
	return notifications, nil
}

// Extended User Service functionality implementations - User Notifications Creation

// validateUserNotification validates the input UserNotification
func validateUserNotification(notification *models.UserNotification) error {
	if notification == nil {
		return errors.New("notification cannot be nil")
	}
	
	if notification.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}
	
	if notification.Type == "" {
		return errors.New("type is required")
	}
	
	if notification.Title == "" {
		return errors.New("title is required")
	}
	
	// Validate type length (based on database schema)
	if len(notification.Type) > MaxNotificationTypeLength {
		return errors.New("type exceeds maximum length of 50 characters")
	}
	
	// Validate title length (based on database schema)
	if len(notification.Title) > MaxNotificationTitleLength {
		return errors.New("title exceeds maximum length of 200 characters")
	}
	
	return nil
}

// setNotificationDefaults sets default values for UserNotification if not provided
func setNotificationDefaults(notification *models.UserNotification) {
	// Set defaults if not provided
	if notification.ID == uuid.Nil {
		notification.ID = uuid.New()
	}
	
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}
	
	// IsRead defaults to false (Go's zero value for bool), so no need to set explicitly
}

func (r *userRepository) CreateUserNotification(notification *models.UserNotification) error {
	// Validate input
	if err := validateUserNotification(notification); err != nil {
		return err
	}
	
	// Set defaults
	setNotificationDefaults(notification)
	
	// Create the notification in the database
	return r.db.Create(notification).Error
}

// validateNotificationReadParams validates parameters for MarkNotificationAsRead
func validateNotificationReadParams(userID, notificationID uuid.UUID) error {
	if userID == uuid.Nil {
		return errors.New("user_id is required")
	}
	
	if notificationID == uuid.Nil {
		return errors.New("notification_id is required")
	}
	
	return nil
}

func (r *userRepository) MarkNotificationAsRead(userID, notificationID uuid.UUID) error {
	// Validate input parameters
	if err := validateNotificationReadParams(userID, notificationID); err != nil {
		return err
	}
	
	now := time.Now()
	
	// Update notification: set is_read = true and read_at = now
	// Only update if notification belongs to the specified user (security check)
	result := r.db.Model(&models.UserNotification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})
	
	if result.Error != nil {
		return result.Error
	}
	
	// Check if any row was actually updated (notification exists and belongs to user)
	if result.RowsAffected == 0 {
		return errors.New("notification not found or does not belong to user")
	}
	
	return nil
}