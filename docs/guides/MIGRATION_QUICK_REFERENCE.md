# 🚀 Migration Quick Reference Card

## **Essential Commands**

```bash
# Daily workflow
cd services/auth-service
go run cmd/migrate/main.go status    # Check status
go run cmd/migrate/main.go validate  # Validate schema
go run cmd/migrate/main.go migrate   # Apply migrations

# Create new migration
go run cmd/migrate/main.go create YourMigrationName

# Test with Docker
docker-compose -f docker-compose.migration-test.yml up -d
# ... run tests ...
docker-compose -f docker-compose.migration-test.yml down --volumes
```

## **VS Code Tasks** (Ctrl+Shift+P → "Tasks: Run Task")
- `Migration: Validate Schema`
- `Migration: Create New`
- `Test: Schema Integration`
- `Schema: Full Validation Workflow`

## **Git Hooks**
```bash
# Install hooks (one time)
bash scripts/setup-git-hooks.sh

# Hooks run automatically on commit
# To bypass (not recommended): git commit --no-verify
```

## **Migration Template**
```sql
-- ==========================================
-- Migration: XXX_description.sql
-- ==========================================

BEGIN;

-- Your schema changes here
ALTER TABLE users ADD COLUMN new_field VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_users_new_field ON users(new_field);

COMMIT;
```

## **GORM Model Template**
```go
type NewModel struct {
    ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
    Name      string    `gorm:"type:varchar(100);not null"`
    CreatedAt time.Time `gorm:"not null;default:now()"`
    
    User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}
```

## **Troubleshooting**
```bash
# Schema validation fails
go run cmd/migrate/main.go validate  # Check details
docker logs postgres-auth            # Check database logs

# Migration fails
docker-compose down --volumes        # Clean slate
docker-compose -f docker-compose.migration-first.yml up -d

# Pre-commit blocked
go run cmd/migrate/main.go validate  # Fix issues first
git commit --no-verify              # Emergency bypass only
```

## **Environment Variables**
```bash
export AUTH_DB_PASSWORD=testpassword
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=auth_db
export DB_USER=postgres
```

---
**📖 Full guide:** [MIGRATION_DEVELOPER_GUIDE.md](./MIGRATION_DEVELOPER_GUIDE.md)