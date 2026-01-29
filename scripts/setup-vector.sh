#!/bin/bash
# setup-vector.sh - Set up vector search for Claude Code memory
#
# This script:
# 1. Installs sqlite-vector extension (optional, for advanced use)
# 2. Guides user through embedding provider setup (ollama or API)
# 3. Initializes vector search in the memory database
# 4. Processes existing memory entries to generate embeddings
#
# Usage:
#   ./setup-vector.sh              # Full interactive setup
#   ./setup-vector.sh --ollama     # Quick setup with ollama
#   ./setup-vector.sh --openai     # Quick setup with OpenAI API
#   ./setup-vector.sh --status     # Check current setup status

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(dirname "$SCRIPT_DIR")"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"
EMBEDDINGS_SCRIPT="$PLUGIN_DIR/memory/embeddings.sh"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

print_header() {
    echo ""
    echo -e "${BOLD}${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${BLUE}║       Claude Turbo Search - Vector Search Setup            ║${NC}"
    echo -e "${BOLD}${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_step() {
    echo -e "${BLUE}▶${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_step "Checking prerequisites..."

    local missing=()

    # Required: jq for JSON processing
    if ! command -v jq &> /dev/null; then
        missing+=("jq")
    fi

    # Required: curl for API calls
    if ! command -v curl &> /dev/null; then
        missing+=("curl")
    fi

    # Required: python3 for vector operations
    if ! command -v python3 &> /dev/null; then
        missing+=("python3")
    fi

    # Required: sqlite3
    if ! command -v sqlite3 &> /dev/null; then
        missing+=("sqlite3")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        print_error "Missing required tools: ${missing[*]}"
        echo ""
        echo "Install them using:"
        echo "  macOS:  brew install ${missing[*]}"
        echo "  Ubuntu: sudo apt-get install ${missing[*]}"
        exit 1
    fi

    print_success "All prerequisites installed"
}

# Check and offer ollama installation
setup_ollama() {
    echo ""
    echo -e "${BOLD}Setting up Ollama (Local Embeddings)${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Ollama runs locally on your machine - no API costs, full privacy."
    echo ""

    # Check if ollama is installed
    if ! command -v ollama &> /dev/null; then
        print_warning "Ollama not installed"
        echo ""
        echo "Install Ollama:"
        echo ""
        echo "  ${BOLD}macOS:${NC}"
        echo "    brew install ollama"
        echo ""
        echo "  ${BOLD}Linux:${NC}"
        echo "    curl -fsSL https://ollama.ai/install.sh | sh"
        echo ""
        echo "  ${BOLD}Manual:${NC}"
        echo "    https://ollama.ai/download"
        echo ""

        read -p "Press Enter after installing Ollama (or 'q' to quit): " response
        if [ "$response" = "q" ]; then
            exit 0
        fi

        if ! command -v ollama &> /dev/null; then
            print_error "Ollama still not found. Please install and try again."
            exit 1
        fi
    fi

    print_success "Ollama installed"

    # Check if ollama is running
    if ! curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then
        print_warning "Ollama not running"
        echo ""
        echo "Start Ollama in a separate terminal:"
        echo "  ollama serve"
        echo ""

        read -p "Press Enter after starting Ollama: "

        if ! curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then
            print_error "Cannot connect to Ollama. Please start it and try again."
            exit 1
        fi
    fi

    print_success "Ollama running"

    # Pull embedding model
    local model="bge-small-en"
    echo ""
    print_step "Pulling embedding model: $model"
    echo "  (This downloads ~130MB on first run)"
    echo ""

    if ! ollama pull "$model"; then
        print_error "Failed to pull model"
        exit 1
    fi

    print_success "Model $model ready"

    # Run the embeddings setup
    "$EMBEDDINGS_SCRIPT" setup <<< "1"
}

# Setup with OpenAI API
setup_openai() {
    echo ""
    echo -e "${BOLD}Setting up OpenAI API Embeddings${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "OpenAI provides fast, high-quality embeddings via API."
    echo "Cost: ~\$0.02 per 1 million tokens (very cheap)"
    echo ""
    echo "Get your API key from: https://platform.openai.com/api-keys"
    echo ""

    read -p "Enter your OpenAI API key: " api_key

    if [ -z "$api_key" ]; then
        print_error "API key required"
        exit 1
    fi

    # Test the key
    print_step "Validating API key..."
    local response
    response=$(curl -s "https://api.openai.com/v1/models" \
        -H "Authorization: Bearer $api_key" 2>/dev/null)

    if echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        print_error "Invalid API key"
        exit 1
    fi

    print_success "API key valid"

    # Find repo root
    local repo_root
    repo_root=$(cd "$PLUGIN_DIR" && git rev-parse --show-toplevel 2>/dev/null || echo "$PWD")
    local memory_dir="$repo_root/.claude-memory"
    mkdir -p "$memory_dir"

    # Save config
    cat > "$memory_dir/embedding-config.json" <<EOF
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

    print_success "OpenAI configured"
}

# Initialize vector search in database
init_vector_db() {
    echo ""
    print_step "Initializing vector search in memory database..."

    if [ ! -x "$MEMORY_SCRIPT" ]; then
        print_error "Memory script not found: $MEMORY_SCRIPT"
        exit 1
    fi

    "$MEMORY_SCRIPT" init-vector

    print_success "Vector search initialized"
}

# Process existing memory entries
process_embeddings() {
    echo ""
    print_step "Processing existing memory entries..."

    if [ ! -x "$MEMORY_SCRIPT" ]; then
        print_error "Memory script not found"
        exit 1
    fi

    "$MEMORY_SCRIPT" embed

    print_success "Embeddings processed"
}

# Show current status
show_status() {
    echo ""
    echo -e "${BOLD}Vector Search Status${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Check embeddings configuration
    "$EMBEDDINGS_SCRIPT" status 2>/dev/null || echo "Embeddings not configured"

    echo ""

    # Check memory database
    "$MEMORY_SCRIPT" stats 2>/dev/null || echo "Memory database not initialized"
}

# Main interactive setup
interactive_setup() {
    print_header

    echo "Vector search enables semantic understanding of your codebase memory."
    echo "Instead of keyword matching, it finds conceptually related information."
    echo ""
    echo "Example: searching 'login' will also find 'authentication', 'OAuth', etc."
    echo ""

    check_prerequisites

    echo ""
    echo -e "${BOLD}Choose your embedding provider:${NC}"
    echo ""
    echo "  1) ${GREEN}Ollama (Recommended)${NC}"
    echo "     • Runs locally - no API costs"
    echo "     • Full privacy - data never leaves your machine"
    echo "     • Requires ~500MB disk space"
    echo "     • Model: bge-small-en (384 dimensions)"
    echo ""
    echo "  2) OpenAI API"
    echo "     • Fast and reliable"
    echo "     • Requires API key (~\$0.02 per 1M tokens)"
    echo "     • Model: text-embedding-3-small (1536 dimensions)"
    echo ""
    echo "  3) Skip embedding setup (use keyword search only)"
    echo ""

    read -p "Select option [1-3]: " choice

    case "$choice" in
        1)
            setup_ollama
            ;;
        2)
            setup_openai
            ;;
        3)
            echo ""
            print_warning "Skipping embedding setup"
            echo "You can run this script again later to enable vector search."
            exit 0
            ;;
        *)
            print_error "Invalid choice"
            exit 1
            ;;
    esac

    init_vector_db
    process_embeddings

    echo ""
    echo -e "${GREEN}${BOLD}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}${BOLD}║              Vector Search Setup Complete!                 ║${NC}"
    echo -e "${GREEN}${BOLD}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Your memory database now supports semantic search."
    echo ""
    echo "Usage:"
    echo "  • FTS search:    ./memory/memory-db.sh search \"query\""
    echo "  • Vector search: ./memory/memory-db.sh vsearch \"query\""
    echo "  • Check status:  ./memory/memory-db.sh stats"
    echo ""
    echo "The RAG hook will automatically use vector search when available."
    echo ""
}

# Parse arguments
case "${1:-}" in
    --ollama)
        print_header
        check_prerequisites
        setup_ollama
        init_vector_db
        process_embeddings
        print_success "Setup complete!"
        ;;
    --openai)
        print_header
        check_prerequisites
        setup_openai
        init_vector_db
        process_embeddings
        print_success "Setup complete!"
        ;;
    --status)
        show_status
        ;;
    --help|-h)
        echo "Usage: $0 [OPTIONS]"
        echo ""
        echo "Set up vector search for Claude Code memory."
        echo ""
        echo "Options:"
        echo "  (none)      Interactive setup wizard"
        echo "  --ollama    Quick setup with Ollama (local)"
        echo "  --openai    Quick setup with OpenAI API"
        echo "  --status    Show current setup status"
        echo "  --help      Show this help message"
        ;;
    *)
        interactive_setup
        ;;
esac
