package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"shared/events"
	"shared/redis"

	redisClient "github.com/redis/go-redis/v9"
)

// Session represents a user session
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Email     string                 `json:"email"`
	Username  string                 `json:"username"`
	Roles     []string               `json:"roles"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	ExpiresAt time.Time              `json:"expires_at"`
	IPAddress string                 `json:"ip_address,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Active    bool                   `json:"active"`
}

// SessionManager provides distributed session management
type SessionManager struct {
	redis    *redis.RedisManager
	eventBus *events.EventBus
	config   Config
}

// Config contains session configuration
type Config struct {
	DefaultTTL       time.Duration
	MaxSessions      int    // Maximum sessions per user
	EnableEvents     bool   // Enable session events
	EnableLogging    bool   // Enable session logging
	CleanupInterval  time.Duration
	SessionKeyPrefix string
	UserSessionsKey  string // Key pattern for user sessions
}

// NewSessionManager creates a new session manager
func NewSessionManager(client *redisClient.Client, eventBus *events.EventBus, config Config) *SessionManager {
	redisManager := redis.NewRedisManager(client, "session")
	
	sm := &SessionManager{
		redis:    redisManager,
		eventBus: eventBus,
		config:   config,
	}
	
	// Start cleanup routine
	if config.CleanupInterval > 0 {
		go sm.startCleanupRoutine()
	}
	
	return sm
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(ctx context.Context, session Session) error {
	// Set session metadata
	session.CreatedAt = time.Now().UTC()
	session.UpdatedAt = session.CreatedAt
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = session.CreatedAt.Add(sm.config.DefaultTTL)
	}
	session.Active = true
	
	// Check session limits for user
	if sm.config.MaxSessions > 0 {
		if err := sm.enforceSessionLimit(ctx, session.UserID); err != nil {
			return err
		}
	}
	
	// Store session
	sessionKey := sm.sessionKey(session.ID)
	ttl := time.Until(session.ExpiresAt)
	
	if err := sm.redis.Set(ctx, sessionKey, session, ttl); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	// Add to user sessions set
	userSessionsKey := sm.userSessionsKey(session.UserID)
	if err := sm.redis.SAdd(ctx, userSessionsKey, session.ID); err != nil {
		log.Printf("Failed to add session to user sessions set: %v", err)
	}
	
	// Set TTL on user sessions set
	sm.redis.Expire(ctx, userSessionsKey, sm.config.DefaultTTL)
	
	if sm.config.EnableLogging {
		log.Printf("📝 Created session: %s for user: %s", session.ID, session.UserID)
	}
	
	// Publish session created event
	if sm.config.EnableEvents && sm.eventBus != nil {
		event := events.NewAuthEvent(
			events.SessionCreated,
			"session-manager",
			session.UserID,
			session.ID,
			map[string]interface{}{
				"ip_address": session.IPAddress,
				"user_agent": session.UserAgent,
			},
		)
		sm.eventBus.Publish(ctx, event)
	}
	
	return nil
}

// GetSession retrieves a session
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	sessionKey := sm.sessionKey(sessionID)
	
	var session Session
	if err := sm.redis.Get(ctx, sessionKey, &session); err != nil {
		if err == redisClient.Nil {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	// Check if session is expired
	if time.Now().UTC().After(session.ExpiresAt) {
		sm.DeleteSession(ctx, sessionID) // Clean up expired session
		return nil, fmt.Errorf("session expired")
	}
	
	return &session, nil
}

// UpdateSession updates session data
func (sm *SessionManager) UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	
	// Apply updates
	session.UpdatedAt = time.Now().UTC()
	
	if data, ok := updates["data"]; ok {
		if dataMap, ok := data.(map[string]interface{}); ok {
			if session.Data == nil {
				session.Data = make(map[string]interface{})
			}
			for k, v := range dataMap {
				session.Data[k] = v
			}
		}
	}
	
	// Update other fields
	if ipAddress, ok := updates["ip_address"].(string); ok {
		session.IPAddress = ipAddress
	}
	if userAgent, ok := updates["user_agent"].(string); ok {
		session.UserAgent = userAgent
	}
	
	// Save updated session
	sessionKey := sm.sessionKey(sessionID)
	ttl := time.Until(session.ExpiresAt)
	
	return sm.redis.Set(ctx, sessionKey, session, ttl)
}

// RefreshSession extends session TTL
func (sm *SessionManager) RefreshSession(ctx context.Context, sessionID string) error {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	
	// Extend expiration
	session.ExpiresAt = time.Now().UTC().Add(sm.config.DefaultTTL)
	session.UpdatedAt = time.Now().UTC()
	
	// Save refreshed session
	sessionKey := sm.sessionKey(sessionID)
	ttl := time.Until(session.ExpiresAt)
	
	return sm.redis.Set(ctx, sessionKey, session, ttl)
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	// Get session for cleanup and events
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil && err.Error() != "session not found" {
		return err
	}
	
	// Delete session
	sessionKey := sm.sessionKey(sessionID)
	if err := sm.redis.Delete(ctx, sessionKey); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	
	// Remove from user sessions set
	if session != nil {
		userSessionsKey := sm.userSessionsKey(session.UserID)
		sm.redis.SRem(ctx, userSessionsKey, sessionID)
		
		if sm.config.EnableLogging {
			log.Printf("🗑️ Deleted session: %s for user: %s", sessionID, session.UserID)
		}
		
		// Publish session expired event
		if sm.config.EnableEvents && sm.eventBus != nil {
			event := events.NewAuthEvent(
				events.SessionExpired,
				"session-manager",
				session.UserID,
				sessionID,
				nil,
			)
			sm.eventBus.Publish(ctx, event)
		}
	}
	
	return nil
}

// GetUserSessions retrieves all active sessions for a user
func (sm *SessionManager) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	userSessionsKey := sm.userSessionsKey(userID)
	
	sessionIDs, err := sm.redis.SMembers(ctx, userSessionsKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	
	var sessions []*Session
	for _, sessionID := range sessionIDs {
		session, err := sm.GetSession(ctx, sessionID)
		if err != nil {
			// Session might be expired, remove from set
			sm.redis.SRem(ctx, userSessionsKey, sessionID)
			continue
		}
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

// DeleteUserSessions removes all sessions for a user
func (sm *SessionManager) DeleteUserSessions(ctx context.Context, userID string) error {
	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}
	
	for _, session := range sessions {
		if err := sm.DeleteSession(ctx, session.ID); err != nil {
			log.Printf("Failed to delete session %s: %v", session.ID, err)
		}
	}
	
	// Clear user sessions set
	userSessionsKey := sm.userSessionsKey(userID)
	return sm.redis.Delete(ctx, userSessionsKey)
}

// ValidateSession checks if session is valid and active
func (sm *SessionManager) ValidateSession(ctx context.Context, sessionID string) (*Session, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	
	if !session.Active {
		return nil, fmt.Errorf("session is not active")
	}
	
	return session, nil
}

// enforceSessionLimit ensures user doesn't exceed max sessions
func (sm *SessionManager) enforceSessionLimit(ctx context.Context, userID string) error {
	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}
	
	if len(sessions) >= sm.config.MaxSessions {
		// Remove oldest session
		oldestSession := sessions[0]
		for _, session := range sessions {
			if session.CreatedAt.Before(oldestSession.CreatedAt) {
				oldestSession = session
			}
		}
		
		if err := sm.DeleteSession(ctx, oldestSession.ID); err != nil {
			return fmt.Errorf("failed to remove oldest session: %w", err)
		}
		
		if sm.config.EnableLogging {
			log.Printf("🔄 Removed oldest session %s for user %s due to limit", oldestSession.ID, userID)
		}
	}
	
	return nil
}

// Key generation helpers
func (sm *SessionManager) sessionKey(sessionID string) string {
	return fmt.Sprintf("%s:%s", sm.config.SessionKeyPrefix, sessionID)
}

func (sm *SessionManager) userSessionsKey(userID string) string {
	return fmt.Sprintf("%s:user:%s", sm.config.UserSessionsKey, userID)
}

// Cleanup routine for expired sessions
func (sm *SessionManager) startCleanupRoutine() {
	ticker := time.NewTicker(sm.config.CleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		sm.cleanupExpiredSessions()
	}
}

func (sm *SessionManager) cleanupExpiredSessions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// This would scan for expired sessions and remove them
	// Implementation depends on Redis scanning strategy
	log.Println("🧹 Running session cleanup routine")
}

// Session statistics
type SessionStats struct {
	TotalSessions   int64            `json:"total_sessions"`
	ActiveSessions  int64            `json:"active_sessions"`
	ExpiredSessions int64            `json:"expired_sessions"`
	UserCounts      map[string]int64 `json:"user_counts,omitempty"`
}

func (sm *SessionManager) GetStats(ctx context.Context) (*SessionStats, error) {
	// Implementation would collect actual statistics
	stats := &SessionStats{
		UserCounts: make(map[string]int64),
	}
	
	return stats, nil
}

// Health check
func (sm *SessionManager) HealthCheck(ctx context.Context) error {
	// Test session creation and deletion
	testSession := Session{
		ID:     "health-check",
		UserID: "system",
		Email:  "health@check.com",
	}
	
	if err := sm.CreateSession(ctx, testSession); err != nil {
		return fmt.Errorf("session health check failed on create: %w", err)
	}
	
	if err := sm.DeleteSession(ctx, testSession.ID); err != nil {
		return fmt.Errorf("session health check failed on delete: %w", err)
	}
	
	return nil
}

// Default configurations
func DefaultConfig() Config {
	return Config{
		DefaultTTL:       30 * time.Minute,
		MaxSessions:      5,
		EnableEvents:     true,
		EnableLogging:    true,
		CleanupInterval:  15 * time.Minute,
		SessionKeyPrefix: "session",
		UserSessionsKey:  "user_sessions",
	}
}

func ProductionConfig() Config {
	return Config{
		DefaultTTL:       60 * time.Minute,
		MaxSessions:      10,
		EnableEvents:     true,
		EnableLogging:    false,
		CleanupInterval:  30 * time.Minute,
		SessionKeyPrefix: "session",
		UserSessionsKey:  "user_sessions",
	}
}