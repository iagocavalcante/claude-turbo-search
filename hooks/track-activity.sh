#!/bin/bash
# track-activity.sh - PostToolUse hook to track session activity
#
# This hook runs after each tool use and logs:
# - Files that were read, written, or edited
# - Tools that were used
#
# The activity log is used by /remember to generate session summaries.

set -e

# Find repo root
find_repo_root() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        if [ -d "$dir/.git" ]; then
            echo "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo "$PWD"
}

REPO_ROOT="$(find_repo_root)"
MEMORY_DIR="$REPO_ROOT/.claude-memory"
ACTIVITY_FILE="$MEMORY_DIR/activity.log"

# Ensure memory directory exists
mkdir -p "$MEMORY_DIR"

# Read hook input from stdin (JSON with tool_name, tool_input, tool_output)
INPUT=$(cat)

# Extract tool name
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null || echo "")

if [ -z "$TOOL_NAME" ]; then
    exit 0
fi

# Track timestamp
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# Extract relevant information based on tool type
case "$TOOL_NAME" in
    Read|Edit|Write)
        # Extract file path
        FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null || echo "")
        if [ -n "$FILE_PATH" ]; then
            # Make path relative to repo root if possible
            REL_PATH="${FILE_PATH#$REPO_ROOT/}"
            echo "$TIMESTAMP|$TOOL_NAME|$REL_PATH" >> "$ACTIVITY_FILE"
        fi
        ;;

    Bash)
        # Extract command (first 100 chars)
        COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null | head -c 100 || echo "")
        if [ -n "$COMMAND" ]; then
            echo "$TIMESTAMP|$TOOL_NAME|$COMMAND" >> "$ACTIVITY_FILE"
        fi
        ;;

    Glob|Grep)
        # Extract pattern
        PATTERN=$(echo "$INPUT" | jq -r '.tool_input.pattern // empty' 2>/dev/null || echo "")
        if [ -n "$PATTERN" ]; then
            echo "$TIMESTAMP|$TOOL_NAME|$PATTERN" >> "$ACTIVITY_FILE"
        fi
        ;;

    Task)
        # Extract agent type and description
        AGENT=$(echo "$INPUT" | jq -r '.tool_input.subagent_type // empty' 2>/dev/null || echo "")
        DESC=$(echo "$INPUT" | jq -r '.tool_input.description // empty' 2>/dev/null || echo "")
        if [ -n "$AGENT" ]; then
            echo "$TIMESTAMP|$TOOL_NAME|$AGENT: $DESC" >> "$ACTIVITY_FILE"
        fi
        ;;

    *)
        # Log other tools without details
        echo "$TIMESTAMP|$TOOL_NAME|" >> "$ACTIVITY_FILE"
        ;;
esac

# Keep activity log from growing too large (last 200 entries)
if [ -f "$ACTIVITY_FILE" ]; then
    LINES=$(wc -l < "$ACTIVITY_FILE")
    if [ "$LINES" -gt 200 ]; then
        tail -100 "$ACTIVITY_FILE" > "$ACTIVITY_FILE.tmp"
        mv "$ACTIVITY_FILE.tmp" "$ACTIVITY_FILE"
    fi
fi

exit 0
