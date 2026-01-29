#!/bin/bash
# file-suggestion.sh - Turbo file suggestion for Claude Code
# Combines rg + fzf + QMD awareness for fast, semantic file suggestions

# Parse JSON input to get query
QUERY=$(jq -r '.query // ""')

# Use project dir from env, fallback to pwd
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"
cd "$PROJECT_DIR" || exit 1

# Get project name for QMD collection lookup
PROJECT_NAME=$(basename "$PROJECT_DIR")

# Check if QMD collection exists for semantic boost
QMD_AVAILABLE=false
if command -v qmd &> /dev/null; then
  # Check if collection exists by trying to list it
  if qmd collections 2>/dev/null | grep -q "^$PROJECT_NAME$"; then
    QMD_AVAILABLE=true
  fi
fi

# Semantic search results (if available and query is meaningful)
if [ "$QMD_AVAILABLE" = true ] && [ -n "$QUERY" ] && [ ${#QUERY} -gt 2 ]; then
  # Prepend QMD results for relevant docs
  qmd search "$PROJECT_NAME" "$QUERY" --format=paths --limit=5 2>/dev/null
fi

# Fast file search with rg + fzf
{
  # Main search - respects .gitignore, includes hidden files, follows symlinks
  rg --files --follow --hidden . 2>/dev/null

  # Always include important docs even if gitignored
  [ -e docs/CODEBASE_MAP.md ] && echo "docs/CODEBASE_MAP.md"
  [ -e CLAUDE.md ] && echo "CLAUDE.md"
  [ -e README.md ] && echo "README.md"
} | sort -u | fzf --filter "$QUERY" 2>/dev/null | head -15
