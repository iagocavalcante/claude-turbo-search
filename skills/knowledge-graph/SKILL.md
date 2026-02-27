---
name: knowledge-graph
description: Interactive TUI knowledge graph viewer for the persistent memory database.
---

# /knowledge-graph - Knowledge Graph Viewer

Interactive TUI that visualizes the entity relationships, timelines, and statistics from the persistent memory database.

## Instructions

When the user invokes `/knowledge-graph`, run the Go CLI viewer.

### 1. Resolve Paths

```bash
PLUGIN_DIR="${PLUGIN_DIR:-$(find ~/.claude/plugins -name "claude-turbo-search" -type d 2>/dev/null | head -1)}"
[ -z "$PLUGIN_DIR" ] && PLUGIN_DIR="$HOME/claude-turbo-search"
```

### 2. Run the Viewer

```bash
"$PLUGIN_DIR/memory/memory-db.sh" knowledge-graph [subcommand] [entity]
```

### 3. Subcommands

Map user arguments to subcommands:

- `/knowledge-graph` — All views combined (default)
- `/knowledge-graph graph` — Entity relationship graph with tree and edges
- `/knowledge-graph timeline` — Chronological session/knowledge timeline
- `/knowledge-graph stats` — Statistics dashboard with counts and bar charts
- `/knowledge-graph explore <name>` — Drill-down into a specific entity

```bash
# Examples:
"$PLUGIN_DIR/memory/memory-db.sh" knowledge-graph
"$PLUGIN_DIR/memory/memory-db.sh" knowledge-graph stats
"$PLUGIN_DIR/memory/memory-db.sh" knowledge-graph graph
"$PLUGIN_DIR/memory/memory-db.sh" knowledge-graph timeline
"$PLUGIN_DIR/memory/memory-db.sh" knowledge-graph explore auth
```

### 4. Present Output

The command renders colored ANSI output directly to the terminal. Present the output as-is to the user. If the user asks about specific entities or relationships, use the `explore` subcommand to drill down.

## Notes

- Requires Go 1.22+ and SQLite3
- No external Go dependencies — uses ANSI escape codes for colors
- Run `/remember` first if the database is empty
- Run `memory-db.sh init-metadata` if entity tables are missing
