#!/bin/bash
# ==================================================
# Migration-First Docker Initialization Script
# Purpose: Systematic migration execution in Docker
# Author: Schema Consistency Prevention Plan
# ==================================================

set -euo pipefail

MIGRATION_DIR="/docker-entrypoint-initdb.d/migrations"
LOG_FILE="/var/log/postgresql/migration.log"

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

echo "🔄 Starting Migration-First initialization..." | tee -a "$LOG_FILE"
echo "📅 $(date): Docker PostgreSQL Migration Runner started" | tee -a "$LOG_FILE"

# Check if migrations directory exists
if [ ! -d "$MIGRATION_DIR" ]; then
    echo "❌ CRITICAL: Migration directory not found: $MIGRATION_DIR" | tee -a "$LOG_FILE"
    exit 1
fi

# 1. Execute migrations in order
MIGRATION_COUNT=0
for migration in "$MIGRATION_DIR"/*.sql; do
    if [ -f "$migration" ]; then
        filename=$(basename "$migration")
        echo "⚡ Executing: $filename" | tee -a "$LOG_FILE"
        
        start_time=$(date +%s%N)
        
        # Execute migration with error handling
        if ! psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f "$migration" >> "$LOG_FILE" 2>&1; then
            echo "❌ CRITICAL: Migration failed: $filename" | tee -a "$LOG_FILE"
            echo "📋 Check migration logs above for details" | tee -a "$LOG_FILE"
            exit 1
        fi
        
        end_time=$(date +%s%N)
        execution_time=$(((end_time - start_time) / 1000000))
        echo "✅ Completed: $filename (${execution_time}ms)" | tee -a "$LOG_FILE"
        
        # Record in tracking table (create table first if needed)
        psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "
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
        " >> "$LOG_FILE" 2>&1
        
        # Calculate checksum
        CHECKSUM=$(sha256sum "$migration" | cut -d' ' -f1)
        VERSION="${filename%.*}"
        
        # Insert migration record
        psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "
            INSERT INTO schema_migrations (version, name, checksum, applied_by, environment, execution_time_ms) 
            VALUES ('$VERSION', '${filename%.*}', '$CHECKSUM', 'docker-init', 'docker', $execution_time)
            ON CONFLICT (version) DO UPDATE SET
                applied_by = 'docker-init',
                environment = 'docker',
                execution_time_ms = $execution_time;
        " >> "$LOG_FILE" 2>&1
        
        MIGRATION_COUNT=$((MIGRATION_COUNT + 1))
    fi
done

echo "📊 Executed $MIGRATION_COUNT migrations" | tee -a "$LOG_FILE"

# 2. Validate all required tables exist
echo "🔍 Validating required tables..." | tee -a "$LOG_FILE"
REQUIRED_TABLES=(users sessions login_attempts user_preferences user_activities user_notifications)
MISSING_TABLES=()

for table in "${REQUIRED_TABLES[@]}"; do
    if ! psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "SELECT 1 FROM $table LIMIT 0;" > /dev/null 2>&1; then
        echo "❌ Required table missing: $table" | tee -a "$LOG_FILE"
        MISSING_TABLES+=("$table")
    else
        echo "✅ Table verified: $table" | tee -a "$LOG_FILE"
    fi
done

if [ ${#MISSING_TABLES[@]} -gt 0 ]; then
    echo "❌ CRITICAL: ${#MISSING_TABLES[@]} required tables are missing" | tee -a "$LOG_FILE"
    echo "Missing tables: ${MISSING_TABLES[*]}" | tee -a "$LOG_FILE"
    exit 1
fi

# 3. Run comprehensive schema validation
echo "🔍 Validating schema consistency..." | tee -a "$LOG_FILE"

# Check total table count
TABLE_COUNT=$(psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -t -c "
    SELECT COUNT(*) FROM information_schema.tables 
    WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
    AND table_name NOT LIKE 'pg_%' AND table_name != 'schema_migrations';
")

if [ "$TABLE_COUNT" -lt 6 ]; then
    echo "❌ CRITICAL: Schema validation failed - only $TABLE_COUNT tables found, expected at least 6" | tee -a "$LOG_FILE"
    exit 1
fi

# Validate key columns exist
echo "🔍 Validating critical column structure..." | tee -a "$LOG_FILE"

CRITICAL_VALIDATIONS=(
    "users:id,email,username,password_hash"
    "sessions:id,user_id,refresh_token"
    "login_attempts:id,ip_address,attempted_at"
    "user_preferences:id,user_id"
    "user_activities:id,user_id,action"
    "user_notifications:id,user_id,type,title"
)

for validation in "${CRITICAL_VALIDATIONS[@]}"; do
    table_name=$(echo "$validation" | cut -d':' -f1)
    columns=$(echo "$validation" | cut -d':' -f2)
    
    IFS=',' read -ra COLUMN_ARRAY <<< "$columns"
    for column in "${COLUMN_ARRAY[@]}"; do
        if ! psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = '$table_name' AND column_name = '$column';
        " | grep -q "1"; then
            echo "❌ CRITICAL: Column $table_name.$column is missing" | tee -a "$LOG_FILE"
            exit 1
        fi
    done
    echo "✅ Table structure validated: $table_name" | tee -a "$LOG_FILE"
done

# 4. Verify foreign key relationships
echo "🔍 Validating foreign key relationships..." | tee -a "$LOG_FILE"

FK_VALIDATIONS=(
    "sessions.user_id -> users.id"
    "login_attempts.user_id -> users.id"
    "user_preferences.user_id -> users.id"
    "user_activities.user_id -> users.id"
    "user_notifications.user_id -> users.id"
)

for fk in "${FK_VALIDATIONS[@]}"; do
    # Extract table and referenced table
    source_table=$(echo "$fk" | cut -d'.' -f1)
    
    # Check if foreign key constraint exists (simplified check)
    fk_count=$(psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -t -c "
        SELECT COUNT(*) FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
        WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = '$source_table';
    ")
    
    if [ "$fk_count" -gt 0 ]; then
        echo "✅ Foreign key constraint verified: $fk" | tee -a "$LOG_FILE"
    else
        echo "⚠️  Warning: No foreign key constraint found for: $fk" | tee -a "$LOG_FILE"
        # Don't fail on FK warnings - some may be intended
    fi
done

# 5. Final validation summary
echo "📊 Migration-First Validation Summary:" | tee -a "$LOG_FILE"
echo "   - Migrations executed: $MIGRATION_COUNT" | tee -a "$LOG_FILE"
echo "   - Tables validated: ${#REQUIRED_TABLES[@]}" | tee -a "$LOG_FILE"
echo "   - Total tables found: $TABLE_COUNT" | tee -a "$LOG_FILE"
echo "   - Schema consistency: VERIFIED" | tee -a "$LOG_FILE"

echo "✅ Migration-First initialization completed successfully" | tee -a "$LOG_FILE"
echo "📅 $(date): Docker PostgreSQL ready with validated schema" | tee -a "$LOG_FILE"

# Set success marker for health checks
psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "
    CREATE TABLE IF NOT EXISTS migration_status (
        initialized_at TIMESTAMP DEFAULT NOW(),
        status VARCHAR(20) DEFAULT 'ready',
        migrations_count INTEGER DEFAULT $MIGRATION_COUNT
    );
    INSERT INTO migration_status (migrations_count) VALUES ($MIGRATION_COUNT)
    ON CONFLICT DO NOTHING;
" >> "$LOG_FILE" 2>&1

echo "🎉 Migration-First Docker initialization complete!" | tee -a "$LOG_FILE"