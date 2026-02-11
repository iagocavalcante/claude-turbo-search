---
name: knowledge-graph
description: Interactive TUI knowledge graph viewer for the persistent memory database.
---

# /knowledge-graph - Knowledge Graph Viewer

Interactive TUI that visualizes the entity relationships, timelines, and statistics from the persistent memory database.

## Instructions

When the user invokes `/knowledge-graph`, run the Python TUI viewer.

### 1. Resolve Paths

```bash
PLUGIN_DIR="${PLUGIN_DIR:-$(find ~/.claude/plugins -name "claude-turbo-search" -type d 2>/dev/null | head -1)}"
[ -z "$PLUGIN_DIR" ] && PLUGIN_DIR="$HOME/claude-turbo-search"
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo "$PWD")
DB_PATH="$REPO_ROOT/.claude-memory/memory.db"
```

### 2. Run the Viewer

```bash
python3 "$PLUGIN_DIR/skills/knowledge-graph/knowledge_graph.py" --db "$DB_PATH"
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
python3 "$PLUGIN_DIR/skills/knowledge-graph/knowledge_graph.py" --db "$DB_PATH" graph
python3 "$PLUGIN_DIR/skills/knowledge-graph/knowledge_graph.py" --db "$DB_PATH" timeline
python3 "$PLUGIN_DIR/skills/knowledge-graph/knowledge_graph.py" --db "$DB_PATH" stats
python3 "$PLUGIN_DIR/skills/knowledge-graph/knowledge_graph.py" --db "$DB_PATH" explore auth
```

### 4. Present Output

The script renders Rich TUI output directly to the terminal. Present the output as-is to the user. If the user asks about specific entities or relationships, use the `explore` subcommand to drill down.

## Notes

- Requires Python 3.7+ and SQLite3 (both pre-installed on macOS/Linux)
- Rich library is auto-installed if missing; falls back to plain text
- Run `/remember` first if the database is empty
- Run `memory-db.sh init-metadata` if entity tables are missing
