# 🔧 Migration Manager Documentation

**File**: `internal/migrations/migration_manager.go`  
**Purpose**: Core migration logic and schema validation  
**Critical Component**: Migration-First schema consistency engine

## 🎯 Overview

The `MigrationManager` is the core engine that implements Migration-First database schema management. It provides atomic migration execution, schema validation, and comprehensive tracking of database changes across environments.

## 🏗️ Core Architecture

### Primary Components

#### 1. MigrationManager Struct
```go
type MigrationManager struct {
    db            *gorm.DB     // GORM database connection
    sqlDB         *sql.DB      // Raw SQL connection for transactions
    migrationsDir string       // Migration files directory
    environment   string       // Current environment context
}
```

#### 2. MigrationRecord Model
```go
type MigrationRecord struct {
    ID              int       `gorm:"primaryKey;autoIncrement"`
    Version         string    `gorm:"uniqueIndex;not null;size:50"`
    Name            string    `gorm:"not null;size:255"`
    Checksum        string    `gorm:"not null;size:64"`        // SHA-256
    AppliedAt       time.Time `gorm:"not null"`
    AppliedBy       string    `gorm:"size:100"`
    Environment     string    `gorm:"not null;size:50"`
    ExecutionTimeMs int       `gorm:"not null"`
}
```

**Critical**: Uses table name `schema_migrations` for tracking applied migrations.

#### 3. Migration Struct
```go
type Migration struct {
    Version   string  // Migration version (timestamp)
    Name      string  // Descriptive name
    FilePath  string  // Source file path
    Content   string  // Full file content
    Checksum  string  // SHA-256 integrity check
    UpSQL     string  // Forward migration SQL
    DownSQL   string  // Rollback SQL (planned feature)
}
```

## 🔄 Core Operations

### 1. Migration Discovery (`loadMigrationFiles()`)
**Purpose**: Scans migration directory and loads all `.sql` files

**Process**:
1. Walks migration directory recursively
2. Parses filename format: `{version}_{name}.sql`
3. Calculates SHA-256 checksum for integrity
4. Sorts by version for sequential application

```go
// Expected filename format
001_initial_schema.sql
002_add_user_preferences.sql
003_add_sessions_table.sql
```

### 2. Pending Migration Detection (`GetPendingMigrations()`)
**Process**:
1. Load all available migration files
2. Query database for applied migrations (by environment)
3. Return migrations not yet applied

**Environment Isolation**: Each environment tracks migrations independently.

### 3. Atomic Migration Application (`applyMigration()`)
**Critical Features**:
- **Transaction-based**: All changes in single transaction
- **Integrity verification**: Checksum validation
- **Execution tracking**: Performance metrics
- **Automatic rollback**: On any error

```go
// Transaction lifecycle
tx, err := m.sqlDB.Begin()
defer tx.Rollback()  // Automatic rollback on error

// Execute migration SQL
tx.Exec(migration.UpSQL)

// Record in tracking table
tx.Exec(insertSQL, record...)

// Commit only if all succeed
tx.Commit()
```

### 4. Schema Consistency Validation (`ValidateSchema()`)
**Purpose**: Ensures database matches expected schema

**Validation Process**:
1. Queries `information_schema` for current tables
2. Validates required tables exist
3. Checks column definitions and types
4. Reports inconsistencies with recommendations

**Required Tables**:
- `users`
- `sessions` 
- `login_attempts`
- `user_preferences`
- `user_activities`
- `user_notifications`
- `schema_migrations`

## 🔐 Security & Integrity Features

### 1. Checksum Verification
```go
func (m *MigrationManager) calculateChecksum(content string) string {
    h := sha256.Sum256([]byte(content))
    return fmt.Sprintf("%x", h)
}
```
**Purpose**: Prevents migration tampering and ensures consistency across environments.

### 2. Environment Isolation
```go
if err := m.db.Where("environment = ?", m.environment).Find(&records).Error
```
**Purpose**: Prevents cross-environment pollution of migration state.

### 3. Transaction Safety
All migrations execute within database transactions with automatic rollback on failure.

## 📊 Migration Tracking

### Schema Migrations Table Structure
```sql
CREATE TABLE schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT NOW(),
    applied_by VARCHAR(100),
    environment VARCHAR(50) NOT NULL,
    execution_time_ms INTEGER NOT NULL
);
```

### Indexes for Performance
```sql
CREATE INDEX idx_schema_migrations_version ON schema_migrations(version);
CREATE INDEX idx_schema_migrations_environment ON schema_migrations(environment);
CREATE INDEX idx_schema_migrations_applied_at ON schema_migrations(applied_at);
```

## 🚀 Migration File Parsing

### UP/DOWN Section Detection
```go
func (m *MigrationManager) splitMigrationContent(content string) (upSQL, downSQL string) {
    // Splits content on "-- DOWN MIGRATION" or "-- ROLLBACK" markers
    // UP section: Everything before DOWN marker
    // DOWN section: Everything after DOWN marker
}
```

### File Format Expected
```sql
-- Migration content (UP section)
BEGIN;
CREATE TABLE new_table (...);
COMMIT;

-- DOWN MIGRATION (ROLLBACK)
BEGIN;
DROP TABLE new_table;
COMMIT;
```

## 📈 Performance Monitoring

### Execution Time Tracking
```go
startTime := time.Now()
// ... execute migration
executionTime := time.Since(startTime)
record.ExecutionTimeMs = int(executionTime.Milliseconds())
```

### Migration Results
```go
type MigrationResult struct {
    Migration     *Migration
    Success       bool
    Error         error
    ExecutionTime time.Duration
    RollbackSQL   string
}
```

## 🔄 Integration Points

### 1. CLI Tool Integration
Called by `cmd/migrate/main.go` for all migration operations.

### 2. Application Startup
Can be integrated into application startup for automatic migrations:
```go
migrationManager, err := migrations.NewMigrationManager(db, "migrations", env)
migrationManager.ApplyMigrations()
```

### 3. Docker Integration
Called during container startup via migration scripts.

## ⚠️ Critical Implementation Notes

### 1. Foreign Key Constraints
**Never disable FK constraints during migrations** - maintains referential integrity.

### 2. Environment Handling
Each environment (dev/test/prod) maintains separate migration state.

### 3. Error Handling
**Fail-fast approach**: Any migration error stops the entire process and rolls back.

### 4. File Integrity
SHA-256 checksums prevent unauthorized modification of migration files.

## 🐛 Error Scenarios & Recovery

### 1. Migration Failure
- Transaction automatically rolls back
- Migration not recorded in tracking table
- Safe to retry after fixing issues

### 2. Checksum Mismatch
- Indicates file modification after application
- Prevents re-application of modified migrations
- Requires new migration to make changes

### 3. Database Connection Issues
- Fails fast with clear error messages
- No partial state changes
- Safe to retry

## 📚 Related Components

- **CLI Tool**: `cmd/migrate/main.go` - User interface
- **Schema Validator**: `schema_validator.go` - Schema consistency checking
- **Migration Files**: `migrations/*.sql` - Actual schema changes
- **Configuration**: `config/*.toml` - Database connection settings

## 🔧 Usage Examples

```go
// Initialize manager
manager, err := NewMigrationManager(db, "migrations", "production")

// Check status
status, err := manager.GetMigrationStatus()

// Apply pending migrations
results, err := manager.ApplyMigrations()

// Validate schema
err = manager.ValidateSchema()
```

## 🎯 Future Enhancements

1. **Rollback Support**: Automatic DOWN migration execution
2. **Parallel Migrations**: Independent migration streams
3. **Migration Dependencies**: Explicit dependency management
4. **Advanced Validation**: Deep schema comparison with GORM models

This migration manager is the foundation of the Migration-First approach, ensuring reliable and consistent database schema evolution across all environments.