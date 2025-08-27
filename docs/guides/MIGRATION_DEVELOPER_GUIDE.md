# 🔄 Migration Developer Workflow Guide

## 📋 **Overview**

This guide provides developers with comprehensive instructions for working with the Migration-First schema consistency system. Follow these workflows to maintain schema integrity and prevent production issues.

---

## 🚀 **Quick Start**

### **1. Initial Setup**
```bash
# Install Git hooks for automatic schema validation
bash scripts/setup-git-hooks.sh

# Verify installation
git config core.hooksPath  # Should show: .githooks
```

### **2. Daily Development Workflow**
```bash
# 1. Check current migration status
cd services/auth-service
go run cmd/migrate/main.go status

# 2. If behind, apply pending migrations
go run cmd/migrate/main.go migrate

# 3. Validate schema consistency
go run cmd/migrate/main.go validate

# 4. Run tests to ensure everything works
go test ./internal/models ./internal/repositories -v
```

---

## 🔧 **Migration CLI Commands**

### **Status Commands**
```bash
# Show migration status
go run cmd/migrate/main.go status
# Output: Shows applied migrations, pending migrations, and schema state

# Validate schema alignment
go run cmd/migrate/main.go validate
# Output: Checks GORM models vs database schema consistency
```

### **Migration Commands**
```bash
# Apply pending migrations
go run cmd/migrate/main.go migrate

# Create new migration
go run cmd/migrate/main.go create AddUserTwoFactor

# Rollback last migration (if supported)
go run cmd/migrate/main.go rollback
```

---

## 📝 **Creating New Migrations**

### **Step 1: Identify the Need**
Create migrations when:
- Adding new tables
- Modifying existing table structure
- Adding/removing columns
- Changing column types
- Adding/removing indexes
- Creating/dropping constraints

### **Step 2: Create Migration File**
```bash
# Navigate to auth service
cd services/auth-service

# Create migration with descriptive name
go run cmd/migrate/main.go create AddUserNotificationPreferences
```

### **Step 3: Edit Migration File**
Migration files are located in `services/auth-service/migrations/`:

```sql
-- ==========================================
-- Migration: 002_add_user_notification_preferences.sql
-- Author: Developer Name
-- Date: 2024-01-XX
-- Purpose: Add user notification preference settings
-- Dependencies: 001_initial_schema.sql
-- ==========================================

-- 🔄 FORWARD MIGRATION (UP)
BEGIN;

-- Create notification preferences table
CREATE TABLE user_notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email_notifications BOOLEAN DEFAULT true,
    push_notifications BOOLEAN DEFAULT true,
    sms_notifications BOOLEAN DEFAULT false,
    notification_frequency VARCHAR(20) DEFAULT 'immediate' CHECK (notification_frequency IN ('immediate', 'daily', 'weekly', 'disabled')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add indexes for performance
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_notification_preferences_user_id ON user_notification_preferences(user_id);
CREATE INDEX IF NOT EXISTS idx_user_notification_preferences_frequency ON user_notification_preferences(notification_frequency);

-- Insert default preferences for existing users
INSERT INTO user_notification_preferences (user_id)
SELECT id FROM users
WHERE id NOT IN (SELECT user_id FROM user_notification_preferences);

COMMIT;

-- 🔙 ROLLBACK MIGRATION (DOWN)
-- To rollback this migration manually:
-- DROP TABLE IF EXISTS user_notification_preferences CASCADE;
```

### **Step 4: Update GORM Models**
If adding new tables, create corresponding GORM models:

```go
// internal/models/user_notification_preferences.go
type UserNotificationPreferences struct {
    ID                   uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    UserID               uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
    EmailNotifications   bool      `gorm:"default:true" json:"email_notifications"`
    PushNotifications    bool      `gorm:"default:true" json:"push_notifications"`
    SMSNotifications     bool      `gorm:"default:false" json:"sms_notifications"`
    NotificationFrequency string   `gorm:"type:varchar(20);default:'immediate';check:notification_frequency IN ('immediate','daily','weekly','disabled')" json:"notification_frequency"`
    CreatedAt            time.Time `gorm:"not null;default:now()" json:"created_at"`
    UpdatedAt            time.Time `gorm:"not null;default:now()" json:"updated_at"`

    // Relationships
    User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}
```

### **Step 5: Validate and Test**
```bash
# 1. Validate migration syntax and schema alignment
go run cmd/migrate/main.go validate

# 2. Run integration tests
docker-compose -f docker-compose.migration-test.yml up -d
sleep 15
go test ./internal/models ./internal/repositories -v

# 3. Clean up test environment
docker-compose -f docker-compose.migration-test.yml down --volumes
```

---

## 🔍 **Schema Validation Workflow**

### **Pre-commit Validation**
The Git pre-commit hook automatically:
1. Detects migration file changes
2. Starts test database
3. Applies migrations
4. Validates schema consistency
5. Blocks commit if validation fails

### **Manual Validation**
```bash
# Start test environment
docker-compose -f docker-compose.migration-test.yml up -d

# Wait for database to be ready
sleep 15

# Run validation
cd services/auth-service
go run cmd/migrate/main.go validate

# Run comprehensive tests
go test ./internal/models ./internal/repositories -v

# Clean up
cd ../..
docker-compose -f docker-compose.migration-test.yml down --volumes
```

---

## 🐳 **Docker Integration**

### **Migration-First Development**
```bash
# Start Migration-First environment (automatically applies migrations)
docker-compose -f docker-compose.migration-first.yml up -d

# View migration logs
docker logs postgres-auth

# Check migration status inside container
docker exec postgres-auth psql -U postgres -d auth_db -c "SELECT * FROM schema_migrations ORDER BY applied_at;"
```

### **Test Environment**
```bash
# Start test database for schema validation
docker-compose -f docker-compose.migration-test.yml up -d postgres-auth-test

# Connect to test database
docker exec -it postgres-auth-test psql -U postgres -d auth_db_test

# Clean up test environment
docker-compose -f docker-compose.migration-test.yml down --volumes
```

---

## 🔧 **VS Code Integration**

### **Available Tasks** (Ctrl+Shift+P → "Tasks: Run Task")

1. **Migration: Validate Schema** - Run schema validation
2. **Migration: Show Status** - Check migration status  
3. **Migration: Create New** - Create new migration file
4. **Migration: Apply Pending** - Apply pending migrations
5. **Test: Schema Integration** - Run schema tests with fresh DB
6. **Docker: Start Migration-First Environment** - Start production-like environment
7. **Schema: Full Validation Workflow** - Complete validation pipeline

### **Task Examples**
```json
// Quickly validate schema
"Migration: Validate Schema"

// Create new migration interactively
"Migration: Create New"  // Will prompt for migration name

// Run full validation pipeline
"Schema: Full Validation Workflow"
```

---

## ⚠️ **Common Issues and Solutions**

### **Schema Validation Fails**
```bash
❌ CRITICAL: Table structure mismatch for table 'users'
```

**Solutions:**
1. Check migration file syntax
2. Ensure GORM model matches SQL schema
3. Run migration manually: `go run cmd/migrate/main.go migrate`
4. Check column types and constraints alignment

### **Migration Apply Fails**
```bash
❌ CRITICAL: Migration failed: 002_add_notifications.sql
```

**Solutions:**
1. Check PostgreSQL logs: `docker logs postgres-auth`
2. Verify SQL syntax in migration file
3. Ensure dependencies exist (referenced tables/columns)
4. Check for constraint violations

### **Pre-commit Hook Blocks Commit**
```bash
❌ Schema validation failed. Commit blocked.
```

**Solutions:**
1. Run validation manually: `go run cmd/migrate/main.go validate`
2. Fix schema issues in migration files or GORM models
3. Test with: `docker-compose -f docker-compose.migration-test.yml up`
4. Re-attempt commit after fixes

### **Docker Database Won't Start**
```bash
postgres-auth | FATAL: password authentication failed
```

**Solutions:**
1. Set environment variable: `export AUTH_DB_PASSWORD=testpassword`
2. Check `.env` file exists with proper values
3. Clean up volumes: `docker-compose down --volumes`
4. Restart with fresh volumes

---

## 📊 **Best Practices**

### **Migration File Naming**
- Use descriptive names: `AddUserTwoFactor` not `Migration2`
- Include purpose in filename
- Use sequential numbering: `001_`, `002_`, etc.

### **Migration Content**
- Always use transactions (BEGIN/COMMIT)
- Include rollback instructions in comments
- Use `IF NOT EXISTS` for safe operations
- Add proper indexes for foreign keys
- Include data migration if needed

### **GORM Model Alignment**
- Match SQL types exactly: `VARCHAR(100)` → `gorm:"type:varchar(100)"`
- Include all constraints: `NOT NULL`, `DEFAULT`, etc.
- Use proper relationship tags: `foreignKey`, `references`
- Test model operations after schema changes

### **Testing Strategy**
- Run schema validation before every commit
- Test both positive and negative cases
- Verify foreign key constraints work
- Check index creation and performance
- Validate data migrations with real data

---

## 🚀 **Advanced Workflows**

### **Feature Branch Development**
```bash
# 1. Create feature branch
git checkout -b feature/user-notifications

# 2. Create migrations as needed
go run cmd/migrate/main.go create AddNotificationTables

# 3. Develop and validate continuously
go run cmd/migrate/main.go validate

# 4. Before merge, run full validation
bash scripts/full-schema-validation.sh

# 5. Merge with confidence
git checkout main && git merge feature/user-notifications
```

### **Production Deployment**
```bash
# 1. Use Migration-First Docker Compose
docker-compose -f docker-compose.migration-first.yml up -d

# 2. Monitor migration logs
docker logs postgres-auth -f

# 3. Verify schema status
docker exec postgres-auth psql -U postgres -d auth_db -c "SELECT COUNT(*) FROM schema_migrations;"

# 4. Run application health checks
curl http://localhost:8001/health/schema
```

---

## 📈 **Monitoring and Maintenance**

### **Schema Monitoring**
- Check migration status regularly
- Monitor schema validation in CI/CD
- Set up alerts for schema drift
- Review migration performance metrics

### **Maintenance Tasks**
- Periodically review and optimize migrations
- Clean up old migration files if safe
- Update GORM models to match schema changes
- Document significant schema changes

---

## 🆘 **Emergency Procedures**

### **Schema Drift in Production**
1. **Immediate Assessment**
   ```bash
   go run cmd/migrate/main.go status
   go run cmd/migrate/main.go validate
   ```

2. **Create Hotfix Migration**
   ```bash
   go run cmd/migrate/main.go create HotfixSchemaAlignment
   # Edit migration to match current production schema
   ```

3. **Test and Deploy**
   ```bash
   # Test thoroughly
   docker-compose -f docker-compose.migration-test.yml up -d
   go run cmd/migrate/main.go validate
   
   # Deploy with Migration-First
   docker-compose -f docker-compose.migration-first.yml up -d
   ```

### **Migration Rollback**
1. **Assess Impact**
   - Identify affected tables and data
   - Check for dependent migrations
   - Plan rollback strategy

2. **Execute Rollback**
   ```sql
   -- Manual rollback (example)
   BEGIN;
   DROP TABLE IF EXISTS problematic_table CASCADE;
   DELETE FROM schema_migrations WHERE version = '002_problematic_migration';
   COMMIT;
   ```

3. **Validate Recovery**
   ```bash
   go run cmd/migrate/main.go status
   go run cmd/migrate/main.go validate
   ```

---

## 📚 **Additional Resources**

- [SCHEMA_CONSISTENCY_PREVENTION_PLAN.md](./SCHEMA_CONSISTENCY_PREVENTION_PLAN.md) - Comprehensive prevention strategy
- [SCHEMA_ISSUES_ANALYSIS.md](./SCHEMA_ISSUES_ANALYSIS.md) - Historical issue analysis
- [GORM_CODING_STANDARDS.md](./GORM_CODING_STANDARDS.md) - GORM best practices
- [TEST_DOCUMENTATION.md](./TEST_DOCUMENTATION.md) - Testing guidelines

---

**🎯 Remember: Migration-First development ensures schema consistency and prevents production issues. Always validate before committing!**