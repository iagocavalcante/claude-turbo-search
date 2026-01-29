#!/bin/bash
# memory-db.sh - SQLite memory database operations
#
# Usage:
#   memory-db.sh init                     # Initialize database
#   memory-db.sh search "query"           # Search memory (FTS)
#   memory-db.sh add-session "summary" "files" "tools" "topics"
#   memory-db.sh add-knowledge "area" "summary" "patterns"
#   memory-db.sh add-fact "fact" "category"
#   memory-db.sh recent [n]               # Get n recent sessions (default 5)
#   memory-db.sh context "query" [limit]  # Get context for injection

set -e

# Find repo root and memory database
find_repo_root() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        if [ -d "$dir/.git" ]; then
            echo "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo "$PWD"  # Fallback to current directory
}

REPO_ROOT="$(find_repo_root)"
MEMORY_DIR="$REPO_ROOT/.claude-memory"
DB_FILE="$MEMORY_DIR/memory.db"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMA_FILE="$SCRIPT_DIR/schema.sql"

# Ensure memory directory exists
ensure_dir() {
    mkdir -p "$MEMORY_DIR"
}

# Initialize database with schema
cmd_init() {
    ensure_dir
    if [ ! -f "$DB_FILE" ]; then
        sqlite3 "$DB_FILE" < "$SCHEMA_FILE"
        echo "Memory database initialized at $DB_FILE"
    else
        echo "Memory database already exists at $DB_FILE"
    fi
}

# Search memory using FTS
cmd_search() {
    local query="$1"
    local limit="${2:-10}"

    if [ ! -f "$DB_FILE" ]; then
        echo "No memory database found. Run 'memory-db.sh init' first."
        exit 1
    fi

    sqlite3 -json "$DB_FILE" <<EOF
SELECT
    source_type,
    source_id,
    snippet(memory_fts, 0, '**', '**', '...', 32) as match
FROM memory_fts
WHERE memory_fts MATCH '$query'
ORDER BY rank
LIMIT $limit;
EOF
}

# Add a session summary
cmd_add_session() {
    local summary="$1"
    local files="$2"
    local tools="$3"
    local topics="$4"

    ensure_dir
    [ ! -f "$DB_FILE" ] && cmd_init

    sqlite3 "$DB_FILE" <<EOF
INSERT INTO sessions (summary, files_touched, tools_used, topics)
VALUES ('$(echo "$summary" | sed "s/'/''/g")',
        '$(echo "$files" | sed "s/'/''/g")',
        '$(echo "$tools" | sed "s/'/''/g")',
        '$(echo "$topics" | sed "s/'/''/g")');
EOF
    echo "Session saved."
}

# Add or update knowledge about a code area
cmd_add_knowledge() {
    local area="$1"
    local summary="$2"
    local patterns="$3"

    ensure_dir
    [ ! -f "$DB_FILE" ] && cmd_init

    sqlite3 "$DB_FILE" <<EOF
INSERT INTO knowledge (area, summary, patterns)
VALUES ('$(echo "$area" | sed "s/'/''/g")',
        '$(echo "$summary" | sed "s/'/''/g")',
        '$(echo "$patterns" | sed "s/'/''/g")')
ON CONFLICT(area) DO UPDATE SET
    summary = excluded.summary,
    patterns = excluded.patterns,
    updated_at = CURRENT_TIMESTAMP;
EOF
    echo "Knowledge saved for: $area"
}

# Add a fact
cmd_add_fact() {
    local fact="$1"
    local category="${2:-general}"

    ensure_dir
    [ ! -f "$DB_FILE" ] && cmd_init

    sqlite3 "$DB_FILE" <<EOF
INSERT INTO facts (fact, category)
VALUES ('$(echo "$fact" | sed "s/'/''/g")',
        '$(echo "$category" | sed "s/'/''/g")');
EOF
    echo "Fact saved."
}

# Get recent sessions
cmd_recent() {
    local limit="${1:-5}"

    if [ ! -f "$DB_FILE" ]; then
        echo "No memory database found."
        exit 0
    fi

    sqlite3 -json "$DB_FILE" <<EOF
SELECT id, created_at, summary, topics
FROM sessions
ORDER BY created_at DESC
LIMIT $limit;
EOF
}

# Get context for injection (combines memory search with recent sessions)
cmd_context() {
    local query="$1"
    local token_limit="${2:-1500}"

    if [ ! -f "$DB_FILE" ]; then
        exit 0  # Silent exit if no memory yet
    fi

    # Estimate ~4 chars per token
    local char_limit=$((token_limit * 4))
    local output=""

    # Get relevant facts first (highest value, lowest cost)
    local facts
    facts=$(sqlite3 "$DB_FILE" "SELECT fact FROM facts ORDER BY created_at DESC LIMIT 5;" 2>/dev/null || echo "")
    if [ -n "$facts" ]; then
        output+="## Project Facts\n"
        while IFS= read -r fact; do
            output+="- $fact\n"
        done <<< "$facts"
        output+="\n"
    fi

    # Get relevant knowledge areas
    if [ -n "$query" ]; then
        local knowledge
        knowledge=$(sqlite3 "$DB_FILE" <<EOF 2>/dev/null || echo ""
SELECT area, summary FROM knowledge
WHERE area LIKE '%${query}%' OR summary LIKE '%${query}%'
LIMIT 3;
EOF
)
        if [ -n "$knowledge" ]; then
            output+="## Relevant Code Areas\n"
            output+="$knowledge\n\n"
        fi
    fi

    # Get recent session summaries
    local sessions
    sessions=$(sqlite3 "$DB_FILE" "SELECT summary FROM sessions ORDER BY created_at DESC LIMIT 3;" 2>/dev/null || echo "")
    if [ -n "$sessions" ]; then
        output+="## Recent Work\n"
        while IFS= read -r session; do
            output+="- $session\n"
        done <<< "$sessions"
        output+="\n"
    fi

    # Search for query-specific context
    if [ -n "$query" ]; then
        local search_results
        search_results=$(sqlite3 "$DB_FILE" <<EOF 2>/dev/null || echo ""
SELECT snippet(memory_fts, 0, '', '', '...', 32) as match
FROM memory_fts
WHERE memory_fts MATCH '${query}'
ORDER BY rank
LIMIT 5;
EOF
)
        if [ -n "$search_results" ]; then
            output+="## Related Context\n"
            output+="$search_results\n"
        fi
    fi

    # Truncate to token limit
    echo -e "$output" | head -c "$char_limit"
}

# Show database stats
cmd_stats() {
    if [ ! -f "$DB_FILE" ]; then
        echo "No memory database found."
        exit 0
    fi

    echo "Memory Database: $DB_FILE"
    echo ""
    sqlite3 "$DB_FILE" <<EOF
SELECT 'Sessions' as type, COUNT(*) as count FROM sessions
UNION ALL
SELECT 'Knowledge areas', COUNT(*) FROM knowledge
UNION ALL
SELECT 'Facts', COUNT(*) FROM facts;
EOF
}

# Main command dispatch
case "${1:-}" in
    init)
        cmd_init
        ;;
    search)
        cmd_search "${2:-}" "${3:-10}"
        ;;
    add-session)
        cmd_add_session "$2" "$3" "$4" "$5"
        ;;
    add-knowledge)
        cmd_add_knowledge "$2" "$3" "$4"
        ;;
    add-fact)
        cmd_add_fact "$2" "$3"
        ;;
    recent)
        cmd_recent "${2:-5}"
        ;;
    context)
        cmd_context "$2" "${3:-1500}"
        ;;
    stats)
        cmd_stats
        ;;
    *)
        echo "Usage: $0 {init|search|add-session|add-knowledge|add-fact|recent|context|stats}"
        echo ""
        echo "Commands:"
        echo "  init                    Initialize memory database"
        echo "  search <query>          Search memory"
        echo "  add-session <summary> <files> <tools> <topics>"
        echo "  add-knowledge <area> <summary> <patterns>"
        echo "  add-fact <fact> [category]"
        echo "  recent [n]              Show n recent sessions"
        echo "  context <query> [limit] Get context for injection"
        echo "  stats                   Show database statistics"
        exit 1
        ;;
esac
