-- ==========================================
-- Migration: 001_initial_schema.sql
-- Purpose: Complete initial schema for Auth Service
-- Author: Migration Manager
-- Date: 2025-08-26
-- Environment: ALL
-- ==========================================

-- 🔄 FORWARD MIGRATION (UP)
BEGIN;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enum types
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM ('user', 'admin', 'moderator');
    END IF;
END $$;

-- ==========================================
-- USERS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Authentication fields
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    
    -- User status and role
    role user_role NOT NULL DEFAULT 'user',
    is_active BOOLEAN NOT NULL DEFAULT true,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    
    -- OAuth fields
    google_id VARCHAR(255),
    git_hub_id VARCHAR(255), -- Note: GORM creates this as git_hub_id, not github_id
    facebook_id VARCHAR(255),
    
    -- Profile fields (integrated from UserProfile)
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    phone_number VARCHAR(20),
    bio TEXT,
    avatar_url VARCHAR(500),
    date_of_birth TIMESTAMP,
    gender VARCHAR(20),
    country VARCHAR(100),
    city VARCHAR(100),
    timezone VARCHAR(50),
    language VARCHAR(10) DEFAULT 'en',
    website VARCHAR(500),
    
    -- Social media fields
    linkedin VARCHAR(500),
    twitter VARCHAR(500),
    github VARCHAR(500), -- This is separate from git_hub_id (OAuth)
    
    -- Security and tracking
    last_login_at TIMESTAMP,
    last_login_ip INET,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Create indexes for users table
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_github_id ON users(git_hub_id) WHERE git_hub_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_facebook_id ON users(facebook_id) WHERE facebook_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_phone_number ON users(phone_number) WHERE phone_number IS NOT NULL;

-- ==========================================
-- SESSIONS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Session data
    refresh_token VARCHAR(512) UNIQUE NOT NULL,
    access_token_hash VARCHAR(255),
    
    -- Session metadata
    ip_address INET,
    user_agent TEXT,
    device_info JSONB,
    
    -- Session status
    is_active BOOLEAN NOT NULL DEFAULT true,
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP
);

-- Create indexes for sessions table
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions(refresh_token);
CREATE INDEX IF NOT EXISTS idx_sessions_is_active ON sessions(is_active);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);

-- ==========================================
-- LOGIN ATTEMPTS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS login_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- User identification (may be null for failed attempts)
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255), -- Store email for failed attempts
    username VARCHAR(100), -- Store username for failed attempts
    
    -- Attempt details
    success BOOLEAN NOT NULL DEFAULT false,
    failure_reason VARCHAR(255),
    
    -- Request metadata
    ip_address INET NOT NULL,
    user_agent TEXT,
    
    -- Timestamps
    attempted_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for login_attempts table
CREATE INDEX IF NOT EXISTS idx_login_attempts_user_id ON login_attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_login_attempts_email ON login_attempts(email);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_address ON login_attempts(ip_address);
CREATE INDEX IF NOT EXISTS idx_login_attempts_attempted_at ON login_attempts(attempted_at);
CREATE INDEX IF NOT EXISTS idx_login_attempts_success ON login_attempts(success);

-- ==========================================
-- USER PREFERENCES TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS user_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Notification preferences
    email_notifications BOOLEAN NOT NULL DEFAULT true,
    push_notifications BOOLEAN NOT NULL DEFAULT true,
    marketing_emails BOOLEAN NOT NULL DEFAULT false,
    
    -- Security preferences
    two_factor_enabled BOOLEAN NOT NULL DEFAULT false,
    
    -- UI preferences
    theme VARCHAR(20) NOT NULL DEFAULT 'light',
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    
    -- Privacy preferences
    privacy_level VARCHAR(20) NOT NULL DEFAULT 'normal',
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for user_preferences table
CREATE INDEX IF NOT EXISTS idx_user_preferences_user_id ON user_preferences(user_id);
CREATE INDEX IF NOT EXISTS idx_user_preferences_theme ON user_preferences(theme);
CREATE INDEX IF NOT EXISTS idx_user_preferences_privacy_level ON user_preferences(privacy_level);

-- Add constraints for user_preferences
ALTER TABLE user_preferences 
ADD CONSTRAINT check_valid_theme 
CHECK (theme IN ('light', 'dark', 'auto'));

ALTER TABLE user_preferences 
ADD CONSTRAINT check_valid_privacy_level 
CHECK (privacy_level IN ('private', 'normal', 'public'));

-- ==========================================
-- USER ACTIVITIES TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS user_activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Activity details
    action VARCHAR(100) NOT NULL,
    description TEXT,
    
    -- Request metadata
    ip_address INET,
    user_agent TEXT,
    
    -- Additional data
    metadata JSONB DEFAULT '{}',
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for user_activities table
CREATE INDEX IF NOT EXISTS idx_user_activities_user_id ON user_activities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_activities_action ON user_activities(action);
CREATE INDEX IF NOT EXISTS idx_user_activities_created_at ON user_activities(created_at);
CREATE INDEX IF NOT EXISTS idx_user_activities_metadata ON user_activities USING GIN(metadata);

-- ==========================================
-- USER NOTIFICATIONS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS user_notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Notification content
    type VARCHAR(50) NOT NULL,
    title VARCHAR(200) NOT NULL,
    message TEXT,
    
    -- Notification status
    is_read BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMP,
    
    -- Action information
    action_url VARCHAR(500),
    action_text VARCHAR(100),
    
    -- Lifecycle
    expires_at TIMESTAMP,
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for user_notifications table
CREATE INDEX IF NOT EXISTS idx_user_notifications_user_id ON user_notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_user_notifications_type ON user_notifications(type);
CREATE INDEX IF NOT EXISTS idx_user_notifications_is_read ON user_notifications(is_read);
CREATE INDEX IF NOT EXISTS idx_user_notifications_created_at ON user_notifications(created_at);
CREATE INDEX IF NOT EXISTS idx_user_notifications_expires_at ON user_notifications(expires_at);

-- Add constraints for user_notifications
ALTER TABLE user_notifications 
ADD CONSTRAINT check_notification_type 
CHECK (type IN ('info', 'warning', 'error', 'success', 'promotion', 'reminder', 'system'));

-- ==========================================
-- TRIGGERS FOR UPDATED_AT TIMESTAMPS
-- ==========================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at 
    BEFORE UPDATE ON sessions 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_preferences_updated_at 
    BEFORE UPDATE ON user_preferences 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;

-- ==========================================
-- 🔙 DOWN MIGRATION (ROLLBACK)
-- ==========================================
-- To rollback this migration, run:
-- 
-- BEGIN;
-- 
-- DROP TRIGGER IF EXISTS update_user_preferences_updated_at ON user_preferences;
-- DROP TRIGGER IF EXISTS update_sessions_updated_at ON sessions;
-- DROP TRIGGER IF EXISTS update_users_updated_at ON users;
-- DROP FUNCTION IF EXISTS update_updated_at_column();
-- 
-- DROP TABLE IF EXISTS user_notifications CASCADE;
-- DROP TABLE IF EXISTS user_activities CASCADE;
-- DROP TABLE IF EXISTS user_preferences CASCADE;
-- DROP TABLE IF EXISTS login_attempts CASCADE;
-- DROP TABLE IF EXISTS sessions CASCADE;
-- DROP TABLE IF EXISTS users CASCADE;
-- 
-- DROP TYPE IF EXISTS user_role;
-- DROP EXTENSION IF EXISTS "uuid-ossp";
-- 
-- COMMIT;-- Test migration change for E2E validation
