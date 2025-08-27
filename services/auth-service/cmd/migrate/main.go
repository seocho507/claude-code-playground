package main

import (
	"auth-service/internal/migrations"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CLI commands
const (
	CmdStatus   = "status"
	CmdMigrate  = "migrate" 
	CmdValidate = "validate"
	CmdRollback = "rollback"
	CmdCreate   = "create"
	CmdHelp     = "help"
)

var (
	environment = flag.String("env", "development", "Environment (development, test, production)")
	configPath  = flag.String("config", "config/config.toml", "Config file path")
	dryRun      = flag.Bool("dry-run", false, "Show what would be done without executing")
	verbose     = flag.Bool("v", false, "Verbose output")
	force       = flag.Bool("force", false, "Force operation (use with caution)")
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]
	
	// Parse flags that come after the command
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	flag.Parse()
	
	// Help doesn't need database connection
	if command == CmdHelp {
		printHelp()
		return
	}

	// Initialize database connection
	db, err := initDatabase()
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	// Initialize migration manager
	migrationsDir := "migrations"
	migrationManager, err := migrations.NewMigrationManager(db, migrationsDir, *environment)
	if err != nil {
		log.Fatalf("❌ Failed to initialize migration manager: %v", err)
	}

	// Execute command
	switch command {
	case CmdStatus:
		handleStatus(migrationManager)
	case CmdMigrate:
		handleMigrate(migrationManager)
	case CmdValidate:
		handleValidate(db)
	case CmdRollback:
		handleRollback(migrationManager)
	case CmdCreate:
		handleCreate()
	default:
		fmt.Printf("❌ Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func initDatabase() (*gorm.DB, error) {
	// Get database configuration from environment variables
	host := getEnvOrDefault("DB_HOST", "localhost")
	user := getEnvOrDefault("DB_USER", "postgres")
	password := getEnvOrDefault("DB_PASSWORD", "")
	dbname := getEnvOrDefault("DB_NAME", "auth_db")
	port := getEnvOrDefault("DB_PORT", "5432")

	// Build connection string
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port,
	)

	// Configure GORM for migration operations
	config := &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  getLogLevel(),
				IgnoreRecordNotFoundError: false,
				Colorful:                  true,
			},
		),
		DisableAutomaticPing:   false,
		DisableForeignKeyConstraintWhenMigrating: false, // Important: keep FK constraints
	}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Test connection
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func getLogLevel() logger.LogLevel {
	if *verbose {
		return logger.Info
	}
	return logger.Warn
}

func handleStatus(mgr *migrations.MigrationManager) {
	fmt.Println("🔍 Checking migration status...")
	
	status, err := mgr.GetMigrationStatus()
	if err != nil {
		log.Fatalf("❌ Failed to get migration status: %v", err)
	}

	fmt.Printf("\n📊 Migration Status for %s environment:\n", *environment)
	fmt.Printf("   Total migrations: %d\n", status.TotalMigrations)
	fmt.Printf("   Applied: %d\n", status.AppliedMigrations)
	fmt.Printf("   Pending: %d\n", status.PendingMigrations)
	
	if status.PendingMigrations > 0 {
		fmt.Printf("\n⚠️  %d pending migrations need to be applied\n", status.PendingMigrations)
		
		pending, err := mgr.GetPendingMigrations()
		if err != nil {
			log.Printf("Failed to get pending migration details: %v", err)
		} else {
			fmt.Println("\nPending migrations:")
			for _, migration := range pending {
				fmt.Printf("   - %s: %s\n", migration.Version, migration.Name)
			}
		}
		fmt.Println("\nRun 'migrate migrate' to apply pending migrations")
	} else {
		fmt.Println("\n✅ Database is up to date")
	}
}

func handleMigrate(mgr *migrations.MigrationManager) {
	if *dryRun {
		fmt.Println("🔍 DRY RUN: Showing what would be migrated...")
		
		pending, err := mgr.GetPendingMigrations()
		if err != nil {
			log.Fatalf("❌ Failed to get pending migrations: %v", err)
		}

		if len(pending) == 0 {
			fmt.Println("✅ No pending migrations")
			return
		}

		fmt.Printf("\nWould apply %d migrations:\n", len(pending))
		for _, migration := range pending {
			fmt.Printf("   - %s: %s\n", migration.Version, migration.Name)
		}
		fmt.Println("\nRun without --dry-run to apply these migrations")
		return
	}

	fmt.Println("🚀 Applying pending migrations...")
	
	results, err := mgr.ApplyMigrations()
	if err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	if len(results) == 0 {
		fmt.Println("✅ No pending migrations to apply")
		return
	}

	fmt.Printf("\n🎉 Successfully applied %d migrations\n", len(results))
	for _, result := range results {
		fmt.Printf("   ✅ %s: %s (%.2fms)\n", 
			result.Migration.Version, 
			result.Migration.Name, 
			float64(result.ExecutionTime.Nanoseconds())/1e6)
	}
}

func handleValidate(db *gorm.DB) {
	fmt.Println("🔍 Validating database schema...")
	
	validator, err := migrations.NewSchemaValidator(db)
	if err != nil {
		log.Fatalf("❌ Failed to initialize schema validator: %v", err)
	}

	results, err := validator.ValidateAllTables()
	if err != nil {
		log.Fatalf("❌ Schema validation failed: %v", err)
	}

	validCount := 0
	invalidCount := 0
	
	fmt.Println("\n📋 Schema Validation Results:")
	fmt.Println("=" + strings.Repeat("=", 40))
	
	for _, result := range results {
		status := "✅"
		if !result.IsValid {
			status = "❌"
			invalidCount++
		} else {
			validCount++
		}
		
		fmt.Printf("%s %s\n", status, result.TableName)
		
		if !result.IsValid && *verbose {
			if len(result.MissingColumns) > 0 {
				fmt.Printf("   Missing columns: %s\n", strings.Join(result.MissingColumns, ", "))
			}
			if len(result.TypeMismatches) > 0 {
				fmt.Println("   Type mismatches:")
				for _, mismatch := range result.TypeMismatches {
					fmt.Printf("     - %s: expected %s, got %s\n", 
						mismatch.ColumnName, mismatch.ExpectedType, mismatch.ActualType)
				}
			}
			if len(result.RecommendedActions) > 0 {
				fmt.Println("   Recommendations:")
				for _, action := range result.RecommendedActions {
					fmt.Printf("     - %s\n", action)
				}
			}
		}
	}
	
	fmt.Println("=" + strings.Repeat("=", 40))
	fmt.Printf("Summary: %d valid, %d invalid tables\n", validCount, invalidCount)
	
	if invalidCount > 0 {
		fmt.Printf("\n⚠️  %d tables have schema issues\n", invalidCount)
		fmt.Println("Use --verbose flag for detailed information")
		fmt.Println("Consider creating new migrations to fix these issues")
		os.Exit(1)
	} else {
		fmt.Println("\n✅ All tables have valid schemas")
	}
}

func handleRollback(mgr *migrations.MigrationManager) {
	fmt.Println("🔄 Rollback functionality not yet implemented")
	fmt.Println("This is a planned feature for future versions")
	
	if !*force {
		fmt.Println("\nFor now, manual rollback is required:")
		fmt.Println("1. Review the DOWN migration SQL in the migration file")
		fmt.Println("2. Execute the rollback SQL manually")
		fmt.Println("3. Remove the migration record from schema_migrations table")
		return
	}
	
	// TODO: Implement rollback functionality
	log.Fatal("❌ Rollback not implemented yet")
}

func handleCreate() {
	if len(os.Args) < 3 {
		fmt.Println("❌ Migration name required")
		fmt.Println("Usage: migrate create <migration_name>")
		os.Exit(1)
	}
	
	name := os.Args[2]
	
	// Generate migration file
	version := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", version, name)
	filepath := filepath.Join("migrations", filename)
	
	template := generateMigrationTemplate(version, name)
	
	if *dryRun {
		fmt.Printf("🔍 DRY RUN: Would create migration file: %s\n", filepath)
		fmt.Println("\nTemplate content:")
		fmt.Println(template)
		return
	}
	
	// Create migrations directory if it doesn't exist
	if err := os.MkdirAll("migrations", 0755); err != nil {
		log.Fatalf("❌ Failed to create migrations directory: %v", err)
	}
	
	// Write migration file
	if err := os.WriteFile(filepath, []byte(template), 0644); err != nil {
		log.Fatalf("❌ Failed to create migration file: %v", err)
	}
	
	fmt.Printf("✅ Created migration file: %s\n", filepath)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Edit the migration file to add your schema changes")
	fmt.Println("2. Test the migration with 'migrate migrate --dry-run'")
	fmt.Println("3. Apply the migration with 'migrate migrate'")
}


func generateMigrationTemplate(version, name string) string {
	return fmt.Sprintf(`-- ==========================================
-- Migration: %s_%s.sql
-- Purpose: %s
-- Author: Migration System
-- Date: %s
-- Environment: ALL
-- ==========================================

-- 🔄 FORWARD MIGRATION (UP)
BEGIN;

-- Add your schema changes here
-- Example:
-- ALTER TABLE users ADD COLUMN new_field VARCHAR(255);
-- CREATE INDEX idx_users_new_field ON users(new_field);

COMMIT;

-- ==========================================
-- 🔙 DOWN MIGRATION (ROLLBACK)
-- ==========================================
-- To rollback this migration, run:
-- 
-- BEGIN;
-- 
-- -- Reverse the changes above
-- -- Example:
-- -- DROP INDEX IF EXISTS idx_users_new_field;
-- -- ALTER TABLE users DROP COLUMN IF EXISTS new_field;
-- 
-- COMMIT;
`, version, name, name, time.Now().Format("2006-01-02 15:04:05"))
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printHelp() {
	fmt.Println("🚀 Migration-First Schema Management Tool")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  migrate <command> [flags]")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  status    Show migration status")
	fmt.Println("  migrate   Apply pending migrations")
	fmt.Println("  validate  Validate database schema consistency")
	fmt.Println("  create    Create a new migration file")
	fmt.Println("  rollback  Rollback last migration (planned)")
	fmt.Println("  help      Show this help message")
	fmt.Println()
	fmt.Println("FLAGS:")
	fmt.Println("  --env string       Environment (development, test, production) (default: development)")
	fmt.Println("  --config string    Config file path (default: config/config.toml)")
	fmt.Println("  --dry-run          Show what would be done without executing")
	fmt.Println("  --verbose, -v      Verbose output")
	fmt.Println("  --force            Force operation (use with caution)")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  migrate status                              # Check migration status")
	fmt.Println("  migrate migrate --dry-run                   # Preview pending migrations")
	fmt.Println("  migrate migrate                             # Apply pending migrations")
	fmt.Println("  migrate validate --verbose                  # Detailed schema validation")
	fmt.Println("  migrate create add_user_avatar_field        # Create new migration")
	fmt.Println("  migrate status --env=production             # Check production status")
	fmt.Println()
	fmt.Println("MIGRATION-FIRST WORKFLOW:")
	fmt.Println("  1. Create migration: migrate create <name>")
	fmt.Println("  2. Edit migration file in migrations/ directory")
	fmt.Println("  3. Test migration: migrate migrate --dry-run")
	fmt.Println("  4. Apply migration: migrate migrate")
	fmt.Println("  5. Validate schema: migrate validate")
	fmt.Println()
	fmt.Println("🔗 For more information, see: docs/migrations.md")
}