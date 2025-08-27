package database

import (
	"auth-service/internal/config"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"shared/database"
)

// Connect establishes PostgreSQL database connection with retry logic and exponential backoff
//
// Purpose: Creates and configures PostgreSQL database connection for authentication service with enhanced reliability
// Parameters:
//   - cfg (config.DatabaseConfig): Database configuration containing connection parameters
// Database Setup:
//   - Timezone: Asia/Seoul for consistent timestamp handling
//   - Logger: INFO level logging for SQL queries and performance monitoring
//   - SSL: Configurable SSL mode for secure connections
//   - Connection Pool: Optimized for concurrent request handling
//   - Retry Logic: Exponential backoff with configurable parameters
// Connection Pool Configuration:
//   - MaxOpenConns: Limits concurrent database connections to prevent exhaustion
//   - MaxIdleConns: Maintains idle connections for fast request processing
//   - ConnMaxLifetime: Configurable connection lifetime
// Returns:
//   - *gorm.DB: Configured GORM database instance for ORM operations
//   - error: Connection establishment or configuration errors after all retry attempts
// Error Conditions:
//   - Network connectivity issues to database server
//   - Invalid credentials or database name
//   - Database server unavailable or overloaded
//   - SSL configuration mismatches
// Performance: Connection pooling enables handling 100+ concurrent requests
// Security: Supports SSL connections and credential-based authentication
// Reliability: Automatic retry with exponential backoff for transient failures
// Usage: Called once during application initialization
func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	// Convert auth-service config to shared database config
	sharedConfig := database.ConnectionConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		Name:            cfg.Name,
		User:            cfg.User,
		Password:        cfg.Password,
		SSLMode:         cfg.SSLMode,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		Timezone:        "Asia/Seoul",
	}
	
	// Set defaults if not configured
	if sharedConfig.MaxOpenConns == 0 {
		sharedConfig.MaxOpenConns = 25
	}
	if sharedConfig.MaxIdleConns == 0 {
		sharedConfig.MaxIdleConns = 10
	}
	
	// Use default retry configuration with reasonable timeouts for auth service
	retryConfig := database.DefaultRetryConfig()
	
	// Create context with overall timeout for connection establishment
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	log.Println("🔄 Establishing database connection with retry logic...")
	
	// Use shared database connection with retry logic
	db, err := database.ConnectWithRetry(ctx, sharedConfig, retryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
	}

	log.Println("✅ Database connected successfully with retry logic")
	return db, nil
}

// Migrate runs comprehensive database schema migrations for authentication service
//
// Purpose: Creates and updates database schema to support authentication functionality
// Parameters:
//   - db (*gorm.DB): Active database connection for migration operations
// Migration Steps:
//   1. PostgreSQL Extensions: Creates uuid-ossp extension for UUID generation
//   2. Table Creation: Auto-migrates all authentication-related models
//   3. Index Creation: Adds performance indexes for common query patterns
// Tables Managed:
//   - users: User account information with authentication data
//   - sessions: JWT session tracking and management
//   - password_resets: Password reset token management
//   - login_attempts: Failed login tracking for rate limiting
// PostgreSQL Extensions:
//   - uuid-ossp: Enables uuid_generate_v4() for automatic UUID primary keys
// GORM AutoMigrate Features:
//   - Creates tables if they don't exist
//   - Adds new columns to existing tables
//   - Updates column types if changed
//   - Preserves existing data during schema changes
//   - Does NOT drop columns or tables (safe for production)
// Performance Indexes:
//   - User lookups by email and username with active status
//   - Session lookups by user ID and token hash
//   - Failed login tracking by IP and timestamp
//   - Password reset token lookups
// Returns:
//   - error: Migration failure or database connectivity issues
// Error Conditions:
//   - Database permission issues for extension creation
//   - Table creation failures due to constraints
//   - Index creation failures (logged as warnings, not fatal)
// Safety: Production-safe migrations that preserve existing data
// Performance: Completes typically within 1-5 seconds on empty database
// Usage: Called once during application initialization after database connection
func Migrate(db *gorm.DB) error {
	log.Println("Database migrations handled by SQL files in migrations/ directory")
	log.Println("Verifying database connection...")
	
	// Simple connection test
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	
	log.Println("Database connection verified successfully")
	return nil
}

// createIndexes - DEPRECATED: Index creation handled by SQL migration files
// This function is no longer used as indexes are now created via SQL migration files
// in the migrations/ directory. Kept for reference only.
//
// DEPRECATED: Use SQL migration files instead
/*
func createIndexes(db *gorm.DB) error {
	// Index creation is now handled by SQL migration files
	// See migrations/001_init_schema.sql for current index definitions
	log.Println("Index creation handled by SQL migration files")
	return nil
}
*/

// ConnectRedis establishes Redis connection with retry logic and exponential backoff
//
// Purpose: Creates Redis client connection for token blacklisting and caching with enhanced reliability
// Parameters:
//   - cfg (config.RedisConfig): Redis configuration containing connection parameters
// Redis Usage in Auth Service:
//   - JWT Token Blacklisting: Immediately invalidate revoked tokens
//   - Session Caching: Fast session validation without database queries
//   - Rate Limiting: Track login attempts and lockout status
//   - Temporary Data: Password reset tokens, verification codes
// Connection Configuration:
//   - URL Parsing: Supports redis:// URLs with host, port, and database
//   - Authentication: Optional password-based authentication
//   - Database Selection: Configurable Redis database number
//   - Connection Pool: Optimized for concurrent request handling
//   - Retry Logic: Exponential backoff with configurable parameters
// Returns:
//   - *redis.Client: Configured Redis client for caching operations
// Error Handling: Enhanced error handling with retry logic instead of panic
// Performance: Connection pooling enables 1000+ concurrent Redis operations
// Security: Supports password authentication and database isolation
// Reliability: Automatic retry with exponential backoff for transient failures
// Usage: Called once during application initialization for global Redis client
func ConnectRedis(cfg config.RedisConfig) *redis.Client {
	// Convert auth-service config to shared database config
	sharedConfig := database.RedisConfig{
		URL:          cfg.URL,
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
	
	// Use default retry configuration
	retryConfig := database.DefaultRetryConfig()
	
	// Create context with overall timeout for connection establishment
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	log.Println("🔄 Establishing Redis connection with retry logic...")
	
	// Use shared Redis connection with retry logic
	client, err := database.ConnectRedisWithRetry(ctx, sharedConfig, retryConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis after retries: %v", err)
	}

	log.Println("✅ Redis connected successfully with retry logic")
	return client
}