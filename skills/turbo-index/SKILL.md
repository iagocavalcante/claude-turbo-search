---
name: turbo-index
description: Set up project-scoped document search and persistent memory in Claude Code or Codex. Refresh after documentation changes.
---

# Turbo Index

Resolve `PLUGIN_DIR` to the absolute directory two levels above this skill file, resolving symlinks. Set it in each shell invocation; keep the working directory in the target repository.

1. Check dependencies with `bash "$PLUGIN_DIR/scripts/install-deps.sh" --check-only`. Install missing dependencies using the same script without `--check-only` when authorized.
2. Build the memory executable and initialize the current project's memory:

   ```bash
   bash "$PLUGIN_DIR/scripts/setup-memory.sh"
   ```

3. For a standalone skills installation, register MCP for the client being used:

   ```bash
   bash "$PLUGIN_DIR/scripts/setup-mcp.sh" --client codex
   # Use --client claude or --client both as appropriate.
   ```

   Preserve existing servers. If `qmd` is already registered, inspect it before changing it. The project adapter must run `scripts/qmd-mcp.sh`, not an old global `qmd mcp` command. See the README migration instructions.

4. Index current Markdown documentation:

   ```bash
   python3 "$PLUGIN_DIR/scripts/turbo.py" index
   ```

   This uses a dedicated QMD index/collection for this checkout, snapshots allowed Markdown files, respects ignore files and excludes hidden paths, symlinks and sensitive filenames. It does not embed by default. Use `index --embed` when semantic search is wanted; model downloads can be large. Embedding failure leaves keyword search available.

5. Verify with a real project term:

   ```bash
   python3 "$PLUGIN_DIR/scripts/turbo.py" search "authentication"
   python3 "$PLUGIN_DIR/scripts/turbo.py" doctor
   ```

Report what actually succeeded, missing dependencies and stale indexes. Do not claim measured token savings without measurements. Index refresh is explicit and never runs in a prompt hook.

Optional: install hooks with `bash "$PLUGIN_DIR/scripts/setup-hooks.sh" --client codex --rag --memory` (or `claude`). For Claude only, `setup-file-suggestion.sh` adds file suggestions. Cartographer is optional; use it only if available and useful.
