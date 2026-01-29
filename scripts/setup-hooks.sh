#!/bin/bash
# setup-hooks.sh - Configure Claude Code hooks for automatic context injection
#
# Usage:
#   ./setup-hooks.sh              # Install simple hook (suggests file paths)
#   ./setup-hooks.sh --rag        # Install RAG hook (injects content + memory)
#   ./setup-hooks.sh --memory     # Add activity tracking for /remember
#   ./setup-hooks.sh --rag --memory  # Full setup with memory
#   ./setup-hooks.sh --remove     # Remove all turbo-search hooks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(dirname "$SCRIPT_DIR")"
CLAUDE_DIR="$HOME/.claude"
SETTINGS_FILE="$CLAUDE_DIR/settings.json"
BACKUP_DIR="$CLAUDE_DIR/backups"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Parse arguments
HOOK_MODE="simple"
ENABLE_MEMORY=false

for arg in "$@"; do
  case "$arg" in
    --rag)
      HOOK_MODE="rag"
      ;;
    --memory)
      ENABLE_MEMORY=true
      ;;
    --remove)
      echo "Removing turbo-search hooks..."

      if [ -f "$SETTINGS_FILE" ]; then
        # Remove our hooks from UserPromptSubmit and PostToolUse (new format with matchers)
        UPDATED=$(jq '
          # Remove from UserPromptSubmit
          if .hooks.UserPromptSubmit then
            .hooks.UserPromptSubmit = [
              .hooks.UserPromptSubmit[] |
              select(.hooks | all(.command | (contains("pre-prompt-search") or contains("rag-context-hook")) | not))
            ] |
            if (.hooks.UserPromptSubmit | length) == 0 then del(.hooks.UserPromptSubmit) else . end
          else . end |
          # Remove from PostToolUse
          if .hooks.PostToolUse then
            .hooks.PostToolUse = [
              .hooks.PostToolUse[] |
              select(.hooks | all(.command | contains("track-activity") | not))
            ] |
            if (.hooks.PostToolUse | length) == 0 then del(.hooks.PostToolUse) else . end
          else . end |
          # Remove empty hooks object
          if .hooks and (.hooks | keys | length) == 0 then del(.hooks) else . end
        ' "$SETTINGS_FILE" 2>/dev/null || cat "$SETTINGS_FILE")
        echo "$UPDATED" > "$SETTINGS_FILE"
        echo -e "${GREEN}✓${NC} Hooks removed from settings.json"
      fi

      exit 0
      ;;
    --help|-h)
      echo "Usage: $0 [--rag] [--memory] [--remove] [--help]"
      echo ""
      echo "Options:"
      echo "  (none)    Install simple hook (suggests file paths)"
      echo "  --rag     Install RAG hook (injects content + memory context)"
      echo "  --memory  Enable activity tracking for /remember command"
      echo "  --remove  Remove all turbo-search hooks"
      echo "  --help    Show this help message"
      echo ""
      echo "Examples:"
      echo "  $0 --rag           # RAG with memory context"
      echo "  $0 --rag --memory  # RAG + activity tracking for /remember"
      echo ""
      echo "The RAG hook provides better context but uses more tokens per prompt."
      echo "The --memory flag enables tracking what files you work on for /remember."
      exit 0
      ;;
  esac
done

MEMORY_LABEL=""
if [ "$ENABLE_MEMORY" = true ]; then
  MEMORY_LABEL=" + memory"
fi
echo -e "${BLUE}Setting up turbo-search hooks (mode: $HOOK_MODE$MEMORY_LABEL)...${NC}"
echo ""

# Ensure .claude directory exists
mkdir -p "$CLAUDE_DIR"

# Create settings.json if it doesn't exist
if [ ! -f "$SETTINGS_FILE" ]; then
  echo "{}" > "$SETTINGS_FILE"
fi

# Determine the main hook script path
if [ "$HOOK_MODE" = "rag" ]; then
  HOOK_NAME="rag-context-hook.sh"
  HOOK_DESC="RAG context injection (injects content + memory)"
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

# Find activity tracking hook if memory is enabled
ACTIVITY_HOOK=""
if [ "$ENABLE_MEMORY" = true ]; then
  for path in \
    "$HOME/.claude/plugins/"*"/claude-turbo-search/hooks/track-activity.sh" \
    "$HOME/claude-turbo-search/hooks/track-activity.sh" \
    "$PLUGIN_DIR/hooks/track-activity.sh"; do
    if [ -f "$path" ]; then
      ACTIVITY_HOOK="$path"
      break
    fi
  done

  if [ -z "$ACTIVITY_HOOK" ]; then
    echo -e "${YELLOW}Warning: Could not find track-activity.sh hook${NC}"
    echo "Activity tracking will not be enabled."
    ENABLE_MEMORY=false
  fi
fi

echo "Hook type: $HOOK_DESC"
echo "Hook script: $HOOK_SCRIPT"
if [ "$ENABLE_MEMORY" = true ]; then
  echo "Activity tracking: $ACTIVITY_HOOK"
fi
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

# Build jq command based on options
if [ "$ENABLE_MEMORY" = true ]; then
  # Install both UserPromptSubmit and PostToolUse hooks
  UPDATED=$(jq --arg hook "$HOOK_SCRIPT" --arg activity "$ACTIVITY_HOOK" '
    # Initialize hooks object if it does not exist
    .hooks = (.hooks // {}) |

    # Setup UserPromptSubmit
    .hooks.UserPromptSubmit = (.hooks.UserPromptSubmit // []) |
    .hooks.UserPromptSubmit = [
      .hooks.UserPromptSubmit[] |
      select(.hooks | all(.command | (contains("pre-prompt-search") or contains("rag-context-hook")) | not))
    ] |
    .hooks.UserPromptSubmit += [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": $hook,
        "timeout": 10000
      }]
    }] |

    # Setup PostToolUse for activity tracking
    .hooks.PostToolUse = (.hooks.PostToolUse // []) |
    .hooks.PostToolUse = [
      .hooks.PostToolUse[] |
      select(.hooks | all(.command | contains("track-activity") | not))
    ] |
    .hooks.PostToolUse += [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": $activity,
        "timeout": 5000
      }]
    }]
  ' "$SETTINGS_FILE")
else
  # Install only UserPromptSubmit hook
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
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": $hook,
        "timeout": 10000
      }]
    }]
  ' "$SETTINGS_FILE")
fi

echo "$UPDATED" > "$SETTINGS_FILE"

echo -e "${GREEN}✓${NC} Hook configured in $SETTINGS_FILE"
echo ""

if [ "$HOOK_MODE" = "rag" ]; then
  echo -e "${BLUE}RAG Mode Enabled${NC}"
  echo ""
  echo "How it works:"
  echo "  1. When you submit a prompt, the hook extracts key terms"
  echo "  2. It searches memory database for relevant context"
  echo "  3. It searches QMD for relevant code snippets"
  echo "  4. Context is injected to help Claude answer without reading files"
  echo ""
  echo "Token usage: ~1000-1500 tokens per prompt for context"
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

if [ "$ENABLE_MEMORY" = true ]; then
  echo ""
  echo -e "${BLUE}Activity Tracking Enabled${NC}"
  echo ""
  echo "  Files you read/write are logged for /remember"
  echo "  Use /remember at end of sessions to save context"
  echo "  Memory persists across sessions in .claude-memory/"

  # Initialize memory database for current repo
  if [ -x "$MEMORY_SCRIPT" ]; then
    echo ""
    "$MEMORY_SCRIPT" init 2>/dev/null || true
  fi
fi

echo ""
echo "To switch modes:"
echo "  Simple:      $0"
echo "  RAG:         $0 --rag"
echo "  RAG+Memory:  $0 --rag --memory"
echo "  Remove:      $0 --remove"
echo ""
echo -e "${YELLOW}Restart Claude Code to apply changes.${NC}"
