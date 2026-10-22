#!/bin/bash
set -eu
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bash "$ROOT/scripts/build-memory.sh"
python3 -m unittest discover -s "$ROOT/tests" -p 'test_*.py' -v
(cd "$ROOT/memory" && go test ./...)
(cd "$ROOT/web" && go test ./...)
