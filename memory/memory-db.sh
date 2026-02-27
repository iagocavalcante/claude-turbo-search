#!/bin/bash
# memory-db.sh - compatibility wrapper
#
# Go handles stable/core commands. Legacy shell keeps advanced/vector flows
# until they are migrated.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LEGACY_SCRIPT="$SCRIPT_DIR/memory-db-legacy.sh"
GO_DIR="$SCRIPT_DIR"

CORE_COMMANDS=(
  init
  init-vector
  init-metadata
  init-token-metrics
  search
  vsearch
  add-session
  add-knowledge
  add-fact
  add-token-metrics
  recent
  context
  embed
  consolidate
  entity-search
  stats
  token-stats
  knowledge-graph
)

is_core_command() {
  local cmd="$1"
  for item in "${CORE_COMMANDS[@]}"; do
    if [ "$item" = "$cmd" ]; then
      return 0
    fi
  done
  return 1
}

cmd="${1:-}"

if [ -z "$cmd" ]; then
  "$LEGACY_SCRIPT"
  exit $?
fi

if is_core_command "$cmd"; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is required for command '$cmd'. Falling back to legacy shell implementation..." >&2
    "$LEGACY_SCRIPT" "$@"
    exit $?
  fi

  mkdir -p "$REPO_ROOT/.claude-memory/.gocache" "$REPO_ROOT/.claude-memory/.gotmp"
  (
    cd "$GO_DIR"
    MEMORY_SCRIPT_DIR="$SCRIPT_DIR" \
    GOCACHE="$REPO_ROOT/.claude-memory/.gocache" \
    GOTMPDIR="$REPO_ROOT/.claude-memory/.gotmp" \
    go run ./cmd/memorydb "$@"
  )
  exit $?
fi

"$LEGACY_SCRIPT" "$@"
