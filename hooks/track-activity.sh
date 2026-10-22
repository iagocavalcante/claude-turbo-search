#!/bin/bash
# Hook failures must never block a prompt; Python owns parsing and deadlines.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)" || exit 0
command -v python3 >/dev/null || exit 0
python3 "$ROOT/scripts/turbo.py" activity "$@" || exit 0
