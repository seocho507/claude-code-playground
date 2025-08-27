package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
)

type Config struct {
	Server        ServerConfig     `toml:"server"`
	Database      DatabaseConfig   `toml:"database"`
	Redis         RedisConfig      `toml:"redis"`
	JWT           JWTConfig        `toml:"jwt"`
	Logging       LoggingConfig    `toml:"logging"`
	Metrics       MetricsConfig    `toml:"metrics"`
	Tracing       TracingConfig    `toml:"tracing"`
	Security      SecurityConfig   `toml:"security"`
	Email         EmailConfig      `toml:"email"`
	CORS          CORSConfig       `toml:"cors"`
	Health        HealthConfig     `toml:"health"`
	// OAuth2        OAuth2Config     `toml:"oauth2"` // Temporarily disabled for debugging
}

type ServerConfig struct {
	Host            string        `toml:"host"`
	Port            string        `toml:"port"`
	ReadTimeout     time.Duration `toml:"read_timeout"`
	WriteTimeout    time.Duration `toml:"write_timeout"`
	IdleTimeout     time.Duration `toml:"idle_timeout"`
	ShutdownTimeout time.Duration `toml:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Host            string        `toml:"host"`
	Port            string        `toml:"port"`
	Name            string        `toml:"name"`
	User            string        `toml:"user"`
	Password        string        `toml:"password"`
	SSLMode         string        `toml:"ssl_mode"`
	MaxOpenConns    int           `toml:"max_open_conns"`
	MaxIdleConns    int           `toml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `toml:"conn_max_lifetime"`
	MigrationPath   string        `toml:"migration_path"`
}

type RedisConfig struct {
	URL           string        `toml:"url"`
	Password      string        `toml:"password"`
	DB            int           `toml:"db"`
	MaxRetries    int           `toml:"max_retries"`
	PoolSize      int           `toml:"pool_size"`
	MinIdleConns  int           `toml:"min_idle_conns"`
	DialTimeout   time.Duration `toml:"dial_timeout"`
	ReadTimeout   time.Duration `toml:"read_timeout"`
	WriteTimeout  time.Duration `toml:"write_timeout"`
	PoolTimeout   time.Duration `toml:"pool_timeout"`
	IdleTimeout   time.Duration `toml:"idle_timeout"`
}

type JWTConfig struct {
	AccessSecret  string `toml:"access_secret"`
	RefreshSecret string `toml:"refresh_secret"`
	Issuer        string `toml:"issuer"`
	AccessExpiry  string `toml:"access_expiry"`
	RefreshExpiry string `toml:"refresh_expiry"`
	Algorithm     string `toml:"algorithm"`
}

type OAuth2Config struct {
	Google   OAuth2Provider `toml:"google"`
	GitHub   OAuth2Provider `toml:"github"`
	Facebook OAuth2Provider `toml:"facebook"`
}

type OAuth2Provider struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
	RedirectURL  string `toml:"redirect_url"`
	Enabled      bool   `toml:"enabled"`
}

type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	Output string `toml:"output"`
}

type MetricsConfig struct {
	Enabled bool   `toml:"enabled"`
	Path    string `toml:"path"`
	Port    string `toml:"port"`
}

type TracingConfig struct {
	Enabled        bool    `toml:"enabled"`
	ServiceName    string  `toml:"service_name"`
	JaegerEndpoint string  `toml:"jaeger_endpoint"`
	SampleRate     float64 `toml:"sample_rate"`
}

// Rate limiting is handled by Traefik Gateway - no service-level config needed

type SecurityConfig struct {
	BcryptCost              int           `toml:"bcrypt_cost"`
	SessionTimeout          time.Duration `toml:"session_timeout"`
	MaxSessionsPerUser      int           `toml:"max_sessions_per_user"`
	PasswordMinLength       int           `toml:"password_min_length"`
	PasswordRequireSpecial  bool          `toml:"password_require_special"`
	PasswordRequireNumber   bool          `toml:"password_require_number"`
	PasswordRequireUppercase bool         `toml:"password_require_uppercase"`
}

type EmailConfig struct {
	SMTPHost    string `toml:"smtp_host"`
	SMTPPort    int    `toml:"smtp_port"`
	Username    string `toml:"username"`
	Password    string `toml:"password"`
	FromAddress string `toml:"from_address"`
	FromName    string `toml:"from_name"`
}

type CORSConfig struct {
	AllowedOrigins   []string `toml:"allowed_origins"`
	AllowedMethods   []string `toml:"allowed_methods"`
	AllowedHeaders   []string `toml:"allowed_headers"`
	ExposedHeaders   []string `toml:"exposed_headers"`
	AllowCredentials bool     `toml:"allow_credentials"`
	MaxAge           int      `toml:"max_age"`
}

type HealthConfig struct {
	CheckInterval time.Duration `toml:"check_interval"`
	Timeout       time.Duration `toml:"timeout"`
}

// Load reads and parses environment-specific TOML configuration file with comprehensive fallback logic
//
// Purpose: Centralized configuration loading with environment-based file selection and .env integration
// Parameters:
//   - environment (string): Environment selector ("local" or "prod")
//     - "local": Loads config-local.toml with development settings + .env.local
//     - "prod": Loads config.toml with production settings + .env
// Configuration Strategy:
//   1. Load .env file first (environment variables)
//   2. Load TOML configuration file (with env variable substitution)
//   3. Environment variables override TOML settings
// Configuration Files:
//   Local Environment:
//     - .env.local: Development environment variables
//     - config-local.toml: Local TOML with ${VAR:default} patterns
//   Production Environment:
//     - .env: Production environment variables
//     - config.toml: Production TOML with ${VAR:default} patterns
// Configuration Sections Loaded:
//   - Server: HTTP server settings (host, port, timeouts)
//   - Database: PostgreSQL connection and pool configuration
//   - Redis: Cache connection settings and pool configuration
//   - JWT: Token secrets, expiration times, signing algorithm (HS256)
//   - Security: bcrypt cost, session limits, password policies
//   - CORS: Cross-origin policies for web client integration
//   - OAuth2: External provider credentials (Google, GitHub, Facebook)
//   - Logging: Log level, format, output destination
//   - Metrics: Prometheus configuration
//   - Tracing: Jaeger distributed tracing settings
//   - RateLimiting: Login attempt limits and lockout policies
//   - Email: SMTP configuration for notifications
//   - Health: Health check intervals and timeouts
// File Resolution Strategy:
//   1. Service-specific config directory (config/)
//   2. Current working directory config
//   3. Executable directory config
//   4. Fallback to internal config directory
//   5. Legacy parent directory locations
//   6. Current directory as last resort
// Error Handling: 
//   - Panics on missing configuration file (fail-fast approach)
//   - Panics on TOML parsing errors with detailed error message
//   - File path resolution errors are handled gracefully with fallbacks
// Returns: *Config struct containing all parsed application settings
// Side Effects: Sets global application configuration state
// Usage: Called once during application initialization with environment flag
func Load(environment string) (*Config, error) {
	// Step 1: Load .env file based on environment
	if err := loadEnvFile(environment); err != nil {
		// .env file is optional, just log and continue
		fmt.Printf("Warning: Could not load .env file: %v\n", err)
	}

	// Step 2: Get the executable directory for path resolution
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)
	
	// Step 3: Select TOML config file based on environment
	var configFileName string
	if environment == "local" {
		configFileName = "config-local.toml"
	} else {
		configFileName = "config.toml"
	}

	// Step 4: Try different possible locations for the config file
	configPaths := []string{
		"config/" + configFileName,                                     // auth-service/config/ (preferred)
		filepath.Join(".", "config", configFileName),                  // ./config/
		filepath.Join(execDir, "config", configFileName),              // executable directory/config/
		filepath.Join("internal", "config", configFileName),           // internal/config/ (fallback)
		filepath.Join(execDir, "internal", "config", configFileName),  // executable directory/internal/config/
		"../config/" + configFileName,                                 // parent directory (legacy)
		"../../config/" + configFileName,                              // grandparent directory (legacy)
		configFileName,                                                 // current directory (last resort)
	}
	
	// Step 5: Find first existing config file from the path list
	var configPath string
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}
	
	// Return error if no configuration file is found
	if configPath == "" {
		return nil, fmt.Errorf("could not find %s configuration file in any of the expected locations", configFileName)
	}
	
	// Step 6: Parse TOML configuration file into Config struct
	var config Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}
	
	// Step 7: Apply default values for missing fields
	setDefaults(&config)
	
	// Step 8: Expand environment variables in configuration (${VAR:default} patterns)
	// Temporarily disabled for debugging
	// expandEnvironmentVariables(&config)
	
	// Step 9: Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return &config, nil
}

// expandEnvironmentVariables recursively expands environment variables in configuration strings
//
// Purpose: Replaces ${ENV_VAR} or ${ENV_VAR:default_value} patterns with actual environment variable values
// Parameters:
//   - v (interface{}): Configuration structure to process (passed by reference)
// Environment Variable Patterns Supported:
//   - ${ENV_VAR}: Replaces with environment variable value, empty string if not set
//   - ${ENV_VAR:default}: Replaces with environment variable value, or default if not set
//   - ${ENV_VAR:}: Replaces with environment variable value, or empty string if not set
// Processing Strategy:
//   - Uses reflection to traverse all struct fields recursively
//   - Processes string fields for environment variable expansion
//   - Handles nested structs, slices, and pointer types
//   - Preserves non-string field types unchanged
// Security: Only processes string fields to prevent type confusion attacks
// Performance: Processes configuration once during application startup
// Usage: Called automatically during configuration loading
func expandEnvironmentVariables(v interface{}) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return
	}
	
	expandValue(rv.Elem())
}

// expandValue recursively processes reflect.Value for environment variable expansion
func expandValue(rv reflect.Value) {
	switch rv.Kind() {
	case reflect.Struct:
		// Process all fields in struct
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Field(i)
			if field.CanSet() {
				expandValue(field)
			}
		}
	case reflect.Slice:
		// Process all elements in slice
		for i := 0; i < rv.Len(); i++ {
			expandValue(rv.Index(i))
		}
	case reflect.Ptr:
		// Process pointer target if not nil
		if !rv.IsNil() {
			expandValue(rv.Elem())
		}
	case reflect.String:
		// Expand environment variables in string values
		if rv.CanSet() {
			expanded := expandString(rv.String())
			rv.SetString(expanded)
		}
	}
}

// expandString processes individual string for environment variable patterns
//
// Purpose: Replaces ${VAR} and ${VAR:default} patterns with environment variable values
// Parameters:
//   - s (string): Input string that may contain environment variable references
// Returns:
//   - string: String with environment variables expanded
// Supported Patterns:
//   - ${VAR}: Replaced with os.Getenv("VAR"), empty if not set
//   - ${VAR:default}: Replaced with os.Getenv("VAR"), or "default" if not set
//   - ${VAR:}: Replaced with os.Getenv("VAR"), or empty string if not set
// Edge Cases:
//   - Malformed patterns (missing }) are left unchanged
//   - Nested variables are not supported for security
//   - Case-sensitive variable names
// Security: No shell execution, only environment variable lookup
func expandString(s string) string {
	// Find all ${...} patterns
	for {
		start := strings.Index(s, "${")
		if start == -1 {
			break
		}
		
		end := strings.Index(s[start:], "}")
		if end == -1 {
			break
		}
		end += start
		
		// Extract variable reference
		varRef := s[start+2 : end]
		var varName, defaultValue string
		
		// Check for default value pattern ${VAR:default}
		if colonIndex := strings.Index(varRef, ":"); colonIndex != -1 {
			varName = varRef[:colonIndex]
			defaultValue = varRef[colonIndex+1:]
		} else {
			varName = varRef
		}
		
		// Get environment variable value
		envValue := os.Getenv(varName)
		if envValue == "" {
			envValue = defaultValue
		}
		
		// Replace the pattern with the value
		s = s[:start] + envValue + s[end+1:]
	}
	
	return s
}

// setDefaults applies default values to configuration
func setDefaults(cfg *Config) {
	// Server defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8081"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = 30 * time.Second
	}

	// Database defaults
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 10
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		cfg.Database.ConnMaxLifetime = time.Hour
	}

	// Redis defaults
	if cfg.Redis.DB == 0 {
		cfg.Redis.DB = 0 // auth-service uses DB 0, user-service uses DB 1
	}
	if cfg.Redis.MaxRetries == 0 {
		cfg.Redis.MaxRetries = 3
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 10
	}

	// JWT defaults
	if cfg.JWT.Algorithm == "" {
		cfg.JWT.Algorithm = "HS256"
	}
	if cfg.JWT.AccessExpiry == "" {
		cfg.JWT.AccessExpiry = "15m"
	}
	if cfg.JWT.RefreshExpiry == "" {
		cfg.JWT.RefreshExpiry = "168h" // 7 days
	}

	// Security defaults
	if cfg.Security.BcryptCost == 0 {
		cfg.Security.BcryptCost = 12
	}
	if cfg.Security.MaxSessionsPerUser == 0 {
		cfg.Security.MaxSessionsPerUser = 10
	}
	if cfg.Security.PasswordMinLength == 0 {
		cfg.Security.PasswordMinLength = 8
	}
}

// loadEnvFile loads the appropriate .env file based on environment
func loadEnvFile(environment string) error {
	// Determine .env file based on environment
	var envFile string
	if environment == "local" {
		envFile = ".env.local"
	} else {
		envFile = ".env"
	}

	// Try to find .env file in various locations
	envPaths := []string{
		envFile,                    // current directory
		"../" + envFile,            // parent directory
		"../../" + envFile,         // grandparent directory
		"../../../" + envFile,      // great-grandparent (for nested service structure)
	}

	var envPath string
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			envPath = path
			break
		}
	}

	if envPath == "" {
		return fmt.Errorf("could not find %s file in expected locations", envFile)
	}

	// Load the .env file
	if err := godotenv.Load(envPath); err != nil {
		return fmt.Errorf("error loading %s: %w", envPath, err)
	}

	fmt.Printf("✅ Loaded environment variables from: %s\n", envPath)
	return nil
}

// parseBool safely converts string to boolean with fallback
func parseBool(s string, fallback bool) bool {
	if s == "" {
		return fallback
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return fallback
}

// parseInt safely converts string to int with fallback
func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return fallback
}

// parseFloat safely converts string to float64 with fallback
func parseFloat(s string, fallback float64) float64 {
	if s == "" {
		return fallback
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return fallback
}

func validate(cfg *Config) error {
	// Validate required fields
	if cfg.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if cfg.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}

	if cfg.Database.User == "" {
		return fmt.Errorf("database user is required")
	}

	if cfg.JWT.AccessSecret == "" {
		return fmt.Errorf("JWT access secret is required")
	}

	// Validate security settings
	if cfg.Security.BcryptCost < 4 || cfg.Security.BcryptCost > 31 {
		return fmt.Errorf("bcrypt cost must be between 4 and 31")
	}

	if cfg.Security.PasswordMinLength < 6 {
		return fmt.Errorf("password minimum length must be at least 6")
	}

	return nil
}

