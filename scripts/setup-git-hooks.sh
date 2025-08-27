#!/bin/bash
# ===================================================================
# Git Hooks Installation Script
# Purpose: Install schema validation hooks for development workflow
# Author: Schema Consistency Prevention Plan
# ===================================================================

set -e

echo "🔧 Setting up Git hooks for schema validation..."

# Get the project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOOKS_SOURCE_DIR="$PROJECT_ROOT/.githooks"
HOOKS_TARGET_DIR="$PROJECT_ROOT/.git/hooks"

# Check if we're in a Git repository
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo "⚠️  Not in a Git repository. Initializing Git repository..."
    cd "$PROJECT_ROOT"
    git init
    echo "✅ Git repository initialized"
fi

# Check if hooks source directory exists
if [ ! -d "$HOOKS_SOURCE_DIR" ]; then
    echo "❌ Hooks source directory not found: $HOOKS_SOURCE_DIR"
    exit 1
fi

# Create hooks target directory if it doesn't exist
mkdir -p "$HOOKS_TARGET_DIR"

# Install pre-commit hook
if [ -f "$HOOKS_SOURCE_DIR/pre-commit" ]; then
    echo "📝 Installing pre-commit hook..."
    cp "$HOOKS_SOURCE_DIR/pre-commit" "$HOOKS_TARGET_DIR/pre-commit"
    chmod +x "$HOOKS_TARGET_DIR/pre-commit"
    echo "✅ Pre-commit hook installed"
else
    echo "❌ Pre-commit hook source not found"
    exit 1
fi

# Verify installation
if [ -x "$HOOKS_TARGET_DIR/pre-commit" ]; then
    echo "✅ Git hooks installation completed successfully"
    echo ""
    echo "📋 Hooks installed:"
    echo "   ✓ pre-commit - Schema validation before commits"
    echo ""
    echo "💡 Usage:"
    echo "   - Hooks will automatically run on 'git commit'"
    echo "   - To bypass (not recommended): git commit --no-verify"
    echo "   - To test manually: .git/hooks/pre-commit"
    echo ""
    echo "🔍 The pre-commit hook will:"
    echo "   1. Validate schema migrations when migration files change"
    echo "   2. Check for model-migration alignment when model files change"
    echo "   3. Provide warnings for Docker configuration changes"
    echo "   4. Suggest integration tests for schema-related changes"
else
    echo "❌ Hook installation failed"
    exit 1
fi

# Optional: Set up Git hooks path for the repository
echo "🔧 Configuring Git hooks path..."
git config core.hooksPath .githooks

echo ""
echo "🎉 Git hooks setup complete!"
echo "🚀 Your repository is now protected against schema inconsistencies"