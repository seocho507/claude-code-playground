-- ==========================================
-- Migration: 002_fix_sessions_table.sql
-- Purpose: Fix sessions table for logout functionality
-- Author: Migration Manager
-- Date: 2025-08-27
-- Environment: ALL
-- ==========================================

-- 🔄 FORWARD MIGRATION (UP)
BEGIN;

-- Add missing is_revoked column for logout functionality
ALTER TABLE sessions 
ADD COLUMN IF NOT EXISTS is_revoked BOOLEAN NOT NULL DEFAULT false;

-- Add index for is_revoked column for better query performance
CREATE INDEX IF NOT EXISTS idx_sessions_is_revoked ON sessions(is_revoked);

-- Update any existing sessions to ensure proper defaults
UPDATE sessions SET is_revoked = false WHERE is_revoked IS NULL;

COMMIT;

-- ==========================================
-- 🔙 DOWN MIGRATION (ROLLBACK)
-- ==========================================
-- To rollback this migration, run:
-- 
-- BEGIN;
-- DROP INDEX IF EXISTS idx_sessions_is_revoked;
-- ALTER TABLE sessions DROP COLUMN IF EXISTS is_revoked;
-- COMMIT;