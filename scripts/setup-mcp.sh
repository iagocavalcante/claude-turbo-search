#!/bin/bash
# Register through client CLIs without rewriting their configuration files.
set -eu
CLIENT=claude
while [ "$#" -gt 0 ]; do
  case "$1" in
    --client) CLIENT="${2:?--client requires claude, codex, or both}"; shift 2 ;;
    --help|-h) echo "Usage: $0 [--client claude|codex|both]"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done
case "$CLIENT" in claude|codex|both) ;; *) echo 'Invalid client' >&2; exit 1 ;; esac
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
command -v qmd >/dev/null || { echo 'Install QMD before registering its MCP server.' >&2; exit 1; }
if [ "$CLIENT" = both ]; then CLIENTS=(claude codex); else CLIENTS=("$CLIENT"); fi
for client in "${CLIENTS[@]}"; do
  command -v "$client" >/dev/null || { echo "$client CLI is missing" >&2; exit 1; }
done
for client in "${CLIENTS[@]}"; do
  if "$client" mcp get qmd >/dev/null 2>&1; then
    echo "$client: qmd already registered; preserving its configuration."
  elif [ "$client" = claude ]; then
    claude mcp add --transport stdio --scope user qmd -- bash "$ROOT/scripts/qmd-mcp.sh"
  else
    codex mcp add qmd -- bash "$ROOT/scripts/qmd-mcp.sh"
  fi
done
