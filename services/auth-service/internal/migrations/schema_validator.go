package migrations

import (
	"auth-service/internal/models"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// SchemaValidator validates database schema consistency
type SchemaValidator struct {
	db        *gorm.DB
	sqlDB     *sql.DB
	tableName string
}

// SchemaValidationResult contains validation results
type SchemaValidationResult struct {
	TableName         string                    `json:"table_name"`
	IsValid           bool                     `json:"is_valid"`
	MissingColumns    []string                 `json:"missing_columns"`
	ExtraColumns      []string                 `json:"extra_columns"`
	TypeMismatches    []ColumnTypeMismatch     `json:"type_mismatches"`
	MissingIndexes    []string                 `json:"missing_indexes"`
	ExtraIndexes      []string                 `json:"extra_indexes"`
	ConstraintIssues  []ConstraintIssue        `json:"constraint_issues"`
	RecommendedActions []string                `json:"recommended_actions"`
}

// ColumnTypeMismatch represents a column type mismatch
type ColumnTypeMismatch struct {
	ColumnName   string `json:"column_name"`
	ExpectedType string `json:"expected_type"`
	ActualType   string `json:"actual_type"`
}

// ConstraintIssue represents a database constraint issue
type ConstraintIssue struct {
	ConstraintName string `json:"constraint_name"`
	Issue          string `json:"issue"`
	Severity       string `json:"severity"` // "error", "warning", "info"
}

// DatabaseColumn represents a database column schema
type DatabaseColumn struct {
	ColumnName           string
	DataType             string
	IsNullable           string
	ColumnDefault        sql.NullString
	CharacterMaximumLength sql.NullInt64
	NumericPrecision     sql.NullInt64
	NumericScale         sql.NullInt64
}

// DatabaseIndex represents a database index
type DatabaseIndex struct {
	IndexName   string
	ColumnName  string
	IsUnique    bool
	IsPrimary   bool
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator(db *gorm.DB) (*SchemaValidator, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	return &SchemaValidator{
		db:    db,
		sqlDB: sqlDB,
	}, nil
}

// ValidateAllTables validates all model tables against database schema
func (sv *SchemaValidator) ValidateAllTables() ([]*SchemaValidationResult, error) {
	modelTables := map[string]interface{}{
		"users":              &models.User{},
		"sessions":           &models.Session{},
		"login_attempts":     &models.LoginAttempt{},
		"user_preferences":   &models.UserPreference{},
		"user_activities":    &models.UserActivity{},
		"user_notifications": &models.UserNotification{},
	}

	var results []*SchemaValidationResult

	for tableName, model := range modelTables {
		result, err := sv.ValidateTable(tableName, model)
		if err != nil {
			log.Printf("❌ Failed to validate table %s: %v", tableName, err)
			result = &SchemaValidationResult{
				TableName: tableName,
				IsValid:   false,
				RecommendedActions: []string{
					fmt.Sprintf("Manual investigation required: %v", err),
				},
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// ValidateTable validates a specific table against its GORM model
func (sv *SchemaValidator) ValidateTable(tableName string, model interface{}) (*SchemaValidationResult, error) {
	result := &SchemaValidationResult{
		TableName: tableName,
		IsValid:   true,
	}

	// Check if table exists
	exists, err := sv.tableExists(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if table exists: %w", err)
	}
	
	if !exists {
		result.IsValid = false
		result.RecommendedActions = append(result.RecommendedActions,
			fmt.Sprintf("Create table '%s' using migration", tableName))
		return result, nil
	}

	// Get database schema
	dbColumns, err := sv.getDatabaseColumns(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get database columns: %w", err)
	}

	// Get GORM model schema
	modelColumns := sv.getModelColumns(model)

	// Compare columns
	sv.compareColumns(result, modelColumns, dbColumns)

	// Validate indexes
	sv.validateIndexes(result, tableName, model)

	// Validate constraints
	sv.validateConstraints(result, tableName)

	// Generate recommendations
	sv.generateRecommendations(result)

	return result, nil
}

// tableExists checks if a table exists in the database
func (sv *SchemaValidator) tableExists(tableName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)
	`
	
	var exists bool
	err := sv.sqlDB.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// getDatabaseColumns retrieves column information from database
func (sv *SchemaValidator) getDatabaseColumns(tableName string) (map[string]*DatabaseColumn, error) {
	query := `
		SELECT 
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length,
			numeric_precision,
			numeric_scale
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := sv.sqlDB.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query database columns: %w", err)
	}
	defer rows.Close()

	columns := make(map[string]*DatabaseColumn)
	
	for rows.Next() {
		col := &DatabaseColumn{}
		err := rows.Scan(
			&col.ColumnName,
			&col.DataType,
			&col.IsNullable,
			&col.ColumnDefault,
			&col.CharacterMaximumLength,
			&col.NumericPrecision,
			&col.NumericScale,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		
		columns[col.ColumnName] = col
	}

	return columns, nil
}

// getModelColumns extracts expected columns from GORM model
func (sv *SchemaValidator) getModelColumns(model interface{}) map[string]string {
	columns := make(map[string]string)
	
	stmt := &gorm.Statement{DB: sv.db}
	stmt.Parse(model)
	
	for _, field := range stmt.Schema.Fields {
		if field.DBName != "" {
			// Map Go type to expected database type
			dbType := sv.mapGoTypeToDBType(field)
			columns[field.DBName] = dbType
		}
	}
	
	return columns
}

// mapGoTypeToDBType maps Go field types to expected database types
func (sv *SchemaValidator) mapGoTypeToDBType(field *schema.Field) string {
	// This is a simplified mapping - in real implementation, 
	// this would be more comprehensive
	switch field.DataType {
	case schema.String:
		if field.Size > 0 && field.Size <= 255 {
			return fmt.Sprintf("character varying(%d)", field.Size)
		}
		return "text"
	case schema.Bool:
		return "boolean"
	case schema.Int, schema.Uint:
		return "integer"
	case schema.Time:
		return "timestamp without time zone"
	case schema.Bytes:
		return "bytea"
	default:
		return "unknown"
	}
}

// compareColumns compares model columns with database columns
func (sv *SchemaValidator) compareColumns(result *SchemaValidationResult, modelCols map[string]string, dbCols map[string]*DatabaseColumn) {
	// Check for missing columns (in model but not in database)
	for colName := range modelCols {
		if _, exists := dbCols[colName]; !exists {
			result.MissingColumns = append(result.MissingColumns, colName)
			result.IsValid = false
		}
	}

	// Check for extra columns (in database but not in model)  
	for colName := range dbCols {
		if _, exists := modelCols[colName]; !exists {
			// Ignore system columns
			if !sv.isSystemColumn(colName) {
				result.ExtraColumns = append(result.ExtraColumns, colName)
			}
		}
	}

	// Check for type mismatches
	for colName, expectedType := range modelCols {
		if dbCol, exists := dbCols[colName]; exists {
			if !sv.typesMatch(expectedType, dbCol.DataType) {
				result.TypeMismatches = append(result.TypeMismatches, ColumnTypeMismatch{
					ColumnName:   colName,
					ExpectedType: expectedType,
					ActualType:   dbCol.DataType,
				})
				result.IsValid = false
			}
		}
	}
}

// isSystemColumn checks if a column is a system column
func (sv *SchemaValidator) isSystemColumn(colName string) bool {
	systemColumns := []string{
		"oid", "tableoid", "xmin", "cmin", "xmax", "cmax", "ctid",
	}
	
	for _, sysCol := range systemColumns {
		if colName == sysCol {
			return true
		}
	}
	return false
}

// typesMatch checks if expected and actual types are compatible
func (sv *SchemaValidator) typesMatch(expected, actual string) bool {
	// Simplified type matching - in real implementation this would be more comprehensive
	expected = strings.ToLower(strings.TrimSpace(expected))
	actual = strings.ToLower(strings.TrimSpace(actual))
	
	// Direct match
	if expected == actual {
		return true
	}
	
	// Handle common variations
	variations := map[string][]string{
		"character varying": {"varchar", "text"},
		"timestamp without time zone": {"timestamp", "datetime"},
		"boolean": {"bool"},
		"integer": {"int", "int4"},
		"bigint": {"int8"},
		"uuid": {"char(36)"},
	}
	
	for baseType, aliases := range variations {
		if strings.Contains(expected, baseType) {
			for _, alias := range aliases {
				if strings.Contains(actual, alias) {
					return true
				}
			}
		}
	}
	
	return false
}

// validateIndexes validates table indexes
func (sv *SchemaValidator) validateIndexes(result *SchemaValidationResult, tableName string, model interface{}) {
	// Get database indexes
	dbIndexes, err := sv.getDatabaseIndexes(tableName)
	if err != nil {
		log.Printf("Warning: Failed to get database indexes for %s: %v", tableName, err)
		return
	}

	// Get expected indexes from GORM model
	expectedIndexes := sv.getModelIndexes(model)

	// Compare indexes
	for indexName := range expectedIndexes {
		if !sv.indexExists(indexName, dbIndexes) {
			result.MissingIndexes = append(result.MissingIndexes, indexName)
		}
	}

	// Note: We don't check for extra indexes as they might be beneficial
}

// getDatabaseIndexes retrieves index information from database
func (sv *SchemaValidator) getDatabaseIndexes(tableName string) (map[string]*DatabaseIndex, error) {
	query := `
		SELECT
			i.relname as index_name,
			a.attname as column_name,
			ix.indisunique as is_unique,
			ix.indisprimary as is_primary
		FROM
			pg_class t,
			pg_class i,
			pg_index ix,
			pg_attribute a
		WHERE
			t.oid = ix.indrelid
			AND i.oid = ix.indexrelid
			AND a.attrelid = t.oid
			AND a.attnum = ANY(ix.indkey)
			AND t.relkind = 'r'
			AND t.relname = $1
		ORDER BY t.relname, i.relname
	`

	rows, err := sv.sqlDB.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query database indexes: %w", err)
	}
	defer rows.Close()

	indexes := make(map[string]*DatabaseIndex)
	
	for rows.Next() {
		idx := &DatabaseIndex{}
		err := rows.Scan(&idx.IndexName, &idx.ColumnName, &idx.IsUnique, &idx.IsPrimary)
		if err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}
		
		indexes[idx.IndexName] = idx
	}

	return indexes, nil
}

// getModelIndexes extracts expected indexes from GORM model
func (sv *SchemaValidator) getModelIndexes(model interface{}) map[string]bool {
	indexes := make(map[string]bool)
	
	stmt := &gorm.Statement{DB: sv.db}
	stmt.Parse(model)
	
	for _, index := range stmt.Schema.ParseIndexes() {
		indexes[index.Name] = true
	}
	
	return indexes
}

// indexExists checks if an index exists in the database indexes map
func (sv *SchemaValidator) indexExists(indexName string, dbIndexes map[string]*DatabaseIndex) bool {
	_, exists := dbIndexes[indexName]
	return exists
}

// validateConstraints validates table constraints
func (sv *SchemaValidator) validateConstraints(result *SchemaValidationResult, tableName string) {
	// Check foreign key constraints
	fkConstraints, err := sv.getForeignKeyConstraints(tableName)
	if err != nil {
		log.Printf("Warning: Failed to get FK constraints for %s: %v", tableName, err)
		return
	}

	// Validate expected foreign keys
	expectedFK := sv.getExpectedForeignKeys(tableName)
	
	for fkName, expected := range expectedFK {
		if actual, exists := fkConstraints[fkName]; !exists {
			result.ConstraintIssues = append(result.ConstraintIssues, ConstraintIssue{
				ConstraintName: fkName,
				Issue:          "Missing foreign key constraint",
				Severity:       "error",
			})
			result.IsValid = false
		} else if actual != expected {
			result.ConstraintIssues = append(result.ConstraintIssues, ConstraintIssue{
				ConstraintName: fkName,
				Issue:          fmt.Sprintf("FK constraint mismatch: expected %s, got %s", expected, actual),
				Severity:       "warning",
			})
		}
	}
}

// getForeignKeyConstraints retrieves foreign key constraints from database
func (sv *SchemaValidator) getForeignKeyConstraints(tableName string) (map[string]string, error) {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name || ' -> ' || ccu.table_name || '(' || ccu.column_name || ')' as constraint_def
		FROM 
			information_schema.table_constraints AS tc 
			JOIN information_schema.key_column_usage AS kcu
				ON tc.constraint_name = kcu.constraint_name
			JOIN information_schema.constraint_column_usage AS ccu
				ON ccu.constraint_name = tc.constraint_name
		WHERE 
			tc.constraint_type = 'FOREIGN KEY' 
			AND tc.table_name = $1
	`

	rows, err := sv.sqlDB.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query FK constraints: %w", err)
	}
	defer rows.Close()

	constraints := make(map[string]string)
	
	for rows.Next() {
		var constraintName, constraintDef string
		err := rows.Scan(&constraintName, &constraintDef)
		if err != nil {
			return nil, fmt.Errorf("failed to scan FK constraint: %w", err)
		}
		
		constraints[constraintName] = constraintDef
	}

	return constraints, nil
}

// getExpectedForeignKeys returns expected foreign key constraints for a table
func (sv *SchemaValidator) getExpectedForeignKeys(tableName string) map[string]string {
	expectedFK := make(map[string]string)
	
	switch tableName {
	case "sessions":
		expectedFK["sessions_user_id_fkey"] = "user_id -> users(id)"
	case "login_attempts":
		expectedFK["login_attempts_user_id_fkey"] = "user_id -> users(id)"
	case "user_preferences":
		expectedFK["user_preferences_user_id_fkey"] = "user_id -> users(id)"
	case "user_activities":
		expectedFK["user_activities_user_id_fkey"] = "user_id -> users(id)"
	case "user_notifications":
		expectedFK["user_notifications_user_id_fkey"] = "user_id -> users(id)"
	}
	
	return expectedFK
}

// generateRecommendations generates actionable recommendations
func (sv *SchemaValidator) generateRecommendations(result *SchemaValidationResult) {
	if len(result.MissingColumns) > 0 {
		result.RecommendedActions = append(result.RecommendedActions,
			fmt.Sprintf("Run migration to add missing columns: %s", 
				strings.Join(result.MissingColumns, ", ")))
	}
	
	if len(result.ExtraColumns) > 0 {
		result.RecommendedActions = append(result.RecommendedActions,
			fmt.Sprintf("Consider removing unused columns: %s", 
				strings.Join(result.ExtraColumns, ", ")))
	}
	
	if len(result.TypeMismatches) > 0 {
		result.RecommendedActions = append(result.RecommendedActions,
			"Review and fix column type mismatches")
	}
	
	if len(result.MissingIndexes) > 0 {
		result.RecommendedActions = append(result.RecommendedActions,
			"Create missing indexes for better performance")
	}
	
	if len(result.ConstraintIssues) > 0 {
		errorCount := 0
		for _, issue := range result.ConstraintIssues {
			if issue.Severity == "error" {
				errorCount++
			}
		}
		if errorCount > 0 {
			result.RecommendedActions = append(result.RecommendedActions,
				fmt.Sprintf("Fix %d critical constraint issues", errorCount))
		}
	}
}

// GenerateValidationReport generates a comprehensive validation report
func (sv *SchemaValidator) GenerateValidationReport() (string, error) {
	results, err := sv.ValidateAllTables()
	if err != nil {
		return "", fmt.Errorf("failed to validate tables: %w", err)
	}

	var report strings.Builder
	report.WriteString("# Database Schema Validation Report\n\n")
	report.WriteString(fmt.Sprintf("Generated at: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	validCount := 0
	for _, result := range results {
		if result.IsValid {
			validCount++
		}
	}

	report.WriteString(fmt.Sprintf("## Summary\n"))
	report.WriteString(fmt.Sprintf("- Total tables: %d\n", len(results)))
	report.WriteString(fmt.Sprintf("- Valid tables: %d\n", validCount))
	report.WriteString(fmt.Sprintf("- Invalid tables: %d\n\n", len(results)-validCount))

	for _, result := range results {
		status := "✅ VALID"
		if !result.IsValid {
			status = "❌ INVALID"
		}
		
		report.WriteString(fmt.Sprintf("## Table: %s %s\n\n", result.TableName, status))
		
		if len(result.MissingColumns) > 0 {
			report.WriteString("**Missing Columns:**\n")
			for _, col := range result.MissingColumns {
				report.WriteString(fmt.Sprintf("- %s\n", col))
			}
			report.WriteString("\n")
		}
		
		if len(result.TypeMismatches) > 0 {
			report.WriteString("**Type Mismatches:**\n")
			for _, mismatch := range result.TypeMismatches {
				report.WriteString(fmt.Sprintf("- %s: expected %s, got %s\n", 
					mismatch.ColumnName, mismatch.ExpectedType, mismatch.ActualType))
			}
			report.WriteString("\n")
		}
		
		if len(result.RecommendedActions) > 0 {
			report.WriteString("**Recommended Actions:**\n")
			for _, action := range result.RecommendedActions {
				report.WriteString(fmt.Sprintf("- %s\n", action))
			}
			report.WriteString("\n")
		}
	}

	return report.String(), nil
}