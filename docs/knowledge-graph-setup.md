# Knowledge Graph TUI Viewer — Setup & Usage

## Prerequisites

- Python 3.7+
- SQLite3 (pre-installed on macOS/Linux)
- A Claude Turbo Search memory database (created by `/remember` or `/turbo-index`)

## Install from this branch

```bash
git clone -b feature/knowledge-graph-tui https://github.com/iagocavalcante/claude-turbo-search.git
cd claude-turbo-search
```

Or if you already have the repo:

```bash
git fetch origin
git checkout feature/knowledge-graph-tui
```

## Initialize the memory database

If you don't have a memory database yet, create one:

```bash
./memory/memory-db.sh init
./memory/memory-db.sh init-metadata
```

Then populate it by using `/remember` at the end of Claude Code sessions, or manually:

```bash
./memory/memory-db.sh add-session "Implemented auth system" '["src/auth.ts"]' '["edit","search"]' "auth,jwt"
./memory/memory-db.sh add-knowledge "src/auth" "JWT auth with refresh tokens" "Middleware pattern"
./memory/memory-db.sh add-fact "Uses Express.js" "architecture"
```

## Usage

### As a Claude Code skill

Once installed, invoke it in Claude Code:

```
/knowledge-graph            # All views
/knowledge-graph stats      # Statistics dashboard
/knowledge-graph graph      # Entity relationship graph
/knowledge-graph timeline   # Session/knowledge timeline
/knowledge-graph explore auth  # Drill into an entity
```

### Direct CLI usage

```bash
# Default database location
DB="$(git rev-parse --show-toplevel)/.claude-memory/memory.db"

# All views combined
python3 skills/knowledge-graph/knowledge_graph.py --db "$DB"

# Individual views
python3 skills/knowledge-graph/knowledge_graph.py --db "$DB" stats
python3 skills/knowledge-graph/knowledge_graph.py --db "$DB" graph
python3 skills/knowledge-graph/knowledge_graph.py --db "$DB" timeline
python3 skills/knowledge-graph/knowledge_graph.py --db "$DB" explore auth

# Force plain-text output (no Rich dependency)
python3 skills/knowledge-graph/knowledge_graph.py --db "$DB" --plain stats
```

## Views

| Command | What it shows |
|---------|---------------|
| `stats` | Row counts, entity categories, top entities bar chart |
| `graph` | Entity tree grouped by type, relation edges table, co-occurrence pairs |
| `timeline` | Chronological list of sessions and knowledge entries with activity sparkline |
| `explore <name>` | Drill-down: where the entity appears, its relations, and co-occurring entities |
| `full` (default) | All of the above combined |

## Rich library

The viewer auto-installs the [Rich](https://github.com/Textualize/rich) library for colored, formatted TUI output. If installation fails, it falls back to plain-text rendering with box-drawing characters. You can force plain mode with `--plain`.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Database not found" | Run `/turbo-index` or `/remember` first, or check the `--db` path |
| "Entity metadata not initialized" | Run `./memory/memory-db.sh init-metadata` |
| Empty graph / zero counts | Use `/remember` after a few sessions to populate data |
| Rich not rendering colors | Your terminal may not support ANSI colors — try `--plain` |
