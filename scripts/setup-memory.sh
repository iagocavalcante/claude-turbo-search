#!/bin/bash
# setup-memory.sh - Initialize persistent memory for the current repository
#
# Usage:
#   ./setup-memory.sh           # Initialize memory in current repo
#   ./setup-memory.sh --status  # Check memory status
#   ./setup-memory.sh --reset   # Reset memory (delete and reinitialize)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(dirname "$SCRIPT_DIR")"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Find repo root
find_repo_root() {
    local dir="$PWD"
    while [ "$dir" != "/" ]; do
        if [ -d "$dir/.git" ]; then
            echo "$dir"
            return 0
        fi
        dir="$(dirname "$dir")"
    done
    echo ""
}

REPO_ROOT="$(find_repo_root)"
if [ -z "$REPO_ROOT" ]; then
    echo -e "${RED}Error: Not in a git repository${NC}"
    echo "Memory is stored per-repository. Please run from within a git repo."
    exit 1
fi

MEMORY_DIR="$REPO_ROOT/.claude-memory"
DB_FILE="$MEMORY_DIR/memory.db"

case "${1:-}" in
    --status)
        echo -e "${BLUE}Memory Status${NC}"
        echo "Repository: $REPO_ROOT"
        echo ""

        if [ -f "$DB_FILE" ]; then
            echo -e "${GREEN}Memory database exists${NC}"
            echo "Location: $DB_FILE"
            echo "Size: $(du -h "$DB_FILE" | cut -f1)"
            echo ""
            "$MEMORY_SCRIPT" stats
        else
            echo -e "${YELLOW}No memory database found${NC}"
            echo "Run 'setup-memory.sh' to initialize."
        fi
        ;;

    --reset)
        if [ -d "$MEMORY_DIR" ]; then
            echo -e "${YELLOW}Warning: This will delete all memory for this repository${NC}"
            echo "Location: $MEMORY_DIR"
            read -p "Are you sure? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                rm -rf "$MEMORY_DIR"
                echo -e "${GREEN}Memory deleted${NC}"
                "$MEMORY_SCRIPT" init
            else
                echo "Cancelled."
            fi
        else
            echo "No memory to reset."
        fi
        ;;

    --help|-h)
        echo "Usage: $0 [--status|--reset|--help]"
        echo ""
        echo "Options:"
        echo "  (none)    Initialize memory database"
        echo "  --status  Show memory status and statistics"
        echo "  --reset   Delete and reinitialize memory"
        echo "  --help    Show this help message"
        echo ""
        echo "Memory is stored in .claude-memory/ at the repository root."
        echo "Add .claude-memory/ to .gitignore to keep it local."
        ;;

    *)
        echo -e "${BLUE}Setting up Claude Code memory...${NC}"
        echo "Repository: $REPO_ROOT"
        echo ""

        # Make memory script executable
        chmod +x "$MEMORY_SCRIPT"

        # Initialize database
        "$MEMORY_SCRIPT" init

        # Add to .gitignore if not already there
        GITIGNORE="$REPO_ROOT/.gitignore"
        if [ -f "$GITIGNORE" ]; then
            if ! grep -q ".claude-memory" "$GITIGNORE"; then
                echo "" >> "$GITIGNORE"
                echo "# Claude Code memory (local)" >> "$GITIGNORE"
                echo ".claude-memory/" >> "$GITIGNORE"
                echo -e "${GREEN}Added .claude-memory/ to .gitignore${NC}"
            fi
        else
            echo "# Claude Code memory (local)" > "$GITIGNORE"
            echo ".claude-memory/" >> "$GITIGNORE"
            echo -e "${GREEN}Created .gitignore with .claude-memory/${NC}"
        fi

        echo ""
        echo -e "${GREEN}Memory initialized successfully!${NC}"
        echo ""
        echo "Next steps:"
        echo "  1. Use /remember at the end of work sessions to save context"
        echo "  2. Memory will be automatically injected into future prompts"
        echo ""
        echo "Commands:"
        echo "  $0 --status  # Check memory statistics"
        echo "  $0 --reset   # Reset memory database"
        ;;
esac
