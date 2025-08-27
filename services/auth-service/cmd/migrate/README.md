# 🛠️ Migration CLI Tool Documentation

**File**: `cmd/migrate/main.go`  
**Purpose**: Command-line interface for Migration-First schema management  
**Critical Component**: Database schema consistency enforcement

## 🎯 Overview

This is the core CLI tool that implements the Migration-First approach for database schema management. It ensures database schema consistency across all environments and provides comprehensive migration management capabilities.

## 🏗️ Architecture

### Key Components

1. **Command Router** (`main()`)
   - Parses CLI commands and flags
   - Initializes database connection
   - Routes to appropriate handlers

2. **Database Initialization** (`initDatabase()`)
   - Builds PostgreSQL connection string from environment variables
   - Configures GORM with proper logging and constraints
   - Maintains FK constraint enforcement (critical for data integrity)

3. **Command Handlers**
   - `handleStatus()`: Shows migration status
   - `handleMigrate()`: Applies pending migrations
   - `handleValidate()`: Validates schema consistency
   - `handleCreate()`: Creates new migration files
   - `handleRollback()`: Planned rollback functionality

### Database Configuration

```go
config := &gorm.Config{
    Logger: logger.New(...),
    DisableAutomaticPing: false,
    DisableForeignKeyConstraintWhenMigrating: false, // CRITICAL: Maintains FK integrity
}
```

⚠️ **Critical**: FK constraints are enforced during migrations to maintain referential integrity.

## 📋 Available Commands

### 1. Status (`migrate status`)
**Purpose**: Check migration status across environments  
**Key Features**:
- Shows total/applied/pending migration counts
- Lists pending migrations with details
- Environment-aware status checking

```bash
migrate status --env=production --verbose
```

### 2. Migrate (`migrate migrate`)
**Purpose**: Apply pending migrations  
**Key Features**:
- Dry-run capability (`--dry-run`)
- Transaction-based execution
- Execution time tracking
- Comprehensive error handling

```bash
migrate migrate --dry-run --verbose
migrate migrate --env=production
```

### 3. Validate (`migrate validate`)
**Purpose**: Validate database schema consistency  
**Key Features**:
- GORM model vs database schema comparison
- Column type validation
- Missing column detection
- Actionable recommendations

```bash
migrate validate --verbose
```

### 4. Create (`migrate create`)
**Purpose**: Generate new migration files  
**Key Features**:
- Timestamped migration files
- Standardized template generation
- UP/DOWN migration sections
- Proper formatting and documentation

```bash
migrate create add_user_avatar_field --dry-run
```

## 🔧 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | Database host |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | - | Database password (required) |
| `DB_NAME` | `auth_db` | Database name |
| `DB_PORT` | `5432` | Database port |

## 📝 Migration File Format

Generated migration files follow this structure:

```sql
-- ==========================================
-- Migration: 20231201120000_add_user_avatar_field.sql
-- Purpose: add_user_avatar_field
-- Author: Migration System
-- Date: 2023-12-01 12:00:00
-- Environment: ALL
-- ==========================================

-- 🔄 FORWARD MIGRATION (UP)
BEGIN;
-- Schema changes go here
COMMIT;

-- ==========================================
-- 🔙 DOWN MIGRATION (ROLLBACK)
-- ==========================================
-- Rollback instructions (currently manual)
```

## ⚡ Critical Implementation Details

### 1. Transaction Safety
All migrations execute within database transactions:
```go
tx, err := m.sqlDB.Begin()
defer tx.Rollback()
// ... execute migration
tx.Commit()
```

### 2. Checksum Validation
Each migration file is checksummed (SHA-256) to prevent tampering:
```go
checksum := m.calculateChecksum(contentStr)
```

### 3. Environment Isolation
Migrations are tracked per environment to prevent cross-environment issues:
```go
if err := m.db.Where("environment = ?", m.environment).Find(&records).Error
```

### 4. FK Constraint Enforcement
Critical for referential integrity - never disabled during migrations:
```go
DisableForeignKeyConstraintWhenMigrating: false
```

## 🚨 Production Safety Features

1. **Dry Run Mode**: Preview changes without execution
2. **Environment Validation**: Prevents accidental cross-environment migrations
3. **Transaction Rollback**: Automatic rollback on errors
4. **Checksum Verification**: Prevents modified migration files
5. **Verbose Logging**: Detailed execution information

## 🔗 Integration Points

- **Docker**: Called during container startup for automatic migrations
- **CI/CD**: Pre-commit hooks validate schema consistency
- **Development**: Local development workflow integration
- **Production**: Safe production deployment with validation

## 📚 Related Files

- `internal/migrations/migration_manager.go`: Core migration logic
- `migrations/*.sql`: Migration files
- `config/*.toml`: Database configuration
- `docs/guides/MIGRATION_DEVELOPER_GUIDE.md`: Developer workflow

## ⚠️ Important Notes

1. **Never modify applied migrations** - Create new migrations instead
2. **Always test migrations in development first**
3. **Use `--dry-run` before production deployments**
4. **Backup production databases before major schema changes**
5. **Rollback functionality is planned but not yet implemented**

## 🐛 Troubleshooting

### Common Issues

1. **Connection Failures**
   ```bash
   # Verify environment variables
   echo $DB_PASSWORD
   ```

2. **Permission Errors**
   ```bash
   # Ensure database user has schema modification privileges
   GRANT ALL ON SCHEMA public TO your_user;
   ```

3. **Migration Conflicts**
   ```bash
   # Check migration status
   migrate status --verbose
   ```

This CLI tool is the cornerstone of the Migration-First approach and ensures database schema consistency across all environments.