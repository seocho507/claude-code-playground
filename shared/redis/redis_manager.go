package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config contains Redis connection configuration
type Config struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// RedisManager provides centralized Redis operations with namespacing
type RedisManager struct {
	client    *redis.Client
	namespace string
}

// NewRedisManager creates a centralized Redis manager with namespacing
func NewRedisManager(client *redis.Client, namespace string) *RedisManager {
	return &RedisManager{
		client:    client,
		namespace: namespace,
	}
}

// Key generates a namespaced key
func (r *RedisManager) Key(key string) string {
	return fmt.Sprintf("%s:%s", r.namespace, key)
}

// PatternKey generates a namespaced pattern for scanning
func (r *RedisManager) PatternKey(pattern string) string {
	return fmt.Sprintf("%s:%s", r.namespace, pattern)
}

// Cache operations with automatic JSON serialization
func (r *RedisManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	
	return r.client.Set(ctx, r.Key(key), data, ttl).Err()
}

func (r *RedisManager) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := r.client.Get(ctx, r.Key(key)).Bytes()
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, dest)
}

func (r *RedisManager) Delete(ctx context.Context, keys ...string) error {
	namespacedKeys := make([]string, len(keys))
	for i, key := range keys {
		namespacedKeys[i] = r.Key(key)
	}
	
	return r.client.Del(ctx, namespacedKeys...).Err()
}

func (r *RedisManager) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, r.Key(key)).Result()
	return count > 0, err
}

func (r *RedisManager) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, r.Key(key), ttl).Err()
}

// Hash operations
func (r *RedisManager) HSet(ctx context.Context, key string, field string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	
	return r.client.HSet(ctx, r.Key(key), field, data).Err()
}

func (r *RedisManager) HGet(ctx context.Context, key string, field string, dest interface{}) error {
	data, err := r.client.HGet(ctx, r.Key(key), field).Bytes()
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, dest)
}

func (r *RedisManager) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, r.Key(key)).Result()
}

func (r *RedisManager) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, r.Key(key), fields...).Err()
}

// Set operations
func (r *RedisManager) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, r.Key(key), members...).Err()
}

func (r *RedisManager) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, r.Key(key)).Result()
}

func (r *RedisManager) SRem(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SRem(ctx, r.Key(key), members...).Err()
}

// List operations
func (r *RedisManager) LPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.LPush(ctx, r.Key(key), values...).Err()
}

func (r *RedisManager) RPop(ctx context.Context, key string) (string, error) {
	return r.client.RPop(ctx, r.Key(key)).Result()
}

func (r *RedisManager) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, r.Key(key), start, stop).Result()
}

// Pub/Sub operations (namespace-aware)
func (r *RedisManager) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	return r.client.Publish(ctx, r.Key(channel), data).Err()
}

func (r *RedisManager) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	namespacedChannels := make([]string, len(channels))
	for i, channel := range channels {
		namespacedChannels[i] = r.Key(channel)
	}
	
	return r.client.Subscribe(ctx, namespacedChannels...)
}

// Pattern subscribe for event wildcards
func (r *RedisManager) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	namespacedPatterns := make([]string, len(patterns))
	for i, pattern := range patterns {
		namespacedPatterns[i] = r.PatternKey(pattern)
	}
	
	return r.client.PSubscribe(ctx, namespacedPatterns...)
}

// Lock operations for distributed locking
func (r *RedisManager) AcquireLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, r.Key(fmt.Sprintf("lock:%s", key)), value, ttl).Result()
}

func (r *RedisManager) ReleaseLock(ctx context.Context, key string, value string) error {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	
	return r.client.Eval(ctx, script, []string{r.Key(fmt.Sprintf("lock:%s", key))}, value).Err()
}

// Rate limiting operations
func (r *RedisManager) RateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	pipe := r.client.Pipeline()
	rateLimitKey := r.Key(fmt.Sprintf("rate_limit:%s", key))
	
	// Sliding window rate limiting
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()
	
	// Remove old entries
	pipe.ZRemRangeByScore(ctx, rateLimitKey, "0", fmt.Sprintf("%d", windowStart))
	
	// Count current requests
	pipe.ZCard(ctx, rateLimitKey)
	
	// Add current request
	pipe.ZAdd(ctx, rateLimitKey, redis.Z{Score: float64(now), Member: now})
	
	// Set expiration
	pipe.Expire(ctx, rateLimitKey, window)
	
	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	
	count := results[1].(*redis.IntCmd).Val()
	return count < int64(limit), nil
}

// Session operations
func (r *RedisManager) SetSession(ctx context.Context, sessionID string, data interface{}, ttl time.Duration) error {
	return r.Set(ctx, fmt.Sprintf("session:%s", sessionID), data, ttl)
}

func (r *RedisManager) GetSession(ctx context.Context, sessionID string, dest interface{}) error {
	return r.Get(ctx, fmt.Sprintf("session:%s", sessionID), dest)
}

func (r *RedisManager) DeleteSession(ctx context.Context, sessionID string) error {
	return r.Delete(ctx, fmt.Sprintf("session:%s", sessionID))
}

func (r *RedisManager) RefreshSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	return r.Expire(ctx, fmt.Sprintf("session:%s", sessionID), ttl)
}

// Bulk operations
func (r *RedisManager) MSet(ctx context.Context, pairs map[string]interface{}) error {
	args := make([]interface{}, 0, len(pairs)*2)
	
	for key, value := range pairs {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
		}
		args = append(args, r.Key(key), data)
	}
	
	return r.client.MSet(ctx, args...).Err()
}

func (r *RedisManager) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	namespacedKeys := make([]string, len(keys))
	for i, key := range keys {
		namespacedKeys[i] = r.Key(key)
	}
	
	return r.client.MGet(ctx, namespacedKeys...).Result()
}

// Cleanup operations
func (r *RedisManager) FlushNamespace(ctx context.Context) error {
	iter := r.client.Scan(ctx, 0, r.PatternKey("*"), 0).Iterator()
	
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		
		// Delete in batches of 100
		if len(keys) >= 100 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}
	
	// Delete remaining keys
	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}
	
	return iter.Err()
}

// Health check
func (r *RedisManager) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Get underlying client for advanced operations
func (r *RedisManager) Client() *redis.Client {
	return r.client
}

// RedisManagerFactory creates Redis managers for different services
type RedisManagerFactory struct {
	client *redis.Client
}

func NewRedisManagerFactory(client *redis.Client) *RedisManagerFactory {
	return &RedisManagerFactory{client: client}
}

func (f *RedisManagerFactory) ForService(serviceName string) *RedisManager {
	return NewRedisManager(f.client, serviceName)
}

// Common Redis managers for different use cases
func (f *RedisManagerFactory) Cache() *RedisManager {
	return NewRedisManager(f.client, "cache")
}

func (f *RedisManagerFactory) Session() *RedisManager {
	return NewRedisManager(f.client, "session")
}

func (f *RedisManagerFactory) Events() *RedisManager {
	return NewRedisManager(f.client, "events")
}

func (f *RedisManagerFactory) Locks() *RedisManager {
	return NewRedisManager(f.client, "locks")
}

func (f *RedisManagerFactory) RateLimit() *RedisManager {
	return NewRedisManager(f.client, "rate_limit")
}