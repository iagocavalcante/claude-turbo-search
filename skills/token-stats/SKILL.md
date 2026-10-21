---
name: token-stats
description: Show token economics comparing usage with turbo-search vs without. Reports stored metrics and clearly labeled estimates.
---

## Installed paths

Resolve `PLUGIN_DIR` to the absolute directory two levels above this `SKILL.md` file.
Use the skill's actual installed location, including plugin caches or symlinks; do not infer it from the project's working directory or a home-directory checkout.
Set `PLUGIN_DIR` explicitly in each shell invocation below, keeping the working directory in the user's target repository.


# /token-stats - Token Economics Dashboard

Show the token savings achieved by using search-first exploration vs blind file reading.

## Instructions

When the user invokes `/token-stats`, analyze token usage and display savings.

### 1. Gather activity data

If the runtime exposes the current session ID, read only that session:

```bash
python3 "$PLUGIN_DIR/scripts/turbo.py" activity-read --session "$SESSION_ID"
```

Count JSON `tool` fields (`Read`, `Edit`, `Write`, `apply_patch`) only when present. Shell command contents are intentionally not logged; do not infer file reads from `Bash` events. If the session ID is unavailable, report session metrics as unavailable. Do not read other sessions or the legacy shared activity.log.

### 2. Calculate Codebase Stats

```bash
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo "$PWD")

# Common exclusions for dependencies/build artifacts across languages
EXCLUDES=(
  # Node.js/JavaScript
  --glob '!node_modules/' --glob '!bower_components/' --glob '!.npm/'
  --glob '!.yarn/' --glob '!.pnpm-store/' --glob '!dist/' --glob '!build/' --glob '!coverage/'
  # Elixir
  --glob '!_build/' --glob '!deps/' --glob '!.elixir_ls/' --glob '!.fetch/'
  # Java
  --glob '!target/' --glob '!.gradle/' --glob '!.mvn/' --glob '!out/' --glob '!.settings/'
  # Ruby
  --glob '!vendor/' --glob '!.bundle/' --glob '!.gem/'
  # PHP
  --glob '!vendor/' --glob '!.composer/'
  # Python
  --glob '!__pycache__/' --glob '!.venv/' --glob '!venv/' --glob '!.tox/'
  --glob '!.mypy_cache/' --glob '!.pytest_cache/' --glob '!*.egg-info/'
  # General
  --glob '!.git/'
)

# File types to count
FILE_TYPES="--glob *.ts --glob *.js --glob *.tsx --glob *.jsx --glob *.py --glob *.go --glob *.rs --glob *.ex --glob *.exs --glob *.java --glob *.rb --glob *.php --glob *.md"

# Total files in codebase (excluding dependencies)
TOTAL_FILES=$(rg --files "${EXCLUDES[@]}" $FILE_TYPES "$REPO_ROOT" 2>/dev/null | wc -l | tr -d ' ')

# Estimate total tokens (count lines in source files)
TOTAL_LINES=$(rg --files "${EXCLUDES[@]}" $FILE_TYPES "$REPO_ROOT" 2>/dev/null | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
ESTIMATED_TOTAL_TOKENS=$((TOTAL_LINES * 4 / 3))  # ~1.33 tokens per word, ~3 words per line

echo ""
echo "=== Codebase Size ==="
echo "Total indexable files: $TOTAL_FILES"
echo "Total lines: $TOTAL_LINES"
echo "Estimated tokens: $ESTIMATED_TOTAL_TOKENS"
```

### 3. Query Memory for Historical Data

```bash
: "${PLUGIN_DIR:?Set PLUGIN_DIR from this skill file location}"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"

if [ -f "$MEMORY_SCRIPT" ]; then
    echo ""
    echo "=== Memory Stats ==="
    "$MEMORY_SCRIPT" stats 2>/dev/null || echo "Run /turbo-index first to initialize memory"

    echo ""
    echo "=== Token Metrics (All-Time) ==="
    "$MEMORY_SCRIPT" token-stats 2>/dev/null || echo "{}"
fi
```

### 4. Calculate and Display Economics

Based on the gathered data, calculate and present:

**Token Economics Model:**

| Scenario | Calculation | Typical Cost |
|----------|-------------|--------------|
| **Blind exploration** | Read 20+ files to find relevant code | ~50,000 tokens |
| **With turbo-search** | Search (50 tokens) + Read 3-5 targeted files | ~5,000 tokens |
| **Savings** | | **~90%** |

**Present this table to the user:**

```
╔══════════════════════════════════════════════════════════════╗
║                  TOKEN ECONOMICS DASHBOARD                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Codebase: [PROJECT_NAME]                                    ║
║  Total Files: [X] | Total Lines: [Y] | Est. Tokens: [Z]      ║
║                                                               ║
╠══════════════════════════════════════════════════════════════╣
║                     THIS SESSION                              ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Searches performed:     [N]     (~50 tokens each)           ║
║  Files read (targeted):  [M]     (~1,000 tokens each)        ║
║  Files edited:           [K]                                  ║
║                                                               ║
║  ─────────────────────────────────────────────────────────── ║
║                                                               ║
║  ESTIMATED USAGE WITH PLUGIN:     ~[X] tokens                ║
║  ESTIMATED WITHOUT (blind read):  ~[Y] tokens                ║
║                                                               ║
║  ✨ SAVINGS: ~[Z]% ([Y-X] tokens saved)                      ║
║                                                               ║
╠══════════════════════════════════════════════════════════════╣
║                   HISTORICAL (ALL SESSIONS)                   ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Tracked sessions:       [N]   (from token-stats JSON)       ║
║  Total searches:         [S]                                  ║
║  Total files read:       [R]                                  ║
║  Total files edited:     [E]                                  ║
║                                                               ║
║  All-time tokens used:   ~[U] tokens                         ║
║  All-time without plugin:~[W] tokens                         ║
║  All-time tokens saved:  ~[V] tokens                         ║
║                                                               ║
║  (No data yet? Use /remember to start tracking)              ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
```

### 5. Token Calculation Logic

Use these estimates for calculations:

```
SEARCH_COST = 50              # tokens per qmd search
FILE_READ_COST = 1000         # avg tokens per file read
BLIND_EXPLORATION_FILES = 20  # files typically read without search
TARGETED_READ_FILES = 3       # files read with search-first approach

# With plugin
with_plugin = (SEARCHES * SEARCH_COST) + (FILES_READ * FILE_READ_COST)

# Without plugin (estimate)
without_plugin = BLIND_EXPLORATION_FILES * FILE_READ_COST

# Savings
savings_tokens = without_plugin - with_plugin
savings_percent = (savings_tokens / without_plugin) * 100
```

### 6. Pro Tips

End with actionable suggestions:

```
💡 Pro Tips to Maximize Savings:
   • Use project-scoped search before reading large files
   • Run /remember at session end to build context
   • Check /memory-stats for accumulated knowledge
   • The more you use it, the smarter it gets!
```

## Notes

- Estimates are based on typical Claude token encoding
- Actual savings vary based on codebase structure and task type
- Historical data requires using /remember consistently
