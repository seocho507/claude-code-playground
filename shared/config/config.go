package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// BaseConfig contains common configuration fields used across all microservices
type BaseConfig struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	CORS     CORSConfig     `mapstructure:"cors"`
	Health   HealthConfig   `mapstructure:"health"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            string        `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DatabaseConfig contains database connection configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            string        `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	MigrationPath   string        `mapstructure:"migration_path"`
}

// RedisConfig contains Redis connection configuration
type RedisConfig struct {
	URL           string        `mapstructure:"url"`
	Password      string        `mapstructure:"password"`
	DB            int           `mapstructure:"db"`
	MaxRetries    int           `mapstructure:"max_retries"`
	PoolSize      int           `mapstructure:"pool_size"`
	MinIdleConns  int           `mapstructure:"min_idle_conns"`
	DialTimeout   time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	PoolTimeout   time.Duration `mapstructure:"pool_timeout"`
	IdleTimeout   time.Duration `mapstructure:"idle_timeout"`
}

// JWTConfig contains JWT configuration
type JWTConfig struct {
	AccessSecret  string        `mapstructure:"access_secret"`
	RefreshSecret string        `mapstructure:"refresh_secret"`
	Issuer        string        `mapstructure:"issuer"`
	AccessExpiry  time.Duration `mapstructure:"access_expiry"`
	RefreshExpiry time.Duration `mapstructure:"refresh_expiry"`
	Algorithm     string        `mapstructure:"algorithm"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Port    string `mapstructure:"port"`
}

// TracingConfig contains distributed tracing configuration
type TracingConfig struct {
	Enabled         bool    `mapstructure:"enabled"`
	ServiceName     string  `mapstructure:"service_name"`
	JaegerEndpoint  string  `mapstructure:"jaeger_endpoint"`
	SampleRate      float64 `mapstructure:"sample_rate"`
}

// CORSConfig contains CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// HealthConfig contains health check configuration
type HealthConfig struct {
	CheckInterval time.Duration `mapstructure:"check_interval"`
	Timeout       time.Duration `mapstructure:"timeout"`
}

// LoadOptions contains configuration loading options
type LoadOptions struct {
	ServiceName   string   // Service name for config file selection
	ConfigPaths   []string // Additional paths to search for config files
	EnvPrefix     string   // Environment variable prefix
	DefaultValues map[string]interface{} // Default configuration values
}

// Load loads configuration using Viper with unified loading strategy
//
// Parameters:
//   - opts: Configuration loading options
//
// Returns:
//   - *BaseConfig: Loaded configuration
//   - error: Configuration loading error
//
// Features:
//   - Environment-based config file selection
//   - Multiple config path search
//   - Environment variable overrides
//   - Default value setting
//   - Configuration validation
func Load(opts LoadOptions) (*BaseConfig, error) {
	v := viper.New()
	
	// Set configuration file type
	v.SetConfigType("toml")

	// Determine environment
	env := os.Getenv("ENV")
	if env == "" {
		env = "local"
	}

	// Set configuration file name based on environment
	var configFileName string
	if env == "production" || env == "prod" {
		configFileName = fmt.Sprintf("%s.toml", opts.ServiceName)
	} else {
		configFileName = fmt.Sprintf("%s-local.toml", opts.ServiceName)
	}
	
	v.SetConfigName(configFileName[:len(configFileName)-5]) // Remove .toml extension

	// Add configuration search paths
	defaultPaths := []string{
		"./config",
		"/app/config",
		"../config",
		"../../config",
		".",
	}
	
	// Add custom paths first (higher priority)
	for _, path := range opts.ConfigPaths {
		v.AddConfigPath(path)
	}
	
	// Add default paths
	for _, path := range defaultPaths {
		v.AddConfigPath(path)
	}

	// Set environment variable configuration
	v.AutomaticEnv()
	if opts.EnvPrefix != "" {
		v.SetEnvPrefix(opts.EnvPrefix)
	}

	// Set default values
	setDefaults(v, opts.DefaultValues)

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found: %s", configFileName)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Expand environment variables
	expandEnvVars(v)

	// Unmarshal configuration
	var config BaseConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets reasonable default values for configuration
func setDefaults(v *viper.Viper, customDefaults map[string]interface{}) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.idle_timeout", "120s")
	v.SetDefault("server.shutdown_timeout", "30s")

	// Database defaults
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "1h")

	// Redis defaults
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.max_retries", 3)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 5)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("redis.pool_timeout", "4s")
	v.SetDefault("redis.idle_timeout", "5m")

	// JWT defaults
	v.SetDefault("jwt.algorithm", "HS256")
	v.SetDefault("jwt.access_expiry", "15m")
	v.SetDefault("jwt.refresh_expiry", "7d")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")

	// Tracing defaults
	v.SetDefault("tracing.enabled", false)
	v.SetDefault("tracing.sample_rate", 0.1)

	// CORS defaults
	v.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allowed_headers", []string{"Content-Type", "Authorization"})
	v.SetDefault("cors.allow_credentials", true)
	v.SetDefault("cors.max_age", 86400)

	// Health defaults
	v.SetDefault("health.check_interval", "30s")
	v.SetDefault("health.timeout", "5s")

	// Apply custom defaults
	for key, value := range customDefaults {
		v.SetDefault(key, value)
	}
}

// expandEnvVars expands environment variables in configuration values
func expandEnvVars(v *viper.Viper) {
	envKeys := []string{
		"database.host",
		"database.port",
		"database.name",
		"database.user",
		"database.password",
		"redis.url",
		"redis.password",
		"jwt.access_secret",
		"jwt.refresh_secret",
	}

	for _, key := range envKeys {
		value := v.GetString(key)
		if value != "" {
			expanded := os.ExpandEnv(value)
			v.Set(key, expanded)
		}
	}
}

// validate performs basic configuration validation
func validate(cfg *BaseConfig) error {
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

	return nil
}

// GetConfigFilePath returns the full path to the configuration file
func GetConfigFilePath(serviceName, environment string) (string, error) {
	var configFileName string
	if environment == "production" || environment == "prod" {
		configFileName = fmt.Sprintf("%s.toml", serviceName)
	} else {
		configFileName = fmt.Sprintf("%s-local.toml", serviceName)
	}

	searchPaths := []string{
		"./config",
		"/app/config",
		"../config",
		"../../config",
		".",
	}

	for _, path := range searchPaths {
		fullPath := filepath.Join(path, configFileName)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("config file not found: %s", configFileName)
}