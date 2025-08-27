package migrations

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MigrationManager handles database schema migrations with strict consistency
type MigrationManager struct {
	db          *gorm.DB
	sqlDB       *sql.DB
	migrationsDir string
	environment   string
}

// MigrationRecord tracks applied migrations in the database
type MigrationRecord struct {
	ID          int       `gorm:"primaryKey;autoIncrement"`
	Version     string    `gorm:"uniqueIndex;not null;size:50"`
	Name        string    `gorm:"not null;size:255"`
	Checksum    string    `gorm:"not null;size:64"` // SHA-256 of migration file
	AppliedAt   time.Time `gorm:"not null"`
	AppliedBy   string    `gorm:"size:100"`
	Environment string    `gorm:"not null;size:50"`
	ExecutionTimeMs int   `gorm:"not null"`
}

// TableName overrides the table name used by this model
func (MigrationRecord) TableName() string {
	return "schema_migrations"
}

// Migration represents a single database migration
type Migration struct {
	Version   string
	Name      string
	FilePath  string
	Content   string
	Checksum  string
	UpSQL     string
	DownSQL   string
}

// MigrationResult contains the result of migration execution
type MigrationResult struct {
	Migration     *Migration
	Success       bool
	Error         error
	ExecutionTime time.Duration
	RollbackSQL   string
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *gorm.DB, migrationsDir, environment string) (*MigrationManager, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB from gorm.DB: %w", err)
	}

	manager := &MigrationManager{
		db:            db,
		sqlDB:         sqlDB,
		migrationsDir: migrationsDir,
		environment:   environment,
	}

	// Ensure migration tracking table exists
	if err := manager.ensureMigrationsTable(); err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %w", err)
	}

	return manager, nil
}

// ensureMigrationsTable creates the migration tracking table if it doesn't exist
func (m *MigrationManager) ensureMigrationsTable() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		id SERIAL PRIMARY KEY,
		version VARCHAR(50) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		checksum VARCHAR(64) NOT NULL,
		applied_at TIMESTAMP NOT NULL DEFAULT NOW(),
		applied_by VARCHAR(100),
		environment VARCHAR(50) NOT NULL,
		execution_time_ms INTEGER NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_schema_migrations_version ON schema_migrations(version);
	CREATE INDEX IF NOT EXISTS idx_schema_migrations_environment ON schema_migrations(environment);
	CREATE INDEX IF NOT EXISTS idx_schema_migrations_applied_at ON schema_migrations(applied_at);
	`

	if _, err := m.sqlDB.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	log.Println("✅ Schema migrations table ensured")
	return nil
}

// GetPendingMigrations returns migrations that haven't been applied yet
func (m *MigrationManager) GetPendingMigrations() ([]*Migration, error) {
	// Load all migration files
	allMigrations, err := m.loadMigrationFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to load migration files: %w", err)
	}

	// Get applied migrations from database
	appliedVersions, err := m.getAppliedVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Filter out already applied migrations
	var pending []*Migration
	for _, migration := range allMigrations {
		if !appliedVersions[migration.Version] {
			pending = append(pending, migration)
		}
	}

	return pending, nil
}

// loadMigrationFiles loads and parses all migration files from the directory
func (m *MigrationManager) loadMigrationFiles() ([]*Migration, error) {
	var migrations []*Migration

	err := filepath.WalkDir(m.migrationsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		// Parse migration file name (format: 001_name.sql)
		fileName := d.Name()
		parts := strings.SplitN(fileName, "_", 2)
		if len(parts) != 2 {
			log.Printf("Warning: Skipping invalid migration file name: %s", fileName)
			return nil
		}

		version := parts[0]
		name := strings.TrimSuffix(parts[1], ".sql")

		migration, err := m.parseMigrationFile(path, version, name)
		if err != nil {
			return fmt.Errorf("failed to parse migration file %s: %w", path, err)
		}

		migrations = append(migrations, migration)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		vi, _ := strconv.Atoi(migrations[i].Version)
		vj, _ := strconv.Atoi(migrations[j].Version)
		return vi < vj
	})

	return migrations, nil
}

// parseMigrationFile parses a single migration file
func (m *MigrationManager) parseMigrationFile(filePath, version, name string) (*Migration, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file: %w", err)
	}

	contentStr := string(content)
	checksum := m.calculateChecksum(contentStr)

	// Split UP and DOWN migrations (if present)
	upSQL, downSQL := m.splitMigrationContent(contentStr)

	return &Migration{
		Version:  version,
		Name:     name,
		FilePath: filePath,
		Content:  contentStr,
		Checksum: checksum,
		UpSQL:    upSQL,
		DownSQL:  downSQL,
	}, nil
}

// splitMigrationContent splits migration content into UP and DOWN sections
func (m *MigrationManager) splitMigrationContent(content string) (upSQL, downSQL string) {
	lines := strings.Split(content, "\n")
	var upLines, downLines []string
	inDownSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- DOWN MIGRATION") || strings.HasPrefix(trimmed, "-- ROLLBACK") {
			inDownSection = true
			continue
		}

		if inDownSection {
			downLines = append(downLines, line)
		} else {
			upLines = append(upLines, line)
		}
	}

	upSQL = strings.TrimSpace(strings.Join(upLines, "\n"))
	downSQL = strings.TrimSpace(strings.Join(downLines, "\n"))
	return
}

// getAppliedVersions returns a map of applied migration versions
func (m *MigrationManager) getAppliedVersions() (map[string]bool, error) {
	var records []MigrationRecord
	
	if err := m.db.Where("environment = ?", m.environment).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to query migration records: %w", err)
	}

	applied := make(map[string]bool)
	for _, record := range records {
		applied[record.Version] = true
	}

	return applied, nil
}

// ApplyMigrations applies all pending migrations
func (m *MigrationManager) ApplyMigrations() ([]*MigrationResult, error) {
	pending, err := m.GetPendingMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending migrations: %w", err)
	}

	if len(pending) == 0 {
		log.Println("✅ No pending migrations to apply")
		return nil, nil
	}

	log.Printf("🚀 Applying %d pending migrations...", len(pending))

	var results []*MigrationResult
	for _, migration := range pending {
		result := m.applyMigration(migration)
		results = append(results, result)

		if !result.Success {
			log.Printf("❌ Migration %s failed: %v", migration.Version, result.Error)
			return results, fmt.Errorf("migration %s failed: %w", migration.Version, result.Error)
		}

		log.Printf("✅ Applied migration %s: %s (%.2fms)", 
			migration.Version, migration.Name, float64(result.ExecutionTime.Nanoseconds())/1e6)
	}

	log.Printf("🎉 Successfully applied %d migrations", len(results))
	return results, nil
}

// applyMigration applies a single migration
func (m *MigrationManager) applyMigration(migration *Migration) *MigrationResult {
	startTime := time.Now()
	
	result := &MigrationResult{
		Migration: migration,
		Success:   false,
	}

	// Begin transaction
	tx, err := m.sqlDB.Begin()
	if err != nil {
		result.Error = fmt.Errorf("failed to begin transaction: %w", err)
		return result
	}
	defer tx.Rollback()

	// Execute migration SQL
	if _, err := tx.Exec(migration.UpSQL); err != nil {
		result.Error = fmt.Errorf("failed to execute migration SQL: %w", err)
		return result
	}

	// Record migration in tracking table
	executionTime := time.Since(startTime)
	record := MigrationRecord{
		Version:         migration.Version,
		Name:            migration.Name,
		Checksum:        migration.Checksum,
		AppliedAt:       time.Now(),
		AppliedBy:       "migration_manager",
		Environment:     m.environment,
		ExecutionTimeMs: int(executionTime.Milliseconds()),
	}

	insertSQL := `
		INSERT INTO schema_migrations (version, name, checksum, applied_at, applied_by, environment, execution_time_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if _, err := tx.Exec(insertSQL, record.Version, record.Name, record.Checksum, 
		record.AppliedAt, record.AppliedBy, record.Environment, record.ExecutionTimeMs); err != nil {
		result.Error = fmt.Errorf("failed to record migration: %w", err)
		return result
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		result.Error = fmt.Errorf("failed to commit migration: %w", err)
		return result
	}

	result.Success = true
	result.ExecutionTime = executionTime
	result.RollbackSQL = migration.DownSQL
	return result
}

// ValidateSchema validates current database schema against expected schema
func (m *MigrationManager) ValidateSchema() error {
	// This would implement schema validation logic
	log.Println("🔍 Validating database schema consistency...")
	
	// Get current schema information
	tables, err := m.getCurrentTables()
	if err != nil {
		return fmt.Errorf("failed to get current tables: %w", err)
	}

	// Validate required tables exist
	requiredTables := []string{
		"users", "sessions", "login_attempts", 
		"user_preferences", "user_activities", "user_notifications",
		"schema_migrations",
	}

	for _, table := range requiredTables {
		if !contains(tables, table) {
			return fmt.Errorf("required table '%s' is missing", table)
		}
	}

	log.Println("✅ Schema validation passed")
	return nil
}

// getCurrentTables returns list of tables in current database
func (m *MigrationManager) getCurrentTables() ([]string, error) {
	rows, err := m.sqlDB.Query(`
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// GetMigrationStatus returns the current migration status
func (m *MigrationManager) GetMigrationStatus() (*MigrationStatus, error) {
	all, err := m.loadMigrationFiles()
	if err != nil {
		return nil, err
	}

	pending, err := m.GetPendingMigrations()
	if err != nil {
		return nil, err
	}

	applied := len(all) - len(pending)

	return &MigrationStatus{
		TotalMigrations:   len(all),
		AppliedMigrations: applied,
		PendingMigrations: len(pending),
		Environment:       m.environment,
		LastAppliedAt:     time.Now(), // This should query the actual last migration
	}, nil
}

// MigrationStatus represents current migration status
type MigrationStatus struct {
	TotalMigrations   int       `json:"total_migrations"`
	AppliedMigrations int       `json:"applied_migrations"`
	PendingMigrations int       `json:"pending_migrations"`
	Environment       string    `json:"environment"`
	LastAppliedAt     time.Time `json:"last_applied_at"`
}

// calculateChecksum calculates SHA-256 checksum of content
func (m *MigrationManager) calculateChecksum(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// contains checks if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}