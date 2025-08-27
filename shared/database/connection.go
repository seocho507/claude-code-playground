package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectionConfig contains database connection configuration
type ConnectionConfig struct {
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	Timezone        string
}

// RetryConfig contains retry logic configuration
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retry attempts
	InitialInterval time.Duration // Initial retry interval
	MaxInterval     time.Duration // Maximum retry interval
	Multiplier      float64       // Backoff multiplier
	MaxElapsedTime  time.Duration // Maximum total elapsed time for all retries
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      5,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		MaxElapsedTime:  5 * time.Minute,
	}
}

// ConnectWithRetry establishes a database connection with retry logic and exponential backoff
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - dbConfig: Database connection configuration
//   - retryConfig: Retry behavior configuration
//
// Returns:
//   - *gorm.DB: Successfully connected database instance
//   - error: Connection error if all retry attempts failed
//
// Features:
//   - Exponential backoff with configurable multiplier
//   - Maximum retry attempts and elapsed time limits
//   - Context-aware cancellation
//   - Comprehensive connection health verification
//   - Automatic connection pool configuration
func ConnectWithRetry(ctx context.Context, dbConfig ConnectionConfig, retryConfig RetryConfig) (*gorm.DB, error) {
	// Build PostgreSQL DSN
	timezone := dbConfig.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		dbConfig.Host, dbConfig.Port, dbConfig.User, dbConfig.Password,
		dbConfig.Name, dbConfig.SSLMode, timezone,
	)

	var db *gorm.DB
	var lastErr error
	
	interval := retryConfig.InitialInterval
	startTime := time.Now()
	
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("database connection cancelled: %w", ctx.Err())
		default:
		}
		
		// Check if maximum elapsed time exceeded
		if time.Since(startTime) > retryConfig.MaxElapsedTime {
			return nil, fmt.Errorf("database connection timeout after %v: %w", retryConfig.MaxElapsedTime, lastErr)
		}
		
		log.Printf("Attempting database connection (attempt %d/%d)...", attempt+1, retryConfig.MaxRetries+1)
		
		// Attempt connection
		gormConfig := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
			PrepareStmt: true,
		}
		
		db, lastErr = gorm.Open(postgres.Open(dsn), gormConfig)
		if lastErr == nil {
			// Connection successful, configure connection pool
			sqlDB, err := db.DB()
			if err != nil {
				lastErr = fmt.Errorf("failed to get underlying sql.DB: %w", err)
			} else {
				// Configure connection pool
				if dbConfig.MaxOpenConns > 0 {
					sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
				} else {
					sqlDB.SetMaxOpenConns(25) // Default
				}
				
				if dbConfig.MaxIdleConns > 0 {
					sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
				} else {
					sqlDB.SetMaxIdleConns(10) // Default
				}
				
				if dbConfig.ConnMaxLifetime > 0 {
					sqlDB.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)
				}
				
				// Test connection with context
				pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err = sqlDB.PingContext(pingCtx)
				cancel()
				
				if err != nil {
					lastErr = fmt.Errorf("database ping failed: %w", err)
				} else {
					log.Printf("✅ Database connected successfully on attempt %d", attempt+1)
					return db, nil
				}
			}
		}
		
		// Log the error
		log.Printf("❌ Database connection failed (attempt %d/%d): %v", attempt+1, retryConfig.MaxRetries+1, lastErr)
		
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
			return nil, fmt.Errorf("database connection cancelled during retry: %w", ctx.Err())
		case <-time.After(interval):
			// Continue to next attempt
		}
		
		interval = nextInterval
	}
	
	return nil, fmt.Errorf("database connection failed after %d attempts: %w", retryConfig.MaxRetries+1, lastErr)
}

// HealthCheck performs a database health check with timeout
func HealthCheck(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}
	
	return nil
}

// Close safely closes the database connection
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	return sqlDB.Close()
}