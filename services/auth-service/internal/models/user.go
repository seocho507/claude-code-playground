package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleUser      UserRole = "user"
	RoleAdmin     UserRole = "admin"
	RoleModerator UserRole = "moderator"
	// RolePremium removed - not in database enum (user_role: 'user', 'admin', 'moderator')
)

// User represents the unified user entity matching 001_initial_schema.sql exactly
// All fields correspond directly to database columns for schema consistency
type User struct {
	// Core identity - matches database PRIMARY KEY and UNIQUE constraints
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primary_key"`
	Email        string         `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	Username     string         `json:"username" gorm:"type:varchar(100);uniqueIndex;not null"`
	
	// Authentication & Authorization - matches user_role enum and defaults
	PasswordHash         string         `json:"-" gorm:"type:varchar(255);not null"`
	Role                 UserRole       `json:"role" gorm:"type:user_role;default:'user'"`
	IsActive             bool           `json:"is_active" gorm:"default:true"`
	EmailVerified        bool           `json:"email_verified" gorm:"default:false"`
	
	// OAuth2 fields - matches database VARCHAR(255) columns  
	GoogleID             string         `json:"-" gorm:"type:varchar(255);column:google_id"`
	GitHubID             string         `json:"-" gorm:"type:varchar(255);column:git_hub_id"` // Database uses git_hub_id
	FacebookID           string         `json:"-" gorm:"type:varchar(255);column:facebook_id"`
	
	// Profile fields - integrated from UserProfile, matches database schema
	FirstName    string         `json:"first_name" gorm:"type:varchar(100)"`
	LastName     string         `json:"last_name" gorm:"type:varchar(100)"`
	PhoneNumber  string         `json:"phone_number" gorm:"type:varchar(20)"`
	Bio          string         `json:"bio" gorm:"type:text"`
	AvatarURL    string         `json:"avatar_url" gorm:"type:varchar(500)"`
	DateOfBirth  *time.Time     `json:"date_of_birth,omitempty"`
	Gender       string         `json:"gender,omitempty" gorm:"type:varchar(10)"`
	Country      string         `json:"country,omitempty" gorm:"type:varchar(100)"`
	City         string         `json:"city,omitempty" gorm:"type:varchar(100)"`
	Timezone     string         `json:"timezone,omitempty" gorm:"type:varchar(50)"`
	Language     string         `json:"language" gorm:"type:varchar(10);default:'en'"`
	Website      string         `json:"website,omitempty" gorm:"type:varchar(500)"`
	
	// Social media fields - separate from OAuth IDs
	LinkedIn     string         `json:"linkedin,omitempty" gorm:"type:varchar(500);column:linkedin"`
	Twitter      string         `json:"twitter,omitempty" gorm:"type:varchar(500)"`
	GitHub       string         `json:"github,omitempty" gorm:"type:varchar(500);column:github"` // Social profile, not OAuth
	
	// Security tracking - matches database TIMESTAMP and INET columns
	LastLoginAt          *time.Time     `json:"last_login_at"`
	LastLoginIP          *string        `json:"-" gorm:"type:inet"` // Nullable INET to prevent empty string errors
	FailedLoginAttempts  int            `json:"-" gorm:"default:0"`
	LockedUntil          *time.Time     `json:"-"`
	
	// Timestamps - standard GORM fields matching database
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `json:"-" gorm:"index"`
	
	// Application-level fields (not in database schema)
	IsVerified           bool           `json:"is_verified" gorm:"-"` 
	VerifiedAt           *time.Time     `json:"verified_at,omitempty" gorm:"-"`
	Avatar               string         `json:"avatar" gorm:"-"`
	
	// Relations - Authentication
	Sessions             []Session           `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	PasswordResets       []PasswordReset     `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	
	// Relations - Extended from User Service  
	Profile              *UserProfile        `json:"profile,omitempty" gorm:"constraint:OnDelete:CASCADE"`
	Roles                []Role              `json:"roles,omitempty" gorm:"many2many:user_roles"`
	Preferences          *UserPreference     `json:"preferences,omitempty" gorm:"constraint:OnDelete:CASCADE"`
	Activities           []UserActivity      `json:"activities,omitempty" gorm:"constraint:OnDelete:CASCADE"`
	Notifications        []UserNotification  `json:"notifications,omitempty" gorm:"constraint:OnDelete:CASCADE"`
}

// BeforeCreate hook to set UUID if not already set
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Session represents authenticated user sessions - matches 001_initial_schema.sql exactly
// Migration-First: Model structure follows database schema as source of truth
type Session struct {
	// Identity and ownership - PRIMARY KEY and FOREIGN KEY constraints
	ID               uuid.UUID      `json:"id" gorm:"type:uuid;primary_key"`
	UserID           uuid.UUID      `json:"user_id" gorm:"type:uuid;not null;index"`  // FK to users(id) CASCADE
	
	// Token security - authentication token storage with database constraints
	RefreshToken     string         `json:"-" gorm:"uniqueIndex;not null;size:512"`   // UNIQUE VARCHAR(512) NOT NULL
	AccessTokenHash  string         `json:"-" gorm:"size:255"`                        // VARCHAR(255) for hashed tokens
	
	// Session context - request metadata with proper PostgreSQL types
	IPAddress        string         `json:"ip_address" gorm:"type:inet"`              // INET for IP addresses
	UserAgent        string         `json:"user_agent" gorm:"type:text"`              // TEXT for user agent strings
	DeviceInfo       string         `json:"device_info" gorm:"type:jsonb"`            // JSONB for device metadata
	
	// Session lifecycle - status tracking with database defaults
	IsActive         bool           `json:"is_active" gorm:"default:true"`            // BOOLEAN DEFAULT true
	
	// Audit trail - timestamp tracking with triggers
	CreatedAt        time.Time      `json:"created_at"`                               // TIMESTAMP DEFAULT NOW()
	UpdatedAt        time.Time      `json:"updated_at"`                               // TIMESTAMP with trigger
	ExpiresAt        time.Time      `json:"expires_at" gorm:"not null"`               // TIMESTAMP NOT NULL
	LastUsedAt       *time.Time     `json:"last_used_at"`                             // TIMESTAMP (nullable)
	
	// Relations - foreign key relationship
	User             User           `json:"-" gorm:"foreignKey:UserID"`
}

// BeforeCreate hook to set UUID if not already set
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type PasswordReset struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primary_key"`
	UserID    uuid.UUID      `json:"user_id" gorm:"type:uuid;not null;index"`
	Token     string         `json:"-" gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time      `json:"expires_at" gorm:"not null"`
	UsedAt    *time.Time     `json:"used_at"`
	CreatedAt time.Time      `json:"created_at"`
	
	// Relations
	User      User           `json:"-" gorm:"foreignKey:UserID"`
}

// BeforeCreate hook to set UUID if not already set
func (p *PasswordReset) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// LoginAttempt records authentication attempts for security auditing
// Matches database schema exactly from 001_initial_schema.sql
type LoginAttempt struct {
	// Primary identification
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key"`
	
	// User tracking - nullable for failed attempts with invalid credentials
	UserID        *uuid.UUID `json:"user_id" gorm:"type:uuid;index"`              // FK to users(id) ON DELETE CASCADE
	Email         string     `json:"email,omitempty" gorm:"size:255;index"`       // VARCHAR(255) - email used
	Username      string     `json:"username,omitempty" gorm:"size:100"`          // VARCHAR(100) - username used
	
	// Attempt outcome
	Success       bool       `json:"success" gorm:"not null;default:false"`       // BOOLEAN DEFAULT false
	FailureReason string     `json:"failure_reason,omitempty" gorm:"size:255"`    // VARCHAR(255) - error detail
	
	// Request context with PostgreSQL network types
	IPAddress     string     `json:"ip_address" gorm:"type:inet;not null;index"`  // INET for IP tracking
	UserAgent     string     `json:"user_agent,omitempty" gorm:"type:text"`       // TEXT for browser info
	
	// Audit timestamp
	AttemptedAt   time.Time  `json:"attempted_at" gorm:"default:now()"`           // TIMESTAMP DEFAULT NOW()
	
	// Relations
	User          *User      `json:"-" gorm:"foreignKey:UserID"`                  // Optional FK relation
}

// BeforeCreate hook to set UUID if not already set
func (l *LoginAttempt) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for User model
func (User) TableName() string {
	return "users"
}

// TableName returns the table name for Session model
func (Session) TableName() string {
	return "sessions"
}

// TableName returns the table name for PasswordReset model
func (PasswordReset) TableName() string {
	return "password_resets"
}

// TableName returns the table name for LoginAttempt model
func (LoginAttempt) TableName() string {
	return "login_attempts"
}

// IsLocked checks if user account is locked
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// CanAttemptLogin checks if user can attempt login
func (u *User) CanAttemptLogin() bool {
	return u.IsActive && !u.IsLocked()
}

// IncrementFailedAttempts increments failed login attempts
func (u *User) IncrementFailedAttempts() {
	u.FailedLoginAttempts++
	if u.FailedLoginAttempts >= 5 {
		lockUntil := time.Now().Add(15 * time.Minute)
		u.LockedUntil = &lockUntil
	}
}

// ResetFailedAttempts resets failed login attempts
func (u *User) ResetFailedAttempts() {
	u.FailedLoginAttempts = 0
	u.LockedUntil = nil
}

// UserProfile contains extended user information
type UserProfile struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Bio           string         `gorm:"type:text" json:"bio"`
	AvatarURL     string         `json:"avatar_url"`
	DateOfBirth   *time.Time     `json:"date_of_birth,omitempty"`
	Gender        string         `json:"gender,omitempty"`
	Country       string         `json:"country,omitempty"`
	City          string         `json:"city,omitempty"`
	Timezone      string         `json:"timezone,omitempty"`
	Language      string         `gorm:"default:'en'" json:"language"`
	Website       string         `json:"website,omitempty"`
	LinkedIn      string         `json:"linkedin,omitempty"`
	Twitter       string         `json:"twitter,omitempty"`
	GitHub        string         `json:"github,omitempty"`
	IsPublic      bool           `gorm:"default:true" json:"is_public"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func (p *UserProfile) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// UserPreference represents user preferences matching 001_initial_schema.sql exactly
// Migration-First principle: Database schema defines what fields exist
type UserPreference struct {
	// Primary identification - matches database PRIMARY KEY and FOREIGN KEY
	ID                     uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID                 uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	
	// Notification preferences - boolean fields with database defaults
	EmailNotifications     bool      `gorm:"default:true" json:"email_notifications"`
	PushNotifications      bool      `gorm:"default:true" json:"push_notifications"`
	MarketingEmails        bool      `gorm:"default:false" json:"marketing_emails"`
	
	// Security configuration - matches database constraints
	TwoFactorEnabled       bool      `gorm:"default:false" json:"two_factor_enabled"`
	
	// User interface preferences - VARCHAR fields with database CHECK constraints
	Theme                  string    `gorm:"type:varchar(20);default:'light'" json:"theme"`     // CHECK: 'light', 'dark', 'auto'
	Language               string    `gorm:"type:varchar(10);default:'en'" json:"language"`     // VARCHAR(10) DEFAULT 'en'
	
	// Privacy configuration - matches database CHECK constraint
	PrivacyLevel           string    `gorm:"type:varchar(20);default:'normal'" json:"privacy_level"` // CHECK: 'private', 'normal', 'public'
	
	// Audit timestamps - standard GORM fields with database triggers
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (p *UserPreference) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// UserActivity tracks user actions for audit trail - matches 001_initial_schema.sql exactly
type UserActivity struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`                        // UUID PRIMARY KEY
	UserID      uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`               // UUID FK to users(id)
	Action      string    `gorm:"type:varchar(100);not null" json:"action"`              // VARCHAR(100) NOT NULL to match database
	Description string    `gorm:"type:text" json:"description,omitempty"`                // TEXT for description
	IPAddress   string    `gorm:"type:inet" json:"ip_address,omitempty"`                 // INET for IP addresses
	UserAgent   string    `gorm:"type:text" json:"user_agent,omitempty"`                 // TEXT for user agent
	Metadata    string    `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`     // JSONB for structured data
	CreatedAt   time.Time `json:"created_at"`                                            // TIMESTAMP DEFAULT NOW()
}

func (a *UserActivity) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	// Ensure Metadata is valid JSON
	if a.Metadata == "" {
		a.Metadata = "{}"
	}
	return nil
}

// UserNotification represents system notifications to users - matches 001_initial_schema.sql exactly  
type UserNotification struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`                    // UUID PRIMARY KEY
	UserID     uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`           // UUID FK to users(id)
	Type       string     `gorm:"type:varchar(50);not null" json:"type"`           // VARCHAR(50) NOT NULL to match database
	Title      string     `gorm:"type:varchar(200);not null" json:"title"`         // VARCHAR(200) NOT NULL to match database
	Message    string     `gorm:"type:text" json:"message,omitempty"`                // TEXT for notification content
	IsRead     bool       `gorm:"default:false" json:"is_read"`                      // BOOLEAN DEFAULT false
	ReadAt     *time.Time `json:"read_at,omitempty"`                                 // TIMESTAMP nullable
	ActionURL  string     `gorm:"type:varchar(500)" json:"action_url,omitempty"`   // VARCHAR(500) to match database
	ActionText string     `gorm:"type:varchar(100)" json:"action_text,omitempty"` // VARCHAR(100) to match database
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`                              // TIMESTAMP nullable
	CreatedAt  time.Time  `json:"created_at"`                                        // TIMESTAMP DEFAULT NOW()
}

func (n *UserNotification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// Role represents user roles
type Role struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Name        string         `gorm:"uniqueIndex;not null" json:"name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	IsSystem    bool           `gorm:"default:false" json:"is_system"`
	Priority    int            `gorm:"default:0" json:"priority"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	
	// Relations
	Users       []User         `gorm:"many2many:user_roles" json:"-"`
	Permissions []Permission   `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
}

func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// Permission represents system permissions
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null" json:"name"`
	Resource    string    `gorm:"index;not null" json:"resource"`
	Action      string    `gorm:"not null" json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	
	// Relations
	Roles []Role `gorm:"many2many:role_permissions" json:"-"`
}

func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// UserRoleAssignment represents the many-to-many relationship between users and roles
type UserRoleAssignment struct {
	UserID     uuid.UUID `gorm:"type:uuid;primary_key" json:"user_id"`
	RoleID     uuid.UUID `gorm:"type:uuid;primary_key" json:"role_id"`
	AssignedAt time.Time `json:"assigned_at"`
	AssignedBy uuid.UUID `gorm:"type:uuid" json:"assigned_by"`
}

// RolePermission represents the many-to-many relationship between roles and permissions
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primary_key" json:"role_id"`
	PermissionID uuid.UUID `gorm:"type:uuid;primary_key" json:"permission_id"`
	GrantedAt    time.Time `json:"granted_at"`
	GrantedBy    uuid.UUID `gorm:"type:uuid" json:"granted_by"`
}