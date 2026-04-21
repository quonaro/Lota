#!/usr/bin/env bash
set -euo pipefail

# setup-dev.sh - Install all development hooks (pre-commit and cog)

echo "🔧 Setting up development environment..."

# Check if pre-commit is installed
if ! command -v pre-commit &> /dev/null; then
    echo "❌ pre-commit not found. Install it first:"
    echo "   pip install pre-commit"
    exit 1
fi

# Check if cog is installed
if ! command -v cog &> /dev/null; then
    echo "❌ cog not found. Install it first:"
    echo "   cargo install cocogitto"
    exit 1
fi

# Install pre-commit hooks
echo "📦 Installing pre-commit hooks..."
pre-commit install

# Install cog hooks
echo "📦 Installing cog hooks..."
cog install-hook

echo "✅ Development hooks installed successfully!"
