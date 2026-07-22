#!/bin/bash
# memory-db.sh - compatibility wrapper
#
# Go handles stable/core commands. Legacy shell keeps advanced/vector flows
# until they are migrated.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LEGACY_SCRIPT="$SCRIPT_DIR/memory-db-legacy.sh"
GO_DIR="$SCRIPT_DIR"

# Resolve the target repo from the CALLER's working directory, before any
# chdir below. The Go path has to chdir into GO_DIR for `go run` to resolve
# the module, which destroys the cwd the binary would otherwise use to locate
# the repo; passing it explicitly keeps both backends pointed at the same
# per-repo database. Mirrors find_repo_root() in memory-db-legacy.sh.
find_repo_root() {
  local dir="$PWD"
  while [ "$dir" != "/" ]; do
    if [ -d "$dir/.git" ]; then
      echo "$dir"
      return 0
    fi
    dir="$(dirname "$dir")"
  done
  return 1
}

# An explicit MEMORY_REPO_ROOT from the environment always wins; the walk-up
# is only the fallback. The Go side validates whichever value it receives.
if [ -n "${MEMORY_REPO_ROOT:-}" ]; then
  TARGET_REPO_ROOT="$MEMORY_REPO_ROOT"
else
  TARGET_REPO_ROOT="$(find_repo_root)" || TARGET_REPO_ROOT=""
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
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is required for command '$cmd'. Falling back to legacy shell implementation..." >&2
    "$LEGACY_SCRIPT" "$@"
    exit $?
  fi

  if [ -z "$TARGET_REPO_ROOT" ]; then
    echo "memory-db: '$PWD' is not inside a git repository." >&2
    echo "memory-db: per-repo memory needs a repo root; run from a git repo or set MEMORY_REPO_ROOT." >&2
    exit 1
  fi

  mkdir -p "$PLUGIN_ROOT/.claude-memory/.gocache" "$PLUGIN_ROOT/.claude-memory/.gotmp"
  (
    cd "$GO_DIR"
    MEMORY_SCRIPT_DIR="$SCRIPT_DIR" \
    MEMORY_REPO_ROOT="$TARGET_REPO_ROOT" \
    GOCACHE="$PLUGIN_ROOT/.claude-memory/.gocache" \
    GOTMPDIR="$PLUGIN_ROOT/.claude-memory/.gotmp" \
    go run ./cmd/memorydb "$@"
  )
  exit $?
fi

"$LEGACY_SCRIPT" "$@"
