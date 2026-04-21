#!/usr/bin/env bash
set -euo pipefail

if ! command -v cog &>/dev/null; then
  echo "Error: cocogitto (cog) is not installed."
  echo "Install: https://docs.cocogitto.io/guide/installation"
  exit 1
fi

cog install-hook commit-msg
echo "commit-msg hook installed."

# Install post-commit hook for automatic tagging
cat > .git/hooks/post-commit <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Skip if commit message starts with "chore(version)" to avoid infinite loop
commit_msg=$(git log -1 --pretty=%B)
if [[ "$commit_msg" == "chore(version)"* ]]; then
  exit 0
fi

# Create tag without bump commit to avoid infinite loop
cog bump --auto --disable-bump-commit
EOF

chmod +x .git/hooks/post-commit
echo "post-commit hook installed (automatic tagging)."

# Install pre-commit hook for linting and testing
cat > .git/hooks/pre-commit <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

echo "Running go vet..."
if ! go vet ./...; then
  echo "go vet failed"
  exit 1
fi

echo "Running go test..."
if ! go test -race ./...; then
  echo "go test failed"
  exit 1
fi

echo "Pre-commit checks passed."
EOF

chmod +x .git/hooks/pre-commit
echo "pre-commit hook installed (go vet and go test)."
