package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig contains Redis connection configuration
type RedisConfig struct {
	URL          string
	Password     string
	DB           int
	MaxRetries   int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
	IdleTimeout  time.Duration
}

// ConnectRedisWithRetry establishes a Redis connection with retry logic and exponential backoff
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - redisConfig: Redis connection configuration
//   - retryConfig: Retry behavior configuration
//
// Returns:
//   - *redis.Client: Successfully connected Redis client
//   - error: Connection error if all retry attempts failed
//
// Features:
//   - Exponential backoff with configurable multiplier
//   - Maximum retry attempts and elapsed time limits
//   - Context-aware cancellation
//   - Comprehensive connection health verification
//   - Automatic connection pool configuration
func ConnectRedisWithRetry(ctx context.Context, redisConfig RedisConfig, retryConfig RetryConfig) (*redis.Client, error) {
	// Parse Redis URL with connection parameters
	opt, err := redis.ParseURL(redisConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Configure authentication and database selection
	if redisConfig.Password != "" {
		opt.Password = redisConfig.Password
	}
	opt.DB = redisConfig.DB

	// Configure connection pool and timeouts
	if redisConfig.MaxRetries > 0 {
		opt.MaxRetries = redisConfig.MaxRetries
	}
	if redisConfig.PoolSize > 0 {
		opt.PoolSize = redisConfig.PoolSize
	}
	if redisConfig.MinIdleConns > 0 {
		opt.MinIdleConns = redisConfig.MinIdleConns
	}
	if redisConfig.DialTimeout > 0 {
		opt.DialTimeout = redisConfig.DialTimeout
	}
	if redisConfig.ReadTimeout > 0 {
		opt.ReadTimeout = redisConfig.ReadTimeout
	}
	if redisConfig.WriteTimeout > 0 {
		opt.WriteTimeout = redisConfig.WriteTimeout
	}
	if redisConfig.PoolTimeout > 0 {
		opt.PoolTimeout = redisConfig.PoolTimeout
	}
	if redisConfig.IdleTimeout > 0 {
		opt.ConnMaxIdleTime = redisConfig.IdleTimeout
	}

	var client *redis.Client
	var lastErr error
	
	interval := retryConfig.InitialInterval
	startTime := time.Now()
	
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("redis connection cancelled: %w", ctx.Err())
		default:
		}
		
		// Check if maximum elapsed time exceeded
		if time.Since(startTime) > retryConfig.MaxElapsedTime {
			return nil, fmt.Errorf("redis connection timeout after %v: %w", retryConfig.MaxElapsedTime, lastErr)
		}
		
		log.Printf("Attempting Redis connection (attempt %d/%d)...", attempt+1, retryConfig.MaxRetries+1)
		
		// Create Redis client
		client = redis.NewClient(opt)
		
		// Test connection with context
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, lastErr = client.Ping(pingCtx).Result()
		cancel()
		
		if lastErr == nil {
			log.Printf("✅ Redis connected successfully on attempt %d", attempt+1)
			return client, nil
		}
		
		// Close failed client
		client.Close()
		client = nil
		
		// Log the error
		log.Printf("❌ Redis connection failed (attempt %d/%d): %v", attempt+1, retryConfig.MaxRetries+1, lastErr)
		
		// Don't wait after the last attempt
		if attempt == retryConfig.MaxRetries {
			break
		}
		
		// Calculate next retry interval with exponential backoff
		nextInterval := time.Duration(float64(interval) * retryConfig.Multiplier)
		if nextInterval > retryConfig.MaxInterval {
			nextInterval = retryConfig.MaxInterval
		}
		
		log.Printf("⏳ Retrying in %v...", interval)
		
		// Wait for retry interval or context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("redis connection cancelled during retry: %w", ctx.Err())
		case <-time.After(interval):
			// Continue to next attempt
		}
		
		interval = nextInterval
	}
	
	return nil, fmt.Errorf("redis connection failed after %d attempts: %w", retryConfig.MaxRetries+1, lastErr)
}

// RedisHealthCheck performs a Redis health check with timeout
func RedisHealthCheck(ctx context.Context, client *redis.Client) error {
	if client == nil {
		return fmt.Errorf("redis client is nil")
	}
	
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	_, err := client.Ping(pingCtx).Result()
	if err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}
	
	return nil
}

// CloseRedis safely closes the Redis connection
func CloseRedis(client *redis.Client) error {
	if client == nil {
		return nil
	}
	
	return client.Close()
}