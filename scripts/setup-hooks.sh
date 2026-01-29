#!/bin/bash
# setup-hooks.sh - Configure Claude Code hooks for automatic context injection
#
# Usage:
#   ./setup-hooks.sh          # Install simple hook (suggests file paths)
#   ./setup-hooks.sh --rag    # Install RAG hook (injects actual content)
#   ./setup-hooks.sh --remove # Remove all turbo-search hooks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(dirname "$SCRIPT_DIR")"
CLAUDE_DIR="$HOME/.claude"
SETTINGS_FILE="$CLAUDE_DIR/settings.json"
BACKUP_DIR="$CLAUDE_DIR/backups"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Parse arguments
HOOK_MODE="simple"
if [ "$1" = "--rag" ]; then
  HOOK_MODE="rag"
elif [ "$1" = "--remove" ]; then
  echo "Removing turbo-search hooks..."

  if [ -f "$SETTINGS_FILE" ]; then
    # Remove our hooks from UserPromptSubmit (new format with matchers)
    UPDATED=$(jq '
      if .hooks.UserPromptSubmit then
        .hooks.UserPromptSubmit = [
          .hooks.UserPromptSubmit[] |
          select(.hooks | all(.command | (contains("pre-prompt-search") or contains("rag-context-hook")) | not))
        ] |
        # Remove empty UserPromptSubmit array
        if (.hooks.UserPromptSubmit | length) == 0 then del(.hooks.UserPromptSubmit) else . end |
        # Remove empty hooks object
        if (.hooks | keys | length) == 0 then del(.hooks) else . end
      else . end
    ' "$SETTINGS_FILE" 2>/dev/null || cat "$SETTINGS_FILE")
    echo "$UPDATED" > "$SETTINGS_FILE"
    echo -e "${GREEN}✓${NC} Hooks removed from settings.json"
  fi

  exit 0
elif [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
  echo "Usage: $0 [--rag|--remove|--help]"
  echo ""
  echo "Options:"
  echo "  (none)    Install simple hook (suggests file paths)"
  echo "  --rag     Install RAG hook (injects actual content snippets)"
  echo "  --remove  Remove all turbo-search hooks"
  echo "  --help    Show this help message"
  echo ""
  echo "The RAG hook provides better context but uses more tokens per prompt."
  echo "The simple hook is lightweight and just suggests relevant files."
  exit 0
fi

echo -e "${BLUE}Setting up turbo-search hooks (mode: $HOOK_MODE)...${NC}"
echo ""

# Ensure .claude directory exists
mkdir -p "$CLAUDE_DIR"

# Create settings.json if it doesn't exist
if [ ! -f "$SETTINGS_FILE" ]; then
  echo "{}" > "$SETTINGS_FILE"
fi

# Determine the hook script path
if [ "$HOOK_MODE" = "rag" ]; then
  HOOK_NAME="rag-context-hook.sh"
  HOOK_DESC="RAG context injection (injects content snippets)"
else
  HOOK_NAME="pre-prompt-search.sh"
  HOOK_DESC="Simple file suggestion (suggests relevant paths)"
fi

HOOK_SCRIPT=""
for path in \
  "$HOME/.claude/plugins/"*"/claude-turbo-search/hooks/$HOOK_NAME" \
  "$HOME/claude-turbo-search/hooks/$HOOK_NAME" \
  "$PLUGIN_DIR/hooks/$HOOK_NAME"; do
  if [ -f "$path" ]; then
    HOOK_SCRIPT="$path"
    break
  fi
done

if [ -z "$HOOK_SCRIPT" ]; then
  echo -e "${RED}Error: Could not find $HOOK_NAME hook script${NC}"
  exit 1
fi

echo "Hook type: $HOOK_DESC"
echo "Hook script: $HOOK_SCRIPT"
echo ""

# Check if any turbo-search hooks are already configured (new format)
EXISTING_HOOK=$(jq -r '.hooks.UserPromptSubmit[]?.hooks[]? | select(.command | (contains("pre-prompt-search") or contains("rag-context-hook"))) | .command' "$SETTINGS_FILE" 2>/dev/null | head -1)
if [ -n "$EXISTING_HOOK" ]; then
  echo -e "${YELLOW}Warning: Existing turbo-search hook found${NC}"
  echo "  Current: $EXISTING_HOOK"
  echo "  Will be replaced with: $HOOK_SCRIPT"
  echo ""

  # Backup settings
  mkdir -p "$BACKUP_DIR"
  BACKUP_SETTINGS="$BACKUP_DIR/settings.json.$(date +%Y%m%d_%H%M%S).bak"
  cp "$SETTINGS_FILE" "$BACKUP_SETTINGS"
  echo -e "${GREEN}✓${NC} Backed up settings to $BACKUP_SETTINGS"
fi

# Remove existing turbo-search hooks and add the new one (new format with matchers)
UPDATED=$(jq --arg hook "$HOOK_SCRIPT" '
  # Initialize hooks object if it does not exist
  .hooks = (.hooks // {}) |
  # Initialize UserPromptSubmit array if it does not exist
  .hooks.UserPromptSubmit = (.hooks.UserPromptSubmit // []) |
  # Remove existing turbo-search hooks
  .hooks.UserPromptSubmit = [
    .hooks.UserPromptSubmit[] |
    select(.hooks | all(.command | (contains("pre-prompt-search") or contains("rag-context-hook")) | not))
  ] |
  # Add the new hook with matcher format
  .hooks.UserPromptSubmit += [{
    "matcher": {},
    "hooks": [{
      "type": "command",
      "command": $hook,
      "timeout": 10000
    }]
  }]
' "$SETTINGS_FILE")

echo "$UPDATED" > "$SETTINGS_FILE"

echo -e "${GREEN}✓${NC} Hook configured in $SETTINGS_FILE"
echo ""

if [ "$HOOK_MODE" = "rag" ]; then
  echo -e "${BLUE}RAG Mode Enabled${NC}"
  echo ""
  echo "How it works:"
  echo "  1. When you submit a prompt, the hook extracts key terms"
  echo "  2. It searches QMD for relevant content"
  echo "  3. Actual content snippets are injected into the context"
  echo "  4. Claude can answer using this context without reading files"
  echo ""
  echo "Token usage: ~500-2000 tokens per prompt for context"
  echo "Token savings: Often 80%+ by avoiding file reads"
else
  echo -e "${BLUE}Simple Mode Enabled${NC}"
  echo ""
  echo "How it works:"
  echo "  1. When you submit a prompt, the hook extracts key terms"
  echo "  2. It searches QMD for relevant files"
  echo "  3. File path suggestions are shown to Claude"
  echo "  4. Claude can choose to read the suggested files"
  echo ""
  echo "Token usage: ~50-100 tokens per prompt"
  echo "Token savings: Varies based on Claude's choices"
fi

echo ""
echo "To switch modes:"
echo "  Simple: $0"
echo "  RAG:    $0 --rag"
echo "  Remove: $0 --remove"
echo ""
echo -e "${YELLOW}Restart Claude Code to apply changes.${NC}"
