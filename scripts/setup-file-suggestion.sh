#!/bin/bash
# setup-file-suggestion.sh - Install file suggestion script and configure Claude Code
# Usage: ./setup-file-suggestion.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
SETTINGS_FILE="$CLAUDE_DIR/settings.json"
TARGET_SCRIPT="$CLAUDE_DIR/file-suggestion.sh"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "Setting up turbo file suggestion..."

# Ensure .claude directory exists
mkdir -p "$CLAUDE_DIR"

# Copy the file suggestion script
cp "$SCRIPT_DIR/file-suggestion.sh" "$TARGET_SCRIPT"
chmod +x "$TARGET_SCRIPT"
echo -e "${GREEN}✓${NC} Installed file-suggestion.sh to $TARGET_SCRIPT"

# Create settings.json if it doesn't exist
if [ ! -f "$SETTINGS_FILE" ]; then
  echo "{}" > "$SETTINGS_FILE"
fi

# Use jq to add/update fileSuggestion config
UPDATED=$(jq '
  .fileSuggestion = {
    "type": "command",
    "command": "~/.claude/file-suggestion.sh"
  }
' "$SETTINGS_FILE")

echo "$UPDATED" > "$SETTINGS_FILE"

echo -e "${GREEN}✓${NC} Configured fileSuggestion in $SETTINGS_FILE"
echo ""
echo "File suggestion is now using turbo search!"
echo "Restart Claude Code to apply changes."
