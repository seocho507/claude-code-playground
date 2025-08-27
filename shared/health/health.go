package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check represents a health check function
type Check func(ctx context.Context) CheckResult

// CheckResult contains the result of a health check
type CheckResult struct {
	Status    Status        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HealthChecker manages health checks for a service
type HealthChecker struct {
	serviceName string
	checks      map[string]Check
	timeout     time.Duration
	mu          sync.RWMutex
}

// New creates a new health checker
func New(serviceName string, timeout time.Duration) *HealthChecker {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &HealthChecker{
		serviceName: serviceName,
		checks:      make(map[string]Check),
		timeout:     timeout,
	}
}

// AddCheck adds a health check
func (h *HealthChecker) AddCheck(name string, check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// RemoveCheck removes a health check
func (h *HealthChecker) RemoveCheck(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checks, name)
}

// OverallHealth contains the overall health status and individual check results
type OverallHealth struct {
	Service   string                   `json:"service"`
	Status    Status                   `json:"status"`
	Timestamp time.Time                `json:"timestamp"`
	Duration  time.Duration            `json:"duration"`
	Checks    map[string]CheckResult   `json:"checks"`
	Metadata  map[string]interface{}   `json:"metadata,omitempty"`
}

// CheckHealth performs all health checks and returns the overall health
func (h *HealthChecker) CheckHealth(ctx context.Context) OverallHealth {
	start := time.Now()
	
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	h.mu.RLock()
	checksToRun := make(map[string]Check, len(h.checks))
	for name, check := range h.checks {
		checksToRun[name] = check
	}
	h.mu.RUnlock()

	results := make(map[string]CheckResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Run all checks concurrently
	for name, check := range checksToRun {
		wg.Add(1)
		go func(checkName string, checkFunc Check) {
			defer wg.Done()
			result := h.runSingleCheck(ctx, checkFunc)
			
			mu.Lock()
			results[checkName] = result
			mu.Unlock()
		}(name, check)
	}

	wg.Wait()

	// Determine overall status
	overallStatus := h.calculateOverallStatus(results)

	return OverallHealth{
		Service:   h.serviceName,
		Status:    overallStatus,
		Timestamp: start,
		Duration:  time.Since(start),
		Checks:    results,
	}
}

// runSingleCheck runs a single health check with timeout protection
func (h *HealthChecker) runSingleCheck(ctx context.Context, check Check) CheckResult {
	start := time.Now()
	
	// Use a channel to capture the result or timeout
	resultChan := make(chan CheckResult, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- CheckResult{
					Status:    StatusUnhealthy,
					Error:     fmt.Sprintf("panic during health check: %v", r),
					Duration:  time.Since(start),
					Timestamp: start,
				}
			}
		}()
		
		result := check(ctx)
		result.Duration = time.Since(start)
		result.Timestamp = start
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		return CheckResult{
			Status:    StatusUnhealthy,
			Error:     "health check timed out",
			Duration:  time.Since(start),
			Timestamp: start,
		}
	}
}

// calculateOverallStatus determines the overall health status based on individual check results
func (h *HealthChecker) calculateOverallStatus(results map[string]CheckResult) Status {
	if len(results) == 0 {
		return StatusHealthy
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

// Handler returns a Gin handler for health checks
func (h *HealthChecker) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		health := h.CheckHealth(c.Request.Context())
		
		statusCode := http.StatusOK
		if health.Status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		} else if health.Status == StatusDegraded {
			statusCode = http.StatusOK // Still return 200 for degraded
		}

		c.JSON(statusCode, health)
	}
}

// Predefined health checks

// DatabaseCheck creates a health check for a database connection
func DatabaseCheck(db *gorm.DB) Check {
	return func(ctx context.Context) CheckResult {
		if db == nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  "database connection is nil",
			}
		}

		sqlDB, err := db.DB()
		if err != nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  fmt.Sprintf("failed to get underlying sql.DB: %v", err),
			}
		}

		if err := sqlDB.PingContext(ctx); err != nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  fmt.Sprintf("database ping failed: %v", err),
			}
		}

		// Get database stats
		stats := sqlDB.Stats()
		metadata := map[string]interface{}{
			"open_connections": stats.OpenConnections,
			"in_use":          stats.InUse,
			"idle":            stats.Idle,
			"wait_count":      stats.WaitCount,
			"wait_duration":   stats.WaitDuration.String(),
		}

		return CheckResult{
			Status:   StatusHealthy,
			Message:  "database connection is healthy",
			Metadata: metadata,
		}
	}
}

// RedisCheck creates a health check for a Redis connection
func RedisCheck(client *redis.Client) Check {
	return func(ctx context.Context) CheckResult {
		if client == nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  "redis client is nil",
			}
		}

		pong, err := client.Ping(ctx).Result()
		if err != nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  fmt.Sprintf("redis ping failed: %v", err),
			}
		}

		// Get Redis info
		poolStats := client.PoolStats()
		metadata := map[string]interface{}{
			"ping_response":    pong,
			"total_conns":     poolStats.TotalConns,
			"idle_conns":      poolStats.IdleConns,
			"stale_conns":     poolStats.StaleConns,
		}

		return CheckResult{
			Status:   StatusHealthy,
			Message:  "redis connection is healthy",
			Metadata: metadata,
		}
	}
}

// HTTPCheck creates a health check for an HTTP endpoint
func HTTPCheck(url string, timeout time.Duration) Check {
	return func(ctx context.Context) CheckResult {
		client := &http.Client{Timeout: timeout}
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  fmt.Sprintf("failed to create request: %v", err),
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  fmt.Sprintf("HTTP request failed: %v", err),
			}
		}
		defer resp.Body.Close()

		metadata := map[string]interface{}{
			"status_code": resp.StatusCode,
			"url":        url,
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return CheckResult{
				Status:   StatusHealthy,
				Message:  fmt.Sprintf("HTTP endpoint is healthy (status: %d)", resp.StatusCode),
				Metadata: metadata,
			}
		}

		return CheckResult{
			Status:   StatusUnhealthy,
			Error:    fmt.Sprintf("HTTP endpoint returned status %d", resp.StatusCode),
			Metadata: metadata,
		}
	}
}

// CustomCheck creates a custom health check
func CustomCheck(name string, checkFunc func(context.Context) error) Check {
	return func(ctx context.Context) CheckResult {
		if err := checkFunc(ctx); err != nil {
			return CheckResult{
				Status: StatusUnhealthy,
				Error:  err.Error(),
			}
		}

		return CheckResult{
			Status:  StatusHealthy,
			Message: fmt.Sprintf("%s check passed", name),
		}
	}
}