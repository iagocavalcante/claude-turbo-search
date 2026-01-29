#!/bin/bash
# rag-context-hook.sh - RAG-style context injection for Claude Code
# Searches memory database and QMD to inject relevant context before each prompt
#
# This hook automatically provides Claude with:
# 1. Persistent memory (session summaries, knowledge, facts)
# 2. Relevant code snippets from indexed documentation
#
# Usage: Configured as a UserPromptSubmit hook in Claude Code

set -e

# Configuration
MAX_CONTEXT_TOKENS=1500  # Approximate max tokens to inject
MAX_CODE_RESULTS=3       # Max number of code search results
MIN_QUERY_LENGTH=15      # Min prompt length to trigger search
MEMORY_TOKEN_BUDGET=500  # Tokens allocated to memory context

# Find plugin directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(dirname "$SCRIPT_DIR")"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"

# Find repo root for per-repo memory
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

# Get the prompt
PROMPT="${CLAUDE_PROMPT:-$(cat)}"

# Skip if prompt is too short
if [ ${#PROMPT} -lt $MIN_QUERY_LENGTH ]; then
  exit 0
fi

# Skip for certain patterns (commands, simple responses)
if echo "$PROMPT" | grep -qiE "^(/|yes|no|ok|thanks|hi|hello|hey|commit|push|pull|git )"; then
  exit 0
fi

# Extract meaningful search terms from the prompt
# Remove common words and keep substantive terms
extract_search_query() {
  echo "$1" | \
    tr '[:upper:]' '[:lower:]' | \
    tr -cs '[:alnum:]' ' ' | \
    tr ' ' '\n' | \
    grep -vE '^(the|a|an|is|are|was|were|be|been|being|have|has|had|do|does|did|will|would|could|should|may|might|must|can|this|that|these|those|it|its|i|you|we|they|he|she|what|how|why|when|where|which|who|whom|and|or|but|if|then|else|for|to|of|in|on|at|by|with|from|about|into|through|during|before|after|above|below|between|under|over|out|up|down|off|just|only|also|very|really|please|help|me|my|your|can|want|need|like|make|create|add|fix|update|change|show|tell|explain|find|search|look|get|write|read|use|implement|build)$' | \
    awk 'length >= 3' | \
    head -8 | \
    tr '\n' ' '
}

SEARCH_QUERY=$(extract_search_query "$PROMPT")

if [ -z "$SEARCH_QUERY" ]; then
  exit 0
fi

# ============================================
# PHASE 1: Query persistent memory
# ============================================
MEMORY_CONTEXT=""
if [ -x "$MEMORY_SCRIPT" ]; then
  MEMORY_CONTEXT=$("$MEMORY_SCRIPT" context "$SEARCH_QUERY" $MEMORY_TOKEN_BUDGET 2>/dev/null || echo "")
fi

# ============================================
# PHASE 2: Perform QMD search (fast BM25)
# ============================================
CONTEXT=""

# Check if qmd is available and has indexed collections
QMD_AVAILABLE=false
if command -v qmd &> /dev/null; then
  if qmd status 2>/dev/null | grep -q "Collection"; then
    QMD_AVAILABLE=true
  fi
fi

if [ "$QMD_AVAILABLE" = true ]; then
  # Get results with snippets in a parseable format
  SEARCH_RESULTS=$(qmd search "$SEARCH_QUERY" -n $MAX_CODE_RESULTS --json 2>/dev/null || true)

  if [ -n "$SEARCH_RESULTS" ] && [ "$SEARCH_RESULTS" != "[]" ]; then
    RESULT_COUNT=0

    # Process JSON results using jq
    while IFS= read -r result; do
      if [ -z "$result" ]; then
        continue
      fi

      FILE_PATH=$(echo "$result" | jq -r '.path // .file // .docid // empty' 2>/dev/null)
      SNIPPET=$(echo "$result" | jq -r '.snippet // .content // .text // empty' 2>/dev/null)
      SCORE=$(echo "$result" | jq -r '.score // "N/A"' 2>/dev/null)

      if [ -n "$FILE_PATH" ] && [ -n "$SNIPPET" ]; then
        # Clean up the path (remove qmd:// prefix if present)
        CLEAN_PATH=$(echo "$FILE_PATH" | sed 's|^qmd://[^/]*/||')

        CONTEXT="$CONTEXT
### $CLEAN_PATH (relevance: $SCORE)
\`\`\`
$SNIPPET
\`\`\`
"
        RESULT_COUNT=$((RESULT_COUNT + 1))
      fi
    done < <(echo "$SEARCH_RESULTS" | jq -c '.[]' 2>/dev/null)

    # If no context was built, try a simpler approach
    if [ -z "$CONTEXT" ] || [ $RESULT_COUNT -eq 0 ]; then
      # Fallback: Get file paths and fetch snippets directly
      FILE_PATHS=$(qmd search "$SEARCH_QUERY" --files -n $MAX_CODE_RESULTS 2>/dev/null | head -$MAX_CODE_RESULTS)

      if [ -n "$FILE_PATHS" ]; then
        while IFS=',' read -r id score path rest; do
          if [ -n "$path" ]; then
            CLEAN_PATH=$(echo "$path" | sed 's|^qmd://[^/]*/||')
            # Get a snippet from the file
            SNIPPET=$(qmd get "$path" -l 30 2>/dev/null | head -30 || true)
            if [ -n "$SNIPPET" ]; then
              CONTEXT="$CONTEXT
### $CLEAN_PATH
\`\`\`
$SNIPPET
\`\`\`
"
            fi
          fi
        done <<< "$FILE_PATHS"
      fi
    fi
  fi
fi

# ============================================
# PHASE 3: Combine and output context
# ============================================

# Check if we have any context to output
if [ -n "$MEMORY_CONTEXT" ] || [ -n "$CONTEXT" ]; then
  echo ""
  echo "<relevant-context source=\"turbo-search-rag\">"
  echo "The following context was automatically retrieved based on your prompt."
  echo "Use this to answer without reading additional files unless more detail is needed."
  echo ""
  echo "**Search terms:** $SEARCH_QUERY"
  echo ""

  # Output memory context first (higher priority)
  if [ -n "$MEMORY_CONTEXT" ]; then
    echo "---"
    echo "# Memory Context"
    echo ""
    echo "$MEMORY_CONTEXT"
  fi

  # Output code context
  if [ -n "$CONTEXT" ]; then
    echo "---"
    echo "# Code Context"
    echo "$CONTEXT"
  fi

  echo "</relevant-context>"
  echo ""
fi
