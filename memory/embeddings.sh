#!/bin/bash
# embeddings.sh - Generate embeddings using ollama or API
#
# Usage:
#   embeddings.sh generate "text to embed"     # Generate single embedding
#   embeddings.sh batch                        # Process embedding queue
#   embeddings.sh status                       # Show embedding provider status
#   embeddings.sh setup                        # Interactive setup wizard

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Find repo root for config
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
MEMORY_DIR="$REPO_ROOT/.claude-memory"
CONFIG_FILE="$MEMORY_DIR/embedding-config.json"
DB_FILE="$MEMORY_DIR/memory.db"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Load configuration
load_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        echo ""
        return 1
    fi
    cat "$CONFIG_FILE"
}

get_config_value() {
    local key="$1"
    local config
    config=$(load_config)
    if [ -z "$config" ]; then
        echo ""
        return
    fi
    echo "$config" | jq -r ".$key // empty" 2>/dev/null
}

# Generate embedding using Ollama
generate_ollama() {
    local text="$1"
    local host
    local model

    host=$(get_config_value "ollama.host")
    model=$(get_config_value "model")

    host="${host:-http://localhost:11434}"
    model="${model:-bge-small-en}"

    # Call Ollama embedding API
    local response
    response=$(curl -s "$host/api/embeddings" \
        -H "Content-Type: application/json" \
        -d "{\"model\": \"$model\", \"prompt\": $(echo "$text" | jq -Rs .)}" 2>/dev/null)

    if [ $? -ne 0 ]; then
        echo "ERROR: Failed to connect to Ollama at $host" >&2
        return 1
    fi

    # Check for error in response
    local error
    error=$(echo "$response" | jq -r '.error // empty' 2>/dev/null)
    if [ -n "$error" ]; then
        echo "ERROR: $error" >&2
        return 1
    fi

    # Extract embedding array
    echo "$response" | jq -c '.embedding'
}

# Generate embedding using OpenAI API
generate_openai() {
    local text="$1"
    local api_key
    local model

    api_key=$(get_config_value "openai.api_key")
    model=$(get_config_value "openai.model")

    model="${model:-text-embedding-3-small}"

    if [ -z "$api_key" ]; then
        echo "ERROR: OpenAI API key not configured" >&2
        return 1
    fi

    local response
    response=$(curl -s "https://api.openai.com/v1/embeddings" \
        -H "Authorization: Bearer $api_key" \
        -H "Content-Type: application/json" \
        -d "{\"input\": $(echo "$text" | jq -Rs .), \"model\": \"$model\"}" 2>/dev/null)

    if [ $? -ne 0 ]; then
        echo "ERROR: Failed to connect to OpenAI API" >&2
        return 1
    fi

    # Check for error
    local error
    error=$(echo "$response" | jq -r '.error.message // empty' 2>/dev/null)
    if [ -n "$error" ]; then
        echo "ERROR: $error" >&2
        return 1
    fi

    # Extract embedding
    echo "$response" | jq -c '.data[0].embedding'
}

# Generate embedding using Voyage AI
generate_voyage() {
    local text="$1"
    local api_key
    local model

    api_key=$(get_config_value "voyage.api_key")
    model=$(get_config_value "voyage.model")

    model="${model:-voyage-3-lite}"

    if [ -z "$api_key" ]; then
        echo "ERROR: Voyage API key not configured" >&2
        return 1
    fi

    local response
    response=$(curl -s "https://api.voyageai.com/v1/embeddings" \
        -H "Authorization: Bearer $api_key" \
        -H "Content-Type: application/json" \
        -d "{\"input\": [$(echo "$text" | jq -Rs .)], \"model\": \"$model\"}" 2>/dev/null)

    if [ $? -ne 0 ]; then
        echo "ERROR: Failed to connect to Voyage API" >&2
        return 1
    fi

    local error
    error=$(echo "$response" | jq -r '.detail // empty' 2>/dev/null)
    if [ -n "$error" ]; then
        echo "ERROR: $error" >&2
        return 1
    fi

    echo "$response" | jq -c '.data[0].embedding'
}

# Main embedding generation function
cmd_generate() {
    local text="$1"

    if [ -z "$text" ]; then
        echo "ERROR: No text provided" >&2
        return 1
    fi

    local provider
    provider=$(get_config_value "provider")
    provider="${provider:-ollama}"

    case "$provider" in
        ollama)
            generate_ollama "$text"
            ;;
        openai)
            generate_openai "$text"
            ;;
        voyage)
            generate_voyage "$text"
            ;;
        *)
            echo "ERROR: Unknown provider: $provider" >&2
            return 1
            ;;
    esac
}

# Convert JSON array to binary blob for sqlite-vector
embedding_to_blob() {
    local json_array="$1"
    # Convert JSON array of floats to binary format
    # sqlite-vector expects little-endian float32 values
    echo "$json_array" | jq -r '.[]' | while read -r val; do
        python3 -c "import struct; import sys; sys.stdout.buffer.write(struct.pack('<f', $val))"
    done | base64
}

# Process embedding queue
cmd_batch() {
    if [ ! -f "$DB_FILE" ]; then
        echo "No memory database found."
        return 1
    fi

    local pending
    pending=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM embedding_queue WHERE status = 'pending';" 2>/dev/null || echo "0")

    if [ "$pending" -eq 0 ]; then
        echo "No pending embeddings to process."
        return 0
    fi

    echo -e "${BLUE}Processing $pending pending embeddings...${NC}"

    local processed=0
    local errors=0

    # Get pending items
    sqlite3 -separator $'\t' "$DB_FILE" "SELECT id, source_type, source_id, content FROM embedding_queue WHERE status = 'pending' LIMIT 50;" 2>/dev/null | \
    while IFS=$'\t' read -r queue_id source_type source_id content; do
        # Mark as processing
        sqlite3 "$DB_FILE" "UPDATE embedding_queue SET status = 'processing' WHERE id = $queue_id;"

        # Generate embedding
        local embedding
        embedding=$(cmd_generate "$content" 2>&1)

        if echo "$embedding" | grep -q "^ERROR"; then
            # Mark as error
            local error_msg
            error_msg=$(echo "$embedding" | sed 's/ERROR: //')
            sqlite3 "$DB_FILE" "UPDATE embedding_queue SET status = 'error', error_message = '$(echo "$error_msg" | sed "s/'/''/g")' WHERE id = $queue_id;"
            echo -e "${RED}✗${NC} Failed: $source_type #$source_id - $error_msg"
            errors=$((errors + 1))
            continue
        fi

        # Convert to blob and store
        local blob
        blob=$(embedding_to_blob "$embedding")

        # Update the source table with embedding
        case "$source_type" in
            session)
                sqlite3 "$DB_FILE" "UPDATE sessions SET embedding = X'$(echo "$blob" | base64 -d | xxd -p | tr -d '\n')' WHERE id = $source_id;"
                ;;
            knowledge)
                sqlite3 "$DB_FILE" "UPDATE knowledge SET embedding = X'$(echo "$blob" | base64 -d | xxd -p | tr -d '\n')' WHERE id = $source_id;"
                ;;
            fact)
                sqlite3 "$DB_FILE" "UPDATE facts SET embedding = X'$(echo "$blob" | base64 -d | xxd -p | tr -d '\n')' WHERE id = $source_id;"
                ;;
        esac

        # Mark as done
        sqlite3 "$DB_FILE" "UPDATE embedding_queue SET status = 'done', processed_at = CURRENT_TIMESTAMP WHERE id = $queue_id;"
        echo -e "${GREEN}✓${NC} Embedded: $source_type #$source_id"
        processed=$((processed + 1))
    done

    echo ""
    echo -e "Processed: ${GREEN}$processed${NC}, Errors: ${RED}$errors${NC}"
}

# Show status
cmd_status() {
    echo -e "${BLUE}Embedding Configuration${NC}"
    echo "========================"

    if [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${YELLOW}No configuration found.${NC}"
        echo "Run: $0 setup"
        return 1
    fi

    local provider model dimension
    provider=$(get_config_value "provider")
    model=$(get_config_value "model")
    dimension=$(get_config_value "dimension")

    echo "Provider:  $provider"
    echo "Model:     $model"
    echo "Dimension: $dimension"
    echo ""

    # Check provider availability
    case "$provider" in
        ollama)
            local host
            host=$(get_config_value "ollama.host")
            host="${host:-http://localhost:11434}"
            echo -n "Ollama status: "
            if curl -s "$host/api/tags" >/dev/null 2>&1; then
                echo -e "${GREEN}Connected${NC} ($host)"
                # Check if model is available
                if curl -s "$host/api/tags" | jq -e ".models[] | select(.name == \"$model\")" >/dev/null 2>&1; then
                    echo -e "Model $model: ${GREEN}Available${NC}"
                else
                    echo -e "Model $model: ${YELLOW}Not found${NC} - run: ollama pull $model"
                fi
            else
                echo -e "${RED}Not running${NC}"
                echo "Start with: ollama serve"
            fi
            ;;
        openai)
            local api_key
            api_key=$(get_config_value "openai.api_key")
            if [ -n "$api_key" ]; then
                echo -e "OpenAI API key: ${GREEN}Configured${NC}"
            else
                echo -e "OpenAI API key: ${RED}Not set${NC}"
            fi
            ;;
        voyage)
            local api_key
            api_key=$(get_config_value "voyage.api_key")
            if [ -n "$api_key" ]; then
                echo -e "Voyage API key: ${GREEN}Configured${NC}"
            else
                echo -e "Voyage API key: ${RED}Not set${NC}"
            fi
            ;;
    esac

    echo ""

    # Show queue status
    if [ -f "$DB_FILE" ]; then
        echo -e "${BLUE}Embedding Queue${NC}"
        sqlite3 "$DB_FILE" "SELECT status, COUNT(*) as count FROM embedding_queue GROUP BY status;" 2>/dev/null || echo "No queue data"
    fi
}

# Interactive setup wizard
cmd_setup() {
    mkdir -p "$MEMORY_DIR"

    echo -e "${BLUE}Embedding Setup Wizard${NC}"
    echo "======================"
    echo ""
    echo "Choose your embedding provider:"
    echo ""
    echo "  1) Ollama (local, free, recommended)"
    echo "     - Runs locally, no API costs"
    echo "     - Requires ~500MB disk for model"
    echo "     - Model: bge-small-en (384 dimensions)"
    echo ""
    echo "  2) OpenAI API"
    echo "     - Fast and reliable"
    echo "     - Requires API key (\$0.02 per 1M tokens)"
    echo "     - Model: text-embedding-3-small (1536 dimensions)"
    echo ""
    echo "  3) Voyage AI"
    echo "     - High quality embeddings"
    echo "     - Requires API key"
    echo "     - Model: voyage-3-lite (512 dimensions)"
    echo ""

    read -p "Select provider [1-3]: " choice

    case "$choice" in
        1)
            setup_ollama
            ;;
        2)
            setup_openai
            ;;
        3)
            setup_voyage
            ;;
        *)
            echo "Invalid choice"
            return 1
            ;;
    esac
}

setup_ollama() {
    echo ""
    echo -e "${BLUE}Setting up Ollama...${NC}"

    # Check if ollama is installed
    if ! command -v ollama &> /dev/null; then
        echo -e "${YELLOW}Ollama not found.${NC}"
        echo ""
        echo "Install Ollama:"
        echo "  macOS:  brew install ollama"
        echo "  Linux:  curl -fsSL https://ollama.ai/install.sh | sh"
        echo ""
        read -p "Press Enter after installing ollama..."
    fi

    # Check if ollama is running
    local host="http://localhost:11434"
    read -p "Ollama host [$host]: " input_host
    host="${input_host:-$host}"

    if ! curl -s "$host/api/tags" >/dev/null 2>&1; then
        echo -e "${YELLOW}Ollama not running at $host${NC}"
        echo "Start it with: ollama serve"
        echo ""
        read -p "Press Enter after starting ollama..."
    fi

    # Pull model
    local model="bge-small-en"
    echo ""
    echo "Pulling embedding model: $model"
    echo "(This downloads ~130MB on first run)"
    echo ""

    ollama pull "$model" || {
        echo -e "${RED}Failed to pull model${NC}"
        return 1
    }

    # Save config
    cat > "$CONFIG_FILE" <<EOF
{
  "provider": "ollama",
  "model": "$model",
  "dimension": 384,
  "ollama": {
    "host": "$host"
  }
}
EOF

    echo ""
    echo -e "${GREEN}Ollama configured successfully!${NC}"
    echo "Config saved to: $CONFIG_FILE"
}

setup_openai() {
    echo ""
    echo -e "${BLUE}Setting up OpenAI...${NC}"
    echo ""
    echo "Get your API key from: https://platform.openai.com/api-keys"
    echo ""

    read -p "Enter OpenAI API key: " api_key

    if [ -z "$api_key" ]; then
        echo "API key required"
        return 1
    fi

    # Test the key
    echo "Testing API key..."
    local response
    response=$(curl -s "https://api.openai.com/v1/models" \
        -H "Authorization: Bearer $api_key" 2>/dev/null)

    if echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        echo -e "${RED}Invalid API key${NC}"
        return 1
    fi

    cat > "$CONFIG_FILE" <<EOF
{
  "provider": "openai",
  "model": "text-embedding-3-small",
  "dimension": 1536,
  "openai": {
    "api_key": "$api_key",
    "model": "text-embedding-3-small"
  }
}
EOF

    echo ""
    echo -e "${GREEN}OpenAI configured successfully!${NC}"
    echo "Config saved to: $CONFIG_FILE"
}

setup_voyage() {
    echo ""
    echo -e "${BLUE}Setting up Voyage AI...${NC}"
    echo ""
    echo "Get your API key from: https://www.voyageai.com/"
    echo ""

    read -p "Enter Voyage API key: " api_key

    if [ -z "$api_key" ]; then
        echo "API key required"
        return 1
    fi

    cat > "$CONFIG_FILE" <<EOF
{
  "provider": "voyage",
  "model": "voyage-3-lite",
  "dimension": 512,
  "voyage": {
    "api_key": "$api_key",
    "model": "voyage-3-lite"
  }
}
EOF

    echo ""
    echo -e "${GREEN}Voyage AI configured successfully!${NC}"
    echo "Config saved to: $CONFIG_FILE"
}

# Main command dispatch
case "${1:-}" in
    generate)
        cmd_generate "$2"
        ;;
    batch)
        cmd_batch
        ;;
    status)
        cmd_status
        ;;
    setup)
        cmd_setup
        ;;
    *)
        echo "Usage: $0 {generate|batch|status|setup}"
        echo ""
        echo "Commands:"
        echo "  generate <text>   Generate embedding for text"
        echo "  batch             Process pending embeddings queue"
        echo "  status            Show configuration and provider status"
        echo "  setup             Interactive setup wizard"
        exit 1
        ;;
esac
