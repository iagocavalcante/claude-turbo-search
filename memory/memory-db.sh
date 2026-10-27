#!/bin/bash
# memory-db.sh - compatibility wrapper
#
# Go handles stable/core commands. Legacy shell keeps advanced/vector flows
# until they are migrated.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LEGACY_SCRIPT="$SCRIPT_DIR/memory-db-legacy.sh"

# Resolve from the caller before dispatching into the installed backend.
if [ -n "${MEMORY_REPO_ROOT:-}" ]; then
  TARGET_REPO_ROOT="$MEMORY_REPO_ROOT"
else
  TARGET_REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || TARGET_REPO_ROOT=""
fi

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
  supersede
  restore
  handoff
  checkpoint
  export
  query
  entity-search
  stats
  token-stats
  knowledge-graph
  config
  push
  pull
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

  if [ -z "$TARGET_REPO_ROOT" ]; then
    echo "memory-db: '$PWD' is not inside a git repository." >&2
    echo "memory-db: per-repo memory needs a repo root; run from a git repo or set MEMORY_REPO_ROOT." >&2
    exit 1
  fi

  if [ ! -x "$SCRIPT_DIR/memorydb" ]; then
    echo "Memory binary missing. Run: bash \"$PLUGIN_ROOT/scripts/build-memory.sh\"" >&2
    exit 1
  fi
  MEMORY_SCRIPT_DIR="$SCRIPT_DIR" MEMORY_REPO_ROOT="$TARGET_REPO_ROOT" \
    exec "$SCRIPT_DIR/memorydb" "$@"
fi

"$LEGACY_SCRIPT" "$@"
