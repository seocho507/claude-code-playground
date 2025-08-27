package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"shared/events"
	"shared/redis"

	redisClient "github.com/redis/go-redis/v9"
)

// CacheManager provides centralized caching with event-driven invalidation
type CacheManager struct {
	redis    *redis.RedisManager
	eventBus *events.EventBus
	config   Config
}

// Config contains cache configuration
type Config struct {
	DefaultTTL     time.Duration
	UserTTL        time.Duration
	SessionTTL     time.Duration
	TokenTTL       time.Duration
	EnableMetrics  bool
	EnableLogging  bool
	PrefetchRules  map[string]PrefetchRule
}

// PrefetchRule defines cache prefetching behavior
type PrefetchRule struct {
	Pattern    string
	TTL        time.Duration
	Dependency []string // Events that trigger prefetch
}

// NewCacheManager creates a new cache manager
func NewCacheManager(client *redisClient.Client, eventBus *events.EventBus, config Config) *CacheManager {
	redisManager := redis.NewRedisManager(client, "cache")
	
	cm := &CacheManager{
		redis:    redisManager,
		eventBus: eventBus,
		config:   config,
	}
	
	// Register event handlers for cache invalidation
	cm.registerEventHandlers()
	
	return cm
}

// User-specific cache operations
func (cm *CacheManager) SetUser(ctx context.Context, userID string, user interface{}) error {
	key := fmt.Sprintf("user:%s", userID)
	ttl := cm.config.UserTTL
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}
	
	if cm.config.EnableLogging {
		log.Printf("🗃️ Caching user: %s", userID)
	}
	
	return cm.redis.Set(ctx, key, user, ttl)
}

func (cm *CacheManager) GetUser(ctx context.Context, userID string, dest interface{}) error {
	key := fmt.Sprintf("user:%s", userID)
	return cm.redis.Get(ctx, key, dest)
}

func (cm *CacheManager) InvalidateUser(ctx context.Context, userID string) error {
	keys := []string{
		fmt.Sprintf("user:%s", userID),
		fmt.Sprintf("user_profile:%s", userID),
		fmt.Sprintf("user_preferences:%s", userID),
	}
	
	if cm.config.EnableLogging {
		log.Printf("🗑️ Invalidating user cache: %s", userID)
	}
	
	// Publish cache invalidation event
	if cm.eventBus != nil {
		event := events.Event{
			Type:   events.CacheInvalidated,
			Source: "cache-manager",
			Data: map[string]interface{}{
				"type":    "user",
				"user_id": userID,
				"keys":    keys,
			},
		}
		cm.eventBus.Publish(ctx, event)
	}
	
	return cm.redis.Delete(ctx, keys...)
}

// Session cache operations
func (cm *CacheManager) SetSession(ctx context.Context, sessionID string, session interface{}) error {
	key := fmt.Sprintf("session:%s", sessionID)
	ttl := cm.config.SessionTTL
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}
	
	return cm.redis.Set(ctx, key, session, ttl)
}

func (cm *CacheManager) GetSession(ctx context.Context, sessionID string, dest interface{}) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return cm.redis.Get(ctx, key, dest)
}

func (cm *CacheManager) InvalidateSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	
	if cm.config.EnableLogging {
		log.Printf("🗑️ Invalidating session cache: %s", sessionID)
	}
	
	return cm.redis.Delete(ctx, key)
}

func (cm *CacheManager) RefreshSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	ttl := cm.config.SessionTTL
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}
	
	return cm.redis.Expire(ctx, key, ttl)
}

// Token blacklist operations
func (cm *CacheManager) BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	key := fmt.Sprintf("blacklist:%s", tokenID)
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil // Token already expired
	}
	
	if cm.config.EnableLogging {
		log.Printf("🚫 Blacklisting token: %s", tokenID)
	}
	
	return cm.redis.Set(ctx, key, true, ttl)
}

func (cm *CacheManager) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", tokenID)
	return cm.redis.Exists(ctx, key)
}

// Generic cache operations
func (cm *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}
	
	return cm.redis.Set(ctx, key, value, ttl)
}

func (cm *CacheManager) Get(ctx context.Context, key string, dest interface{}) error {
	return cm.redis.Get(ctx, key, dest)
}

func (cm *CacheManager) Delete(ctx context.Context, keys ...string) error {
	if cm.config.EnableLogging && len(keys) > 0 {
		log.Printf("🗑️ Deleting cache keys: %v", keys)
	}
	
	return cm.redis.Delete(ctx, keys...)
}

func (cm *CacheManager) Exists(ctx context.Context, key string) (bool, error) {
	return cm.redis.Exists(ctx, key)
}

// List cache operations (for paginated results)
func (cm *CacheManager) SetList(ctx context.Context, listKey string, items interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}
	
	return cm.redis.Set(ctx, fmt.Sprintf("list:%s", listKey), items, ttl)
}

func (cm *CacheManager) GetList(ctx context.Context, listKey string, dest interface{}) error {
	return cm.redis.Get(ctx, fmt.Sprintf("list:%s", listKey), dest)
}

func (cm *CacheManager) InvalidateListPattern(ctx context.Context, pattern string) error {
	// This would typically use Redis SCAN to find matching keys
	if cm.config.EnableLogging {
		log.Printf("🗑️ Invalidating list pattern: %s", pattern)
	}
	
	// Publish pattern invalidation event
	if cm.eventBus != nil {
		event := events.Event{
			Type:   events.CacheInvalidated,
			Source: "cache-manager",
			Data: map[string]interface{}{
				"type":    "pattern",
				"pattern": pattern,
			},
		}
		cm.eventBus.Publish(ctx, event)
	}
	
	return nil
}

// Cache warming operations
func (cm *CacheManager) WarmCache(ctx context.Context, key string, loader func() (interface{}, error), ttl time.Duration) error {
	// Check if cache exists
	exists, err := cm.Exists(ctx, key)
	if err != nil {
		return err
	}
	
	if exists {
		return nil // Cache is already warm
	}
	
	// Load data
	data, err := loader()
	if err != nil {
		return fmt.Errorf("failed to load data for cache warming: %w", err)
	}
	
	// Cache the data
	if err := cm.Set(ctx, key, data, ttl); err != nil {
		return err
	}
	
	if cm.config.EnableLogging {
		log.Printf("🔥 Warmed cache: %s", key)
	}
	
	// Publish cache warmed event
	if cm.eventBus != nil {
		event := events.Event{
			Type:   events.CacheWarmed,
			Source: "cache-manager",
			Data: map[string]interface{}{
				"key": key,
			},
		}
		cm.eventBus.Publish(ctx, event)
	}
	
	return nil
}

// Batch operations
func (cm *CacheManager) MSet(ctx context.Context, pairs map[string]interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}
	
	// Redis MSet doesn't support TTL, so we need to set each key individually
	for key, value := range pairs {
		if err := cm.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	
	return nil
}

// Event-driven cache invalidation
func (cm *CacheManager) registerEventHandlers() {
	if cm.eventBus == nil {
		return
	}
	
	// User events
	cm.eventBus.RegisterHandler(events.UserUpdated, cm.handleUserUpdated)
	cm.eventBus.RegisterHandler(events.UserDeleted, cm.handleUserDeleted)
	cm.eventBus.RegisterHandler(events.UserPasswordChanged, cm.handleUserPasswordChanged)
	
	// Auth events
	cm.eventBus.RegisterHandler(events.TokenRevoked, cm.handleTokenRevoked)
	cm.eventBus.RegisterHandler(events.SessionExpired, cm.handleSessionExpired)
	
	log.Println("✅ Cache manager event handlers registered")
}

func (cm *CacheManager) handleUserUpdated(ctx context.Context, event events.Event) error {
	if userID, ok := event.Metadata["user_id"].(string); ok {
		return cm.InvalidateUser(ctx, userID)
	}
	return nil
}

func (cm *CacheManager) handleUserDeleted(ctx context.Context, event events.Event) error {
	if userID, ok := event.Metadata["user_id"].(string); ok {
		return cm.InvalidateUser(ctx, userID)
	}
	return nil
}

func (cm *CacheManager) handleUserPasswordChanged(ctx context.Context, event events.Event) error {
	if userID, ok := event.Metadata["user_id"].(string); ok {
		// Invalidate user and all sessions
		if err := cm.InvalidateUser(ctx, userID); err != nil {
			return err
		}
		// Could also invalidate all sessions for this user
	}
	return nil
}

func (cm *CacheManager) handleTokenRevoked(ctx context.Context, event events.Event) error {
	if data, ok := event.Data.(map[string]interface{}); ok {
		if tokenID, ok := data["token_id"].(string); ok {
			if expiresAtStr, ok := data["expires_at"].(string); ok {
				if expiresAt, err := time.Parse(time.RFC3339, expiresAtStr); err == nil {
					return cm.BlacklistToken(ctx, tokenID, expiresAt)
				}
			}
		}
	}
	return nil
}

func (cm *CacheManager) handleSessionExpired(ctx context.Context, event events.Event) error {
	if sessionID, ok := event.Metadata["session_id"].(string); ok {
		return cm.InvalidateSession(ctx, sessionID)
	}
	return nil
}

// Cache statistics (if metrics enabled)
type CacheStats struct {
	Hits         int64 `json:"hits"`
	Misses       int64 `json:"misses"`
	Sets         int64 `json:"sets"`
	Deletes      int64 `json:"deletes"`
	Invalidations int64 `json:"invalidations"`
	HitRate      float64 `json:"hit_rate"`
}

func (cm *CacheManager) GetStats(ctx context.Context) (*CacheStats, error) {
	if !cm.config.EnableMetrics {
		return nil, fmt.Errorf("metrics not enabled")
	}
	
	// This would typically collect metrics from Redis or internal counters
	stats := &CacheStats{
		// Implementation would query actual metrics
	}
	
	if stats.Hits+stats.Misses > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.Hits+stats.Misses)
	}
	
	return stats, nil
}

// Health check
func (cm *CacheManager) HealthCheck(ctx context.Context) error {
	return cm.redis.HealthCheck(ctx)
}

// Default configurations for different environments
func DefaultConfig() Config {
	return Config{
		DefaultTTL:    5 * time.Minute,
		UserTTL:       10 * time.Minute,
		SessionTTL:    30 * time.Minute,
		TokenTTL:      15 * time.Minute,
		EnableMetrics: true,
		EnableLogging: true,
	}
}

func ProductionConfig() Config {
	return Config{
		DefaultTTL:    10 * time.Minute,
		UserTTL:       30 * time.Minute,
		SessionTTL:    60 * time.Minute,
		TokenTTL:      15 * time.Minute,
		EnableMetrics: true,
		EnableLogging: false, // Disable verbose logging in production
	}
}