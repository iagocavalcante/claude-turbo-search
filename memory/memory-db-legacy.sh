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
METADATA_SCHEMA_FILE="$SCRIPT_DIR/schema-metadata.sql"

# Ensure memory directory exists
ensure_dir() {
    mkdir -p "$MEMORY_DIR"
}

# Compress memory text: normalize temporal refs, strip filler, collapse whitespace
compress_memory() {
    local text="$1"
    [ -z "$text" ] && return

    # Compute dates (macOS/Linux compatible)
    local today yesterday
    today=$(date +%Y-%m-%d)
    if date --version >/dev/null 2>&1; then
        yesterday=$(date -d "yesterday" +%Y-%m-%d)
    else
        yesterday=$(date -v-1d +%Y-%m-%d)
    fi

    # All compression via single awk pass: temporal normalization + filler stripping
    text=$(echo "$text" | awk -v today="$today" -v yesterday="$yesterday" '{
        n = split($0, words, " ")
        result = ""
        skip_next = 0
        for (i = 1; i <= n; i++) {
            if (skip_next) { skip_next = 0; continue }
            w = words[i]
            lw = tolower(w)

            # Temporal normalization
            if (lw == "today" || lw == "today," || lw == "today.") {
                suffix = substr(lw, 6)
                w = today suffix
            } else if (lw == "yesterday" || lw == "yesterday," || lw == "yesterday.") {
                suffix = substr(lw, 10)
                w = yesterday suffix
            }

            # Two-word filler detection
            if (i < n) {
                pair = lw " " tolower(words[i+1])
                if (pair == "i think" || pair == "i believe" || pair == "sort of" || pair == "kind of" || pair == "pretty much" || pair == "you know") {
                    skip_next = 1
                    continue
                }
            }

            # Single-word filler removal
            if (lw == "basically" || lw == "actually" || lw == "just" || lw == "really" || lw == "very") {
                continue
            }

            result = (result == "") ? w : result " " w
        }
        print result
    }')

    # Collapse multiple spaces/newlines into single space
    text=$(echo "$text" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g' | sed 's/^ *//;s/ *$//')

    # Deduplicate identical sentences
    text=$(echo "$text" | awk -F'[.!?]' '{
        for(i=1; i<=NF; i++) {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $i)
            if ($i != "" && !seen[$i]++) {
                printf "%s. ", $i
            }
        }
    }' | sed 's/\. $//')

    echo "$text"
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

# Initialize metadata schema (entity index + relations)
cmd_init_metadata() {
    ensure_dir
    [ ! -f "$DB_FILE" ] && cmd_init

    if [ -f "$METADATA_SCHEMA_FILE" ]; then
        sqlite3 "$DB_FILE" < "$METADATA_SCHEMA_FILE"
    else
        # Inline fallback if schema file is missing
        sqlite3 "$DB_FILE" <<'EOF'
CREATE TABLE IF NOT EXISTS entity_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity, entity_type, source_type, source_id)
);
CREATE TABLE IF NOT EXISTS entry_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_type TEXT NOT NULL,
    from_id INTEGER NOT NULL,
    to_type TEXT NOT NULL,
    to_id INTEGER NOT NULL,
    relation TEXT NOT NULL DEFAULT 'related_to',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(from_type, from_id, to_type, to_id, relation)
);
CREATE INDEX IF NOT EXISTS idx_entity_name ON entity_metadata(entity);
CREATE INDEX IF NOT EXISTS idx_entity_type ON entity_metadata(entity_type);
CREATE INDEX IF NOT EXISTS idx_entity_source ON entity_metadata(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_relation_from ON entry_relations(from_type, from_id);
CREATE INDEX IF NOT EXISTS idx_relation_to ON entry_relations(to_type, to_id);
EOF
    fi
    echo "Metadata schema initialized."
}

# Extract entities from text and files_touched, insert into entity_metadata
extract_entities() {
    local source_type="$1"
    local source_id="$2"
    local text="$3"
    local files_json="$4"

    # Ensure metadata tables exist
    local has_meta
    has_meta=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sqlite_master WHERE name = 'entity_metadata';" 2>/dev/null || echo "0")
    [ "$has_meta" -eq 0 ] && return

    local sql=""

    # Parse files_touched JSON array → file entities
    if [ -n "$files_json" ] && [ "$files_json" != "[]" ]; then
        local files_list
        files_list=$(echo "$files_json" | tr -d '[]"' | tr ',' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
        while IFS= read -r file; do
            [ -z "$file" ] && continue
            local escaped_file
            escaped_file=$(echo "$file" | sed "s/'/''/g")
            sql+="INSERT OR IGNORE INTO entity_metadata (entity, entity_type, source_type, source_id) VALUES ('$escaped_file', 'file', '$source_type', $source_id);"
        done <<< "$files_list"
    fi

    # Regex extract file-like paths from text (e.g., src/foo/bar.ts)
    local text_files
    text_files=$(echo "$text" | grep -oE '[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,6}' | grep '/' | sort -u)
    while IFS= read -r file; do
        [ -z "$file" ] && continue
        local escaped_file
        escaped_file=$(echo "$file" | sed "s/'/''/g")
        sql+="INSERT OR IGNORE INTO entity_metadata (entity, entity_type, source_type, source_id) VALUES ('$escaped_file', 'file', '$source_type', $source_id);"
    done <<< "$text_files"

    # Extract PascalCase names → concept entities
    local concepts
    concepts=$(echo "$text" | grep -oE '\b[A-Z][a-z]+([A-Z][a-z]+)+\b' | sort -u)
    while IFS= read -r concept; do
        [ -z "$concept" ] && continue
        local escaped_concept
        escaped_concept=$(echo "$concept" | sed "s/'/''/g")
        sql+="INSERT OR IGNORE INTO entity_metadata (entity, entity_type, source_type, source_id) VALUES ('$escaped_concept', 'concept', '$source_type', $source_id);"
    done <<< "$concepts"

    # Extract dash-separated lowercase names → package entities (e.g., express-session)
    local packages
    packages=$(echo "$text" | grep -oE '\b[a-z][a-z0-9]+-[a-z][a-z0-9-]+\b' | sort -u)
    while IFS= read -r pkg; do
        [ -z "$pkg" ] && continue
        local escaped_pkg
        escaped_pkg=$(echo "$pkg" | sed "s/'/''/g")
        sql+="INSERT OR IGNORE INTO entity_metadata (entity, entity_type, source_type, source_id) VALUES ('$escaped_pkg', 'package', '$source_type', $source_id);"
    done <<< "$packages"

    # Execute all inserts
    if [ -n "$sql" ]; then
        sqlite3 "$DB_FILE" "$sql" 2>/dev/null || true
    fi
}

# Search by entity with optional type filter
cmd_entity_search() {
    local query="$1"
    local entity_type="${2:-}"

    if [ ! -f "$DB_FILE" ]; then
        echo "No memory database found."
        exit 1
    fi

    local has_meta
    has_meta=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sqlite_master WHERE name = 'entity_metadata';" 2>/dev/null || echo "0")
    if [ "$has_meta" -eq 0 ]; then
        echo "Metadata not initialized. Run 'memory-db.sh init-metadata' first."
        return
    fi

    local type_filter=""
    if [ -n "$entity_type" ]; then
        type_filter="AND em.entity_type = '$(echo "$entity_type" | sed "s/'/''/g")'"
    fi

    sqlite3 "$DB_FILE" <<EOF
SELECT em.entity, em.entity_type, em.source_type, em.source_id,
    CASE em.source_type
        WHEN 'session' THEN (SELECT summary FROM sessions WHERE id = em.source_id)
        WHEN 'knowledge' THEN (SELECT area || ': ' || summary FROM knowledge WHERE id = em.source_id)
        WHEN 'fact' THEN (SELECT fact FROM facts WHERE id = em.source_id)
    END as context
FROM entity_metadata em
WHERE em.entity LIKE '%$(echo "$query" | sed "s/'/''/g")%'
$type_filter
ORDER BY em.created_at DESC
LIMIT 10;
EOF
}

# Consolidate overlapping/duplicate memory entries
cmd_consolidate() {
    if [ ! -f "$DB_FILE" ]; then
        echo "No memory database found."
        exit 1
    fi

    local merged=0 removed=0

    # --- Session consolidation: merge sessions with >50% topic overlap ---
    local session_ids
    session_ids=$(sqlite3 "$DB_FILE" "SELECT id FROM sessions ORDER BY created_at DESC;")

    local ids_array=()
    while IFS= read -r id; do
        [ -n "$id" ] && ids_array+=("$id")
    done <<< "$session_ids"

    local to_delete=()
    for ((i=0; i<${#ids_array[@]}; i++)); do
        local id_a="${ids_array[$i]}"
        # Skip if already marked for deletion
        [[ " ${to_delete[*]} " == *" $id_a "* ]] && continue

        local topics_a
        topics_a=$(sqlite3 "$DB_FILE" "SELECT topics FROM sessions WHERE id=$id_a;" | tr ',' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sort)
        local count_a
        count_a=$(echo "$topics_a" | grep -c . || echo "0")
        [ "$count_a" -eq 0 ] && continue

        for ((j=i+1; j<${#ids_array[@]}; j++)); do
            local id_b="${ids_array[$j]}"
            [[ " ${to_delete[*]} " == *" $id_b "* ]] && continue

            local topics_b
            topics_b=$(sqlite3 "$DB_FILE" "SELECT topics FROM sessions WHERE id=$id_b;" | tr ',' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sort)
            local count_b
            count_b=$(echo "$topics_b" | grep -c . || echo "0")
            [ "$count_b" -eq 0 ] && continue

            # Count overlapping topics
            local overlap
            overlap=$(comm -12 <(echo "$topics_a") <(echo "$topics_b") | grep -c . || echo "0")
            local min_count=$(( count_a < count_b ? count_a : count_b ))

            # If >50% overlap, merge into the newer one (id_a) and delete older (id_b)
            if [ "$min_count" -gt 0 ] && [ $((overlap * 100 / min_count)) -gt 50 ]; then
                local summary_a summary_b
                summary_a=$(sqlite3 "$DB_FILE" "SELECT summary FROM sessions WHERE id=$id_a;")
                summary_b=$(sqlite3 "$DB_FILE" "SELECT summary FROM sessions WHERE id=$id_b;")

                local merged_summary
                merged_summary=$(compress_memory "$summary_a $summary_b")
                local escaped_summary
                escaped_summary=$(echo "$merged_summary" | sed "s/'/''/g")

                sqlite3 "$DB_FILE" "UPDATE sessions SET summary='$escaped_summary' WHERE id=$id_a;"
                to_delete+=("$id_b")
                merged=$((merged + 1))
            fi
        done
    done

    # Delete merged sessions
    for del_id in "${to_delete[@]}"; do
        sqlite3 "$DB_FILE" "DELETE FROM sessions WHERE id=$del_id;"
        # Clean up entity metadata
        sqlite3 "$DB_FILE" "DELETE FROM entity_metadata WHERE source_type='session' AND source_id=$del_id;" 2>/dev/null || true
        removed=$((removed + 1))
    done

    # --- Fact consolidation: remove exact duplicates and substring overlaps ---
    local fact_dupes
    fact_dupes=$(sqlite3 "$DB_FILE" <<'EOF'
SELECT f1.id, f2.id FROM facts f1
JOIN facts f2 ON f1.id < f2.id AND f1.category = f2.category
WHERE f1.fact = f2.fact OR INSTR(f1.fact, f2.fact) > 0 OR INSTR(f2.fact, f1.fact) > 0;
EOF
    )

    local fact_del_ids=()
    while IFS='|' read -r id1 id2; do
        [ -z "$id1" ] && continue
        # Keep the longer/more detailed fact
        local len1 len2
        len1=$(sqlite3 "$DB_FILE" "SELECT LENGTH(fact) FROM facts WHERE id=$id1;")
        len2=$(sqlite3 "$DB_FILE" "SELECT LENGTH(fact) FROM facts WHERE id=$id2;")
        if [ "$len1" -ge "$len2" ]; then
            fact_del_ids+=("$id2")
        else
            fact_del_ids+=("$id1")
        fi
    done <<< "$fact_dupes"

    # Deduplicate the deletion list and delete
    local unique_del
    unique_del=$(printf '%s\n' "${fact_del_ids[@]}" | sort -u)
    while IFS= read -r del_id; do
        [ -z "$del_id" ] && continue
        sqlite3 "$DB_FILE" "DELETE FROM facts WHERE id=$del_id;"
        sqlite3 "$DB_FILE" "DELETE FROM entity_metadata WHERE source_type='fact' AND source_id=$del_id;" 2>/dev/null || true
        removed=$((removed + 1))
    done <<< "$unique_del"

    echo "Consolidation complete: $merged merged, $removed removed."
}

# Lightweight check: run consolidation in background if needed
maybe_consolidate() {
    [ ! -f "$DB_FILE" ] && return

    local recent_count
    recent_count=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sessions WHERE created_at > datetime('now', '-30 days');" 2>/dev/null || echo "0")

    if [ "$recent_count" -ge 10 ]; then
        ( cmd_consolidate >/dev/null 2>&1 ) & disown
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
    local summary
    summary=$(compress_memory "$1")
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

    # Extract entities from the new session
    local last_id
    last_id=$(sqlite3 "$DB_FILE" "SELECT MAX(id) FROM sessions;")
    extract_entities "session" "$last_id" "$summary" "$files"

    # Check if consolidation is needed
    maybe_consolidate

    echo "Session saved."
}

# Add or update knowledge about a code area
cmd_add_knowledge() {
    local area="$1"
    local summary
    summary=$(compress_memory "$2")
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

    # Extract entities from the knowledge entry
    local last_id
    last_id=$(sqlite3 "$DB_FILE" "SELECT id FROM knowledge WHERE area = '$(echo "$area" | sed "s/'/''/g")';")
    extract_entities "knowledge" "$last_id" "$summary" ""

    echo "Knowledge saved for: $area"
}

# Add a fact
cmd_add_fact() {
    local fact
    fact=$(compress_memory "$1")
    local category="${2:-general}"

    ensure_dir
    [ ! -f "$DB_FILE" ] && cmd_init

    sqlite3 "$DB_FILE" <<EOF
INSERT INTO facts (fact, category)
VALUES ('$(echo "$fact" | sed "s/'/''/g")',
        '$(echo "$category" | sed "s/'/''/g")');
EOF

    # Extract entities from the fact
    local last_id
    last_id=$(sqlite3 "$DB_FILE" "SELECT MAX(id) FROM facts;")
    extract_entities "fact" "$last_id" "$fact" ""

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
    init-metadata)
        cmd_init_metadata
        ;;
    consolidate)
        cmd_consolidate
        ;;
    entity-search)
        cmd_entity_search "${2:-}" "${3:-}"
        ;;
    *)
        echo "Usage: $0 {init|init-vector|init-metadata|search|vsearch|add-session|add-knowledge|add-fact|recent|context|embed|consolidate|entity-search|stats}"
        echo ""
        echo "Commands:"
        echo "  init                    Initialize memory database"
        echo "  init-vector             Enable vector search support"
        echo "  init-metadata           Initialize entity metadata schema"
        echo "  search <query>          Search memory (FTS keyword search)"
        echo "  vsearch <query>         Semantic vector search"
        echo "  add-session <summary> <files> <tools> <topics>"
        echo "  add-knowledge <area> <summary> <patterns>"
        echo "  add-fact <fact> [category]"
        echo "  recent [n]              Show n recent sessions"
        echo "  context <query> [limit] Get context for injection"
        echo "  embed                   Process embedding queue"
        echo "  consolidate             Merge overlapping sessions and deduplicate facts"
        echo "  entity-search <query> [type]  Search by entity name (type: file|package|concept)"
        echo "  stats                   Show database statistics"
        exit 1
        ;;
esac
