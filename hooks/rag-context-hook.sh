#!/bin/bash
# rag-context-hook.sh - RAG-style context injection for Claude Code
# Searches memory database and QMD to inject relevant context before each prompt
#
# Features:
# 1. Intent-aware retrieval (adjusts strategy per prompt type)
# 2. Adaptive token budgeting (scales with prompt complexity)
# 3. Parallel multi-view retrieval (memory + QMD concurrently)
# 4. Deduplication across retrieval sources
# 5. Entity-based structured search
#
# Usage: Configured as a UserPromptSubmit hook in Claude Code

set -e

# Default configuration (overridden by intent routing)
MAX_CONTEXT_TOKENS=1500
MAX_CODE_RESULTS=3
MIN_QUERY_LENGTH=15
MEMORY_TOKEN_BUDGET=500
USE_VECTOR_SEARCH=true
PREFER_ENTITY_SEARCH=false

# Find plugin directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(dirname "$SCRIPT_DIR")"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"
EMBEDDINGS_SCRIPT="$PLUGIN_DIR/memory/embeddings.sh"

# Find repo root for per-repo memory
find_repo_root() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        if [ -d "$dir/.git" ]; then
            echo "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo "$PWD"
}

REPO_ROOT="$(find_repo_root)"

# Get the prompt
PROMPT="${CLAUDE_PROMPT:-$(cat)}"

# Skip if prompt is too short
if [ ${#PROMPT} -lt $MIN_QUERY_LENGTH ]; then
  exit 0
fi

# Skip for certain patterns (commands, simple responses)
if echo "$PROMPT" | grep -qiE "^(/|yes|no|ok|thanks|hi|hello|hey|commit|push|pull|git )"; then
  exit 0
fi

# ============================================
# STEP 4: Intent Classification + Routing
# ============================================

# Classify the prompt intent and return intent + depth
classify_intent() {
    local prompt="$1"
    local lower_prompt
    lower_prompt=$(echo "$prompt" | tr '[:upper:]' '[:lower:]')

    if echo "$lower_prompt" | grep -qE '(error|bug|fix|crash|fail|broken|exception|traceback|stack trace|segfault|panic)'; then
        echo "debug 5"
    elif echo "$lower_prompt" | grep -qE '(refactor|rename|restructure|reorganize|clean up|simplify|extract|split|merge)'; then
        echo "refactor 4"
    elif echo "$lower_prompt" | grep -qE '(how does|why does|why is|architecture|design|pattern|explain|understand|overview|concept)'; then
        echo "conceptual 4"
    elif echo "$lower_prompt" | grep -qE '(what is|which|where is|where are|who|list all|show me|find the)'; then
        echo "factual 2"
    elif echo "$lower_prompt" | grep -qE '(implement|add|create|build|write|generate|set up|configure|install)'; then
        echo "implement 3"
    else
        echo "general 3"
    fi
}

# Map intent to retrieval configuration
route_retrieval() {
    local intent="$1"

    case "$intent" in
        debug)
            USE_VECTOR_SEARCH=true
            MAX_CODE_RESULTS=5
            MEMORY_TOKEN_BUDGET=300
            PREFER_ENTITY_SEARCH=true
            ;;
        conceptual)
            USE_VECTOR_SEARCH=true
            MAX_CODE_RESULTS=2
            MEMORY_TOKEN_BUDGET=800
            PREFER_ENTITY_SEARCH=false
            ;;
        factual)
            USE_VECTOR_SEARCH=false
            MAX_CODE_RESULTS=2
            MEMORY_TOKEN_BUDGET=400
            PREFER_ENTITY_SEARCH=true
            ;;
        refactor)
            USE_VECTOR_SEARCH=true
            MAX_CODE_RESULTS=4
            MEMORY_TOKEN_BUDGET=400
            PREFER_ENTITY_SEARCH=true
            ;;
        implement)
            USE_VECTOR_SEARCH=true
            MAX_CODE_RESULTS=4
            MEMORY_TOKEN_BUDGET=500
            PREFER_ENTITY_SEARCH=false
            ;;
        *)  # general
            USE_VECTOR_SEARCH=true
            MAX_CODE_RESULTS=3
            MEMORY_TOKEN_BUDGET=500
            PREFER_ENTITY_SEARCH=false
            ;;
    esac
}

# Estimate prompt complexity → adaptive token budget
estimate_complexity() {
    local prompt="$1"
    local score=0

    # Factor 1: Word count
    local word_count
    word_count=$(echo "$prompt" | wc -w | tr -d ' ')
    if [ "$word_count" -gt 50 ]; then
        score=$((score + 2))
    elif [ "$word_count" -gt 20 ]; then
        score=$((score + 1))
    fi

    # Factor 2: Entity count (file paths, PascalCase, etc.)
    local entity_count
    entity_count=$(echo "$prompt" | grep -oE '[a-zA-Z0-9_/-]+\.[a-zA-Z]{1,6}' | wc -l | tr -d ' ')
    entity_count=${entity_count:-0}
    if [ "$entity_count" -gt 3 ]; then
        score=$((score + 2))
    elif [ "$entity_count" -gt 0 ]; then
        score=$((score + 1))
    fi

    # Factor 3: Question markers
    local question_count
    question_count=$(echo "$prompt" | grep -oE '\?' | wc -l | tr -d ' ')
    question_count=${question_count:-0}
    if [ "$question_count" -gt 1 ]; then
        score=$((score + 1))
    fi

    # Factor 4: Code references (backticks, code-like patterns)
    if echo "$prompt" | grep -qE '(`[^`]+`|```|function |class |import |require\()'; then
        score=$((score + 1))
    fi

    # Factor 5: Error messages
    if echo "$prompt" | grep -qE '(Error:|error:|ERRO|stack trace|at line|TypeError|ReferenceError)'; then
        score=$((score + 1))
    fi

    # Map score to budget
    if [ "$score" -ge 5 ]; then
        MAX_CONTEXT_TOKENS=1500
        MAX_CODE_RESULTS=$((MAX_CODE_RESULTS + 2))
    elif [ "$score" -ge 3 ]; then
        MAX_CONTEXT_TOKENS=800
        MAX_CODE_RESULTS=$((MAX_CODE_RESULTS + 1))
    else
        MAX_CONTEXT_TOKENS=300
    fi
}

# ============================================
# Apply intent classification and routing
# ============================================
INTENT_RESULT=$(classify_intent "$PROMPT")
INTENT=$(echo "$INTENT_RESULT" | awk '{print $1}')
INTENT_DEPTH=$(echo "$INTENT_RESULT" | awk '{print $2}')

route_retrieval "$INTENT"
estimate_complexity "$PROMPT"

# Extract meaningful search terms from the prompt
extract_search_query() {
  echo "$1" | \
    tr '[:upper:]' '[:lower:]' | \
    tr -cs '[:alnum:]' ' ' | \
    tr ' ' '\n' | \
    grep -vE '^(the|a|an|is|are|was|were|be|been|being|have|has|had|do|does|did|will|would|could|should|may|might|must|can|this|that|these|those|it|its|i|you|we|they|he|she|what|how|why|when|where|which|who|whom|and|or|but|if|then|else|for|to|of|in|on|at|by|with|from|about|into|through|during|before|after|above|below|between|under|over|out|up|down|off|just|only|also|very|really|please|help|me|my|your|can|want|need|like|make|create|add|fix|update|change|show|tell|explain|find|search|look|get|write|read|use|implement|build)$' | \
    awk 'length >= 3' | \
    head -8 | \
    tr '\n' ' '
}

SEARCH_QUERY=$(extract_search_query "$PROMPT")

if [ -z "$SEARCH_QUERY" ]; then
  exit 0
fi

# ============================================
# STEP 5: Parallel Multi-View Retrieval
# ============================================

# Create temp directory for parallel results
TMPDIR_HOOK=$(mktemp -d "${TMPDIR:-/tmp}/rag-hook.XXXXXX")
trap 'rm -rf "$TMPDIR_HOOK"' EXIT

# Wait for background jobs with timeout
wait_with_timeout() {
    local timeout=8  # 8s max (2s margin from 10s hook timeout)
    local start=$SECONDS
    for pid in "$@"; do
        local elapsed=$(( SECONDS - start ))
        local remaining=$(( timeout - elapsed ))
        if [ "$remaining" -le 0 ]; then
            kill "$pid" 2>/dev/null || true
            continue
        fi
        # Poll with short sleep
        while kill -0 "$pid" 2>/dev/null; do
            elapsed=$(( SECONDS - start ))
            if [ $(( elapsed )) -ge "$timeout" ]; then
                kill "$pid" 2>/dev/null || true
                break
            fi
            sleep 0.1
        done
    done
}

# --- Background job 1: Memory retrieval ---
(
    VECTOR_RESULTS=""
    MEMORY_CONTEXT=""
    ENTITY_RESULTS=""

    if [ -x "$MEMORY_SCRIPT" ]; then
        # Vector search (if enabled)
        if [ "$USE_VECTOR_SEARCH" = true ]; then
            VECTOR_RESULTS=$("$MEMORY_SCRIPT" vsearch "$SEARCH_QUERY" 5 2>/dev/null || echo "")
            # Filter out fallback messages
            if echo "$VECTOR_RESULTS" | grep -q "Falling back"; then
                VECTOR_RESULTS=""
            fi
        fi

        # Structured context (facts, knowledge, recent sessions)
        MEMORY_CONTEXT=$("$MEMORY_SCRIPT" context "$SEARCH_QUERY" $MEMORY_TOKEN_BUDGET 2>/dev/null || echo "")

        # Entity search (if enabled)
        if [ "$PREFER_ENTITY_SEARCH" = true ]; then
            ENTITY_RESULTS=$("$MEMORY_SCRIPT" entity-search "$SEARCH_QUERY" 2>/dev/null || echo "")
        fi
    fi

    echo "$VECTOR_RESULTS" > "$TMPDIR_HOOK/vector_results"
    echo "$MEMORY_CONTEXT" > "$TMPDIR_HOOK/memory_context"
    echo "$ENTITY_RESULTS" > "$TMPDIR_HOOK/entity_results"
) &
PID_MEMORY=$!

# --- Background job 2: QMD code search ---
(
    CONTEXT=""
    QMD_AVAILABLE=false

    if command -v qmd &> /dev/null; then
        if qmd status 2>/dev/null | grep -q "Collection"; then
            QMD_AVAILABLE=true
        fi
    fi

    if [ "$QMD_AVAILABLE" = true ]; then
        SEARCH_RESULTS=$(qmd search "$SEARCH_QUERY" -n $MAX_CODE_RESULTS --json 2>/dev/null || true)

        if [ -n "$SEARCH_RESULTS" ] && [ "$SEARCH_RESULTS" != "[]" ]; then
            RESULT_COUNT=0

            while IFS= read -r result; do
                [ -z "$result" ] && continue

                FILE_PATH=$(echo "$result" | jq -r '.path // .file // .docid // empty' 2>/dev/null)
                SNIPPET=$(echo "$result" | jq -r '.snippet // .content // .text // empty' 2>/dev/null)
                SCORE=$(echo "$result" | jq -r '.score // "N/A"' 2>/dev/null)

                if [ -n "$FILE_PATH" ] && [ -n "$SNIPPET" ]; then
                    CLEAN_PATH=$(echo "$FILE_PATH" | sed 's|^qmd://[^/]*/||')
                    CONTEXT="$CONTEXT
### $CLEAN_PATH (relevance: $SCORE)
\`\`\`
$SNIPPET
\`\`\`
"
                    RESULT_COUNT=$((RESULT_COUNT + 1))
                fi
            done < <(echo "$SEARCH_RESULTS" | jq -c '.[]' 2>/dev/null)

            # Fallback if JSON parsing failed
            if [ -z "$CONTEXT" ] || [ $RESULT_COUNT -eq 0 ]; then
                FILE_PATHS=$(qmd search "$SEARCH_QUERY" --files -n $MAX_CODE_RESULTS 2>/dev/null | head -$MAX_CODE_RESULTS)
                if [ -n "$FILE_PATHS" ]; then
                    while IFS=',' read -r id score path rest; do
                        if [ -n "$path" ]; then
                            CLEAN_PATH=$(echo "$path" | sed 's|^qmd://[^/]*/||')
                            SNIPPET=$(qmd get "$path" -l 30 2>/dev/null | head -30 || true)
                            if [ -n "$SNIPPET" ]; then
                                CONTEXT="$CONTEXT
### $CLEAN_PATH
\`\`\`
$SNIPPET
\`\`\`
"
                            fi
                        fi
                    done <<< "$FILE_PATHS"
                fi
            fi
        fi
    fi

    echo "$CONTEXT" > "$TMPDIR_HOOK/qmd_results"
) &
PID_QMD=$!

# Wait for both jobs
wait_with_timeout $PID_MEMORY $PID_QMD

# Collect results from temp files
VECTOR_RESULTS=$(cat "$TMPDIR_HOOK/vector_results" 2>/dev/null || echo "")
MEMORY_CONTEXT=$(cat "$TMPDIR_HOOK/memory_context" 2>/dev/null || echo "")
ENTITY_RESULTS=$(cat "$TMPDIR_HOOK/entity_results" 2>/dev/null || echo "")
QMD_CONTEXT=$(cat "$TMPDIR_HOOK/qmd_results" 2>/dev/null || echo "")

# ============================================
# Deduplication: remove QMD results for files already in memory
# ============================================
deduplicate_results() {
    local memory_text="$1"
    local qmd_text="$2"

    if [ -z "$memory_text" ] || [ -z "$qmd_text" ]; then
        echo "$qmd_text"
        return
    fi

    # Extract file paths already mentioned in memory context
    local memory_files
    memory_files=$(echo "$memory_text" | grep -oE '[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,6}' | sort -u)

    local deduped="$qmd_text"
    while IFS= read -r file; do
        [ -z "$file" ] && continue
        # Remove QMD sections that reference already-known files
        deduped=$(echo "$deduped" | awk -v f="$file" '
            /^### / { if (index($0, f) > 0) { skip=1; next } else { skip=0 } }
            !skip { print }
        ')
    done <<< "$memory_files"

    echo "$deduped"
}

QMD_CONTEXT=$(deduplicate_results "$MEMORY_CONTEXT $VECTOR_RESULTS $ENTITY_RESULTS" "$QMD_CONTEXT")

# ============================================
# STEP 6: Final Output Assembly
# ============================================

emit_context() {
    local output=""
    local char_limit=$((MAX_CONTEXT_TOKENS * 4))  # ~4 chars per token

    output+="
<relevant-context source=\"turbo-search-rag\">
The following context was automatically retrieved based on your prompt.
Use this to answer without reading additional files unless more detail is needed.

**Intent:** $INTENT (depth: $INTENT_DEPTH) | **Budget:** ~${MAX_CONTEXT_TOKENS} tokens
**Search terms:** $SEARCH_QUERY
"

    # Priority order: Semantic → Entity → Memory → Code

    # 1. Semantic search results (highest relevance)
    if [ -n "$VECTOR_RESULTS" ]; then
        output+="
---
# Semantic Matches

$VECTOR_RESULTS
"
    fi

    # 2. Entity search results
    if [ -n "$ENTITY_RESULTS" ]; then
        output+="
---
# Entity Matches

$ENTITY_RESULTS
"
    fi

    # 3. Memory context (structured knowledge)
    if [ -n "$MEMORY_CONTEXT" ]; then
        output+="
---
# Memory Context

$MEMORY_CONTEXT
"
    fi

    # 4. Code context from QMD
    if [ -n "$QMD_CONTEXT" ]; then
        output+="
---
# Code Context
$QMD_CONTEXT
"
    fi

    output+="</relevant-context>
"

    # Apply adaptive token budget truncation
    echo "$output" | head -c "$char_limit"
}

# Only emit if we have any context
if [ -n "$MEMORY_CONTEXT" ] || [ -n "$QMD_CONTEXT" ] || [ -n "$VECTOR_RESULTS" ] || [ -n "$ENTITY_RESULTS" ]; then
    emit_context
fi
