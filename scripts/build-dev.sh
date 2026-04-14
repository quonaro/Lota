#!/usr/bin/env bash
set -euo pipefail

MODULE="lota"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-s -w"
LDFLAGS+=" -X ${MODULE}/shared.Version=${VERSION}"
LDFLAGS+=" -X ${MODULE}/shared.Commit=${COMMIT}"
LDFLAGS+=" -X ${MODULE}/shared.BuildDate=${BUILD_DATE}"

CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags "${LDFLAGS}" \
  -o lota \
  .

mkdir -p ~/local/bin
cp lota ~/local/bin/

# Add to PATH if not already there
SHELL_CONFIG="$HOME/.zshrc"
[[ "$SHELL" == *"bash"* ]] && SHELL_CONFIG="$HOME/.bashrc"

PATH_LINE='export PATH="$HOME/local/bin:$PATH"'

if ! grep -q 'export PATH="\$HOME/local/bin:\$PATH"' "$SHELL_CONFIG" 2>/dev/null; then
  echo "" >> "$SHELL_CONFIG"
  echo "# lota" >> "$SHELL_CONFIG"
  echo "$PATH_LINE" >> "$SHELL_CONFIG"
  echo "Added to $SHELL_CONFIG"
  echo "Run: source $SHELL_CONFIG"
else
  echo "Already in $SHELL_CONFIG"
fi

