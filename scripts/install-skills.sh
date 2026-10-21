#!/bin/bash
# Standalone skills installation for clients without plugin installation support.
set -eu
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLIENT="${1:-codex}"
case "$CLIENT" in
  codex) DEST="$HOME/.agents/skills" ;;
  claude) DEST="$HOME/.claude/skills" ;;
  *) echo "Usage: $0 [codex|claude]" >&2; exit 1 ;;
esac
for source in "$ROOT"/skills/*; do
  target="$DEST/turbo-$(basename "$source")"
  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ ! -L "$target" ] || [ "$(readlink "$target")" != "$source" ]; then
      echo "Refusing to replace existing skill: $target" >&2
      exit 1
    fi
  fi
done
mkdir -p "$DEST"
for source in "$ROOT"/skills/*; do
  target="$DEST/turbo-$(basename "$source")"
  if [ ! -L "$target" ]; then ln -s "$source" "$target"; fi
done
echo "Installed shared skills in $DEST; restart your client to discover them."
