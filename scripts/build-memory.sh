#!/bin/bash
# Build once during setup; never compile in a prompt hook.
set -eu
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
command -v go >/dev/null || { echo 'Go is required to build memorydb.' >&2; exit 1; }
BUILD_FILE=$(mktemp "$ROOT/memory/.memorydb.XXXXXX")
trap 'rm -f "$BUILD_FILE"' EXIT
cd "$ROOT/memory"
go build -trimpath -o "$BUILD_FILE" ./cmd/memorydb
chmod 755 "$BUILD_FILE"
mv "$BUILD_FILE" "$ROOT/memory/memorydb"
echo "Built $ROOT/memory/memorydb"
