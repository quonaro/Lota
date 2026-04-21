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

# Install pre-commit hook for checksums verification
cat > .git/hooks/pre-commit <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Check if dist directory contains release archives
if [ -d "dist" ]; then
  ARCHIVES=$(find dist -name "lota-*.tar.gz" -type f 2>/dev/null || true)
  
  if [ -n "$ARCHIVES" ]; then
    CHECKSUM_FILE="dist/checksums.txt"
    NEEDS_UPDATE=0
    
    # Check if checksums.txt exists
    if [ ! -f "$CHECKSUM_FILE" ]; then
      NEEDS_UPDATE=1
      echo "checksums.txt not found, will generate..."
    else
      # Check if any archive is newer than checksums.txt
      for archive in $ARCHIVES; do
        if [ "$archive" -nt "$CHECKSUM_FILE" ]; then
          NEEDS_UPDATE=1
          echo "Archive $archive is newer than checksums.txt"
          break
        fi
      done
      
      # Verify existing checksums are valid
      if [ "$NEEDS_UPDATE" -eq 0 ] && command -v sha256sum >/dev/null 2>&1; then
        if ! (cd dist && sha256sum -c checksums.txt >/dev/null 2>&1); then
          NEEDS_UPDATE=1
          echo "Checksums verification failed, will regenerate..."
        fi
      fi
    fi
    
    if [ "$NEEDS_UPDATE" -eq 1 ]; then
      echo "Generating checksums.txt for release archives..."
      (cd dist && sha256sum lota-*.tar.gz > checksums.txt)
      git add "$CHECKSUM_FILE"
      echo "Updated and staged $CHECKSUM_FILE"
    fi
  fi
fi
EOF

chmod +x .git/hooks/pre-commit
echo "pre-commit hook installed (checksums verification for releases)."
