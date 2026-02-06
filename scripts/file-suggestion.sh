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
  # Check if collection exists by listing collections
  if qmd collection list 2>/dev/null | grep -q "$PROJECT_NAME"; then
    QMD_AVAILABLE=true
  fi
fi

# Semantic search results (if available and query is meaningful)
if [ "$QMD_AVAILABLE" = true ] && [ -n "$QUERY" ] && [ ${#QUERY} -gt 2 ]; then
  # Prepend QMD results for relevant docs using correct syntax
  # qmd search <query> -c <collection> --files outputs file paths
  qmd search "$QUERY" -c "$PROJECT_NAME" --files -n 5 2>/dev/null | cut -d',' -f3
fi

# Fast file search with rg + fzf
{
  # Main search - respects .gitignore, includes hidden files, follows symlinks
  # Explicitly exclude common dependency/build directories across languages
  rg --files --follow --hidden \
    --glob '!node_modules/' \
    --glob '!bower_components/' \
    --glob '!.npm/' \
    --glob '!.yarn/' \
    --glob '!.pnpm-store/' \
    --glob '!dist/' \
    --glob '!build/' \
    --glob '!coverage/' \
    --glob '!_build/' \
    --glob '!deps/' \
    --glob '!.elixir_ls/' \
    --glob '!.fetch/' \
    --glob '!target/' \
    --glob '!.gradle/' \
    --glob '!.mvn/' \
    --glob '!out/' \
    --glob '!.settings/' \
    --glob '!vendor/' \
    --glob '!.bundle/' \
    --glob '!.composer/' \
    --glob '!.gem/' \
    --glob '!__pycache__/' \
    --glob '!.venv/' \
    --glob '!venv/' \
    --glob '!.tox/' \
    --glob '!.mypy_cache/' \
    --glob '!.pytest_cache/' \
    --glob '!*.egg-info/' \
    --glob '!.git/' \
    . 2>/dev/null

  # Always include important docs even if gitignored
  [ -e docs/CODEBASE_MAP.md ] && echo "docs/CODEBASE_MAP.md"
  [ -e CLAUDE.md ] && echo "CLAUDE.md"
  [ -e README.md ] && echo "README.md"
} | sort -u | fzf --filter "$QUERY" 2>/dev/null | head -15
