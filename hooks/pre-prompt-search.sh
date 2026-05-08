#!/bin/bash
# pre-prompt-search.sh - Search QMD for relevant context before prompts
# This hook runs before prompts are processed and injects relevant context
#
# Usage: Called automatically by Claude Code hooks system
# Input: User prompt via stdin or $CLAUDE_PROMPT
# Output: Relevant context to inject (stdout)

set -e

# Get the prompt from environment or stdin
# UserPromptSubmit hooks receive a JSON envelope on stdin; extract .prompt.
# Fall back to raw input if it isn't JSON (e.g. manual testing).
RAW_INPUT="${CLAUDE_PROMPT:-$(cat)}"
PROMPT=$(printf '%s' "$RAW_INPUT" | jq -r '.prompt // empty' 2>/dev/null)
[ -z "$PROMPT" ] && PROMPT="$RAW_INPUT"

# Skip if prompt is too short or looks like a command
if [ ${#PROMPT} -lt 10 ]; then
  exit 0
fi

# Skip for certain prompt patterns (commands, greetings, etc)
if echo "$PROMPT" | grep -qiE "^(hi|hello|hey|thanks|ok|yes|no|/|commit|push|pull)"; then
  exit 0
fi

# Check if qmd is available and has collections
if ! command -v qmd &> /dev/null; then
  exit 0
fi

# Check if there are any indexed collections
COLLECTIONS=$(qmd status 2>/dev/null | grep -c "Collection" || echo "0")
if [ "$COLLECTIONS" = "0" ]; then
  exit 0
fi

# Extract key terms from the prompt (simple approach: take longer words)
# Remove common stop words and keep meaningful terms
SEARCH_TERMS=$(echo "$PROMPT" | \
  tr '[:upper:]' '[:lower:]' | \
  tr -cs '[:alnum:]' ' ' | \
  tr ' ' '\n' | \
  grep -vE '^(the|a|an|is|are|was|were|be|been|being|have|has|had|do|does|did|will|would|could|should|may|might|must|can|this|that|these|those|it|its|i|you|we|they|he|she|what|how|why|when|where|which|who|whom|and|or|but|if|then|else|for|to|of|in|on|at|by|with|from|about|into|through|during|before|after|above|below|between|under|over|out|up|down|off|just|only|also|very|really|please|help|me|my|your)$' | \
  awk 'length >= 4' | \
  head -5 | \
  tr '\n' ' ')

if [ -z "$SEARCH_TERMS" ]; then
  exit 0
fi

# Search qmd for relevant content (fast BM25 search)
RESULTS=$(qmd search "$SEARCH_TERMS" --files -n 3 2>/dev/null || true)

if [ -n "$RESULTS" ]; then
  echo ""
  echo "<qmd-context>"
  echo "Relevant files found by semantic search (consider reading these first):"
  echo "$RESULTS" | while IFS=',' read -r id score path context; do
    # Extract just the file path from qmd:// URL
    FILEPATH=$(echo "$path" | sed 's|qmd://[^/]*/||')
    if [ -n "$FILEPATH" ]; then
      echo "  - $FILEPATH"
    fi
  done
  echo "</qmd-context>"
  echo ""
fi
