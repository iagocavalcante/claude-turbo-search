#!/bin/bash
# memory-db.sh - SQLite memory database operations
#
# Usage:
#   memory-db.sh init                     # Initialize database
#   memory-db.sh init-vector              # Add vector search support
#   memory-db.sh search "query"           # Search memory (FTS)
#   memory-db.sh vsearch "query"          # Semantic vector search
#   memory-db.sh add-session "summary" "files" "tools" "topics"
#   memory-db.sh add-knowledge "area" "summary" "patterns"
#   memory-db.sh add-fact "fact" "category"
#   memory-db.sh recent [n]               # Get n recent sessions (default 5)
#   memory-db.sh context "query" [limit]  # Get context for injection
#   memory-db.sh embed                    # Process embedding queue

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
VECTOR_SCHEMA_FILE="$SCRIPT_DIR/schema-vector.sql"
EMBEDDINGS_SCRIPT="$SCRIPT_DIR/embeddings.sh"
VECTOR_EXT=""  # Path to sqlite-vector extension, set by init-vector

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

    # Check for vector support
    local has_vector
    has_vector=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sqlite_master WHERE name = 'vector_meta';" 2>/dev/null || echo "0")
    if [ "$has_vector" -gt 0 ]; then
        echo ""
        echo "Vector Search: Enabled"
        sqlite3 "$DB_FILE" <<EOF
SELECT 'Embedded sessions' as type, COUNT(*) as count FROM sessions WHERE embedding IS NOT NULL
UNION ALL
SELECT 'Embedded knowledge', COUNT(*) FROM knowledge WHERE embedding IS NOT NULL
UNION ALL
SELECT 'Embedded facts', COUNT(*) FROM facts WHERE embedding IS NOT NULL
UNION ALL
SELECT 'Pending embeddings', COUNT(*) FROM embedding_queue WHERE status = 'pending';
EOF
    fi
}

# Initialize vector search support
cmd_init_vector() {
    ensure_dir

    # First ensure base database exists
    if [ ! -f "$DB_FILE" ]; then
        cmd_init
    fi

    # Check if vector schema already applied
    local has_vector
    has_vector=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sqlite_master WHERE name = 'vector_meta';" 2>/dev/null || echo "0")

    if [ "$has_vector" -gt 0 ]; then
        echo "Vector search already initialized."
        return 0
    fi

    # Apply vector schema (without loading extension - we handle that separately)
    # First, filter out the extension loading line from schema
    grep -v "load_extension" "$VECTOR_SCHEMA_FILE" | sqlite3 "$DB_FILE" 2>/dev/null || {
        echo "Error applying vector schema. Running migrations manually..."

        # Manual migration for existing databases
        sqlite3 "$DB_FILE" "ALTER TABLE sessions ADD COLUMN embedding BLOB;" 2>/dev/null || true
        sqlite3 "$DB_FILE" "ALTER TABLE knowledge ADD COLUMN embedding BLOB;" 2>/dev/null || true
        sqlite3 "$DB_FILE" "ALTER TABLE facts ADD COLUMN embedding BLOB;" 2>/dev/null || true

        # Create metadata table
        sqlite3 "$DB_FILE" <<EOF
CREATE TABLE IF NOT EXISTS vector_meta (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT OR REPLACE INTO vector_meta (key, value) VALUES
    ('provider', 'ollama'),
    ('model', 'bge-small-en'),
    ('dimension', '384'),
    ('version', '1');

CREATE TABLE IF NOT EXISTS embedding_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP,
    UNIQUE(source_type, source_id)
);
CREATE INDEX IF NOT EXISTS idx_embed_queue_status ON embedding_queue(status, created_at);
EOF
    }

    echo "Vector search initialized."
    echo ""
    echo "Next steps:"
    echo "  1. Run embedding setup: $EMBEDDINGS_SCRIPT setup"
    echo "  2. Process existing data: $0 embed"
}

# Semantic vector search
cmd_vsearch() {
    local query="$1"
    local limit="${2:-5}"

    if [ ! -f "$DB_FILE" ]; then
        echo "No memory database found. Run 'memory-db.sh init' first."
        exit 1
    fi

    # Check if vector search is available
    local has_vector
    has_vector=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sqlite_master WHERE name = 'vector_meta';" 2>/dev/null || echo "0")

    if [ "$has_vector" -eq 0 ]; then
        echo "Vector search not initialized. Run 'memory-db.sh init-vector' first."
        echo "Falling back to FTS search..."
        cmd_search "$query" "$limit"
        return
    fi

    # Generate embedding for query
    if [ ! -x "$EMBEDDINGS_SCRIPT" ]; then
        echo "Embeddings script not found. Falling back to FTS search..."
        cmd_search "$query" "$limit"
        return
    fi

    local query_embedding
    query_embedding=$("$EMBEDDINGS_SCRIPT" generate "$query" 2>/dev/null)

    if [ -z "$query_embedding" ] || echo "$query_embedding" | grep -q "^ERROR"; then
        echo "Failed to generate query embedding. Falling back to FTS search..."
        cmd_search "$query" "$limit"
        return
    fi

    # For now, use a simple cosine similarity calculation in Python
    # In future, this would use sqlite-vector extension
    python3 - "$DB_FILE" "$query_embedding" "$limit" <<'PYTHON'
import sys
import sqlite3
import json
import struct

db_path = sys.argv[1]
query_embedding = json.loads(sys.argv[2])
limit = int(sys.argv[3])

def blob_to_floats(blob):
    """Convert binary blob to list of floats"""
    if blob is None:
        return None
    floats = []
    for i in range(0, len(blob), 4):
        floats.append(struct.unpack('<f', blob[i:i+4])[0])
    return floats

def cosine_similarity(a, b):
    """Calculate cosine similarity between two vectors"""
    if a is None or b is None or len(a) != len(b):
        return 0.0
    dot_product = sum(x * y for x, y in zip(a, b))
    norm_a = sum(x * x for x in a) ** 0.5
    norm_b = sum(x * x for x in b) ** 0.5
    if norm_a == 0 or norm_b == 0:
        return 0.0
    return dot_product / (norm_a * norm_b)

conn = sqlite3.connect(db_path)
results = []

# Search sessions
for row in conn.execute("SELECT id, summary, embedding FROM sessions WHERE embedding IS NOT NULL"):
    embedding = blob_to_floats(row[2])
    sim = cosine_similarity(query_embedding, embedding)
    results.append(('session', row[0], row[1], sim))

# Search knowledge
for row in conn.execute("SELECT id, area, summary, embedding FROM knowledge WHERE embedding IS NOT NULL"):
    embedding = blob_to_floats(row[3])
    sim = cosine_similarity(query_embedding, embedding)
    results.append(('knowledge', row[0], f"{row[1]}: {row[2]}", sim))

# Search facts
for row in conn.execute("SELECT id, fact, embedding FROM facts WHERE embedding IS NOT NULL"):
    embedding = blob_to_floats(row[2])
    sim = cosine_similarity(query_embedding, embedding)
    results.append(('fact', row[0], row[1], sim))

conn.close()

# Sort by similarity and print top results
results.sort(key=lambda x: x[3], reverse=True)
for source_type, source_id, content, similarity in results[:limit]:
    if similarity > 0.3:  # Minimum similarity threshold
        print(f"[{source_type}:{source_id}] (sim: {similarity:.3f}) {content[:100]}")
PYTHON
}

# Process embedding queue
cmd_embed() {
    if [ ! -x "$EMBEDDINGS_SCRIPT" ]; then
        echo "Embeddings script not found at: $EMBEDDINGS_SCRIPT"
        exit 1
    fi

    # Check if setup is complete
    if [ ! -f "$MEMORY_DIR/embedding-config.json" ]; then
        echo "Embeddings not configured. Running setup..."
        "$EMBEDDINGS_SCRIPT" setup
    fi

    # Queue existing items without embeddings
    if [ -f "$DB_FILE" ]; then
        echo "Queueing items without embeddings..."
        sqlite3 "$DB_FILE" <<EOF
INSERT OR IGNORE INTO embedding_queue (source_type, source_id, content, status)
SELECT 'session', id, summary || ' ' || COALESCE(topics, ''), 'pending'
FROM sessions WHERE embedding IS NULL;

INSERT OR IGNORE INTO embedding_queue (source_type, source_id, content, status)
SELECT 'knowledge', id, area || ' ' || summary || ' ' || COALESCE(patterns, ''), 'pending'
FROM knowledge WHERE embedding IS NULL;

INSERT OR IGNORE INTO embedding_queue (source_type, source_id, content, status)
SELECT 'fact', id, fact || ' ' || COALESCE(category, ''), 'pending'
FROM facts WHERE embedding IS NULL;
EOF
    fi

    # Run batch processing
    "$EMBEDDINGS_SCRIPT" batch
}

# Main command dispatch
case "${1:-}" in
    init)
        cmd_init
        ;;
    init-vector)
        cmd_init_vector
        ;;
    search)
        cmd_search "${2:-}" "${3:-10}"
        ;;
    vsearch)
        cmd_vsearch "${2:-}" "${3:-5}"
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
    embed)
        cmd_embed
        ;;
    stats)
        cmd_stats
        ;;
    *)
        echo "Usage: $0 {init|init-vector|search|vsearch|add-session|add-knowledge|add-fact|recent|context|embed|stats}"
        echo ""
        echo "Commands:"
        echo "  init                    Initialize memory database"
        echo "  init-vector             Enable vector search support"
        echo "  search <query>          Search memory (FTS keyword search)"
        echo "  vsearch <query>         Semantic vector search"
        echo "  add-session <summary> <files> <tools> <topics>"
        echo "  add-knowledge <area> <summary> <patterns>"
        echo "  add-fact <fact> [category]"
        echo "  recent [n]              Show n recent sessions"
        echo "  context <query> [limit] Get context for injection"
        echo "  embed                   Process embedding queue"
        echo "  stats                   Show database statistics"
        exit 1
        ;;
esac
