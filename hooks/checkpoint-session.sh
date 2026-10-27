#!/bin/bash
# Checkpoints never block compaction or shutdown; Python owns the short deadline.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)" || exit 0
command -v python3 >/dev/null || exit 0
python3 "$ROOT/scripts/turbo.py" checkpoint "$@" || exit 0
