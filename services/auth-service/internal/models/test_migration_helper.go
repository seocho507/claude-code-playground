package models

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// runSQLMigrations executes SQL migration files in order
// This replaces GORM AutoMigrate to ensure test environment matches production
func runSQLMigrations(db *gorm.DB, migrationDir string) error {
	// Get all SQL files in migration directory
	pattern := filepath.Join(migrationDir, "*.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find migration files: %v", err)
	}

	// Sort files to ensure proper execution order
	sort.Strings(files)

	// Execute each migration file
	for _, file := range files {
		fmt.Printf("Executing migration: %s\n", filepath.Base(file))
		
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %v", file, err)
		}

		// Split content by statements (simple split by semicolon)
		statements := strings.Split(string(content), ";")
		
		for _, statement := range statements {
			statement = strings.TrimSpace(statement)
			if statement == "" || strings.HasPrefix(statement, "--") {
				continue
			}
			
			if err := db.Exec(statement).Error; err != nil {
				return fmt.Errorf("failed to execute statement in %s: %v\nStatement: %s", file, err, statement)
			}
		}
	}

	fmt.Printf("Successfully executed %d migration files\n", len(files))
	return nil
}

// validateModelSchema validates that a model can be used with the current database schema
func validateModelSchema(db *gorm.DB, model interface{}) error {
	// Test CREATE operation
	if err := db.Create(model).Error; err != nil {
		return fmt.Errorf("model CREATE validation failed: %v", err)
	}

	// Test READ operation  
	if err := db.First(model).Error; err != nil {
		return fmt.Errorf("model READ validation failed: %v", err)
	}

	return nil
}

// setupTestDBWithMigrations sets up test database using SQL migrations instead of AutoMigrate
func setupTestDBWithMigrations(db *gorm.DB) error {
	// Path to migration files (relative to test file location)
	migrationDir := "../../migrations"
	
	// Run SQL migrations
	if err := runSQLMigrations(db, migrationDir); err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	fmt.Println("✅ Test database setup completed with SQL migrations")
	return nil
}