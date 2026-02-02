# Claude Turbo Search

Optimized file search and semantic indexing for large codebases in Claude Code.

## Features

- **Fast file suggestions** - ripgrep + fzf for instant autocomplete
- **Semantic search** - QMD integration for finding relevant docs by meaning
- **Cartographer integration** - Automatic codebase mapping
- **One command setup** - `/turbo-index` does everything
- **QMD skill** - `/qmd` teaches Claude to search before reading files
- **Optional hooks** - Auto-inject relevant context before prompts

## Requirements

### Supported Platforms

| Platform | Package Manager | Status |
|----------|-----------------|--------|
| macOS | Homebrew | Fully supported |
| Ubuntu/Debian | apt | Fully supported |
| Fedora/RHEL | dnf | Fully supported |
| Arch Linux | pacman | Fully supported |
| Windows | - | Not supported (use WSL) |

### Prerequisites

- [Claude Code CLI](https://claude.ai/claude-code) installed
- Bash 4.0+ (default on macOS and Linux)
- A supported package manager (see above)

## Installation

### Option 1: Install from GitHub (recommended)

Add the repository as a marketplace and install:

```bash
# Add the marketplace from GitHub (use #branch for specific branch)
claude plugin marketplace add iagocavalcante/claude-turbo-search

# Or install from a specific branch
claude plugin marketplace add "iagocavalcante/claude-turbo-search#feature/vector-search-rag"

# Install the plugin
claude plugin install claude-turbo-search@claude-turbo-search-dev

# Restart Claude Code to load the plugin
```

### Option 2: From official marketplace (when published)

```bash
claude plugin install claude-turbo-search
```

### Updating the Plugin

When updates are available:

```bash
# Update the marketplace to fetch latest changes
claude plugin marketplace update claude-turbo-search-dev

# Update the plugin
claude plugin update claude-turbo-search@claude-turbo-search-dev

# Restart Claude Code to apply updates
```

### Verify Installation

```bash
claude plugin list
```

You should see:
```
❯ claude-turbo-search@claude-turbo-search-dev
  Version: 1.0.0
  Status: ✔ enabled
```

## Usage

In any project, run:

```
/turbo-index
```

This will:

1. Check and install dependencies (ripgrep, fzf, jq, bun, qmd)
2. Configure fast file suggestions
3. Set up QMD MCP server for semantic search
4. Run cartographer to map the codebase
5. Index all documentation with QMD

### Subsequent runs

Running `/turbo-index` again will:
- Skip dependency installation
- Skip global configuration
- Refresh the project index if files changed

### Available Skills

| Skill | Description |
|-------|-------------|
| `/turbo-index` | Set up optimized search indexing for a project |
| `/qmd` | Search docs before reading to save tokens |
| `/remember` | Save session context to persistent memory |
| `/memory-stats` | View memory database statistics |
| `/token-stats` | Show token economics and savings dashboard |

### Using the QMD Skill

After indexing, use `/qmd` or just ask Claude to search:

```
"Search for authentication logic in this project"
"Find files related to database migrations"
```

Claude will use QMD to find relevant files **before** reading them, saving significant tokens.

### Using Memory Skills

Track your work across sessions:

```bash
# At end of session, save context to memory
/remember

# View accumulated knowledge
/memory-stats

# See token savings in action
/token-stats
```

The memory system uses SQLite FTS5 for instant search across all your saved sessions, knowledge, and facts.

### Manual QMD Commands

```bash
# Fast keyword search (use this first)
qmd search "your query" --files -n 10

# Semantic search (slower, use as fallback)
qmd vsearch "how does the login flow work"

# Get specific file content
qmd get "path/to/file.md"
```

### Optional: Auto-Context Hooks

Enable automatic context injection that searches QMD before each prompt:

```bash
# Simple mode - suggests relevant file paths (lightweight)
~/claude-turbo-search/scripts/setup-hooks.sh

# RAG mode - injects actual content snippets (recommended)
~/claude-turbo-search/scripts/setup-hooks.sh --rag

# Remove hooks
~/claude-turbo-search/scripts/setup-hooks.sh --remove
```

#### Hook Modes Comparison

| Mode | Token Cost | How It Works |
|------|------------|--------------|
| Simple | ~50-100/prompt | Suggests file paths, Claude decides what to read |
| RAG | ~500-2000/prompt | Injects content snippets, Claude often needs no file reads |

**RAG mode** is recommended for large codebases - the upfront token cost is offset by avoiding file reads.

#### How RAG Mode Works

```
1. You submit: "How does authentication work?"
2. Hook extracts: "authentication work"
3. QMD searches indexed docs
4. Hook injects relevant snippets into context
5. Claude answers using injected context
6. No file reads needed = massive token savings
```

## Dependencies

| Tool | Purpose |
|------|---------|
| [ripgrep](https://github.com/BurntSushi/ripgrep) | Fast file search |
| [fzf](https://github.com/junegunn/fzf) | Fuzzy finder |
| [jq](https://github.com/stedolan/jq) | JSON parsing |
| [bun](https://bun.sh) | JavaScript runtime |
| [qmd](https://github.com/tobi/qmd) | Semantic search engine |

All dependencies are installed automatically on first run using your system's package manager.

## How It Saves Tokens

### Before (traditional exploration)
```
Read file1.md (2000 tokens)
Read file2.md (1500 tokens)
Read file3.md (1800 tokens)
→ Found answer in file3.md
Total: 5300 tokens
```

### After (with turbo search)
```
qmd_search "how does auth work" (50 tokens)
→ Returns: file3.md lines 45-62 (200 tokens)
Total: 250 tokens
```

**Estimated savings: 60-80% on exploration tasks**

## Configuration

After running `/turbo-index`, these files are modified:

- `~/.claude/settings.json` - fileSuggestion and mcpServers config
- `~/.claude/file-suggestion.sh` - turbo file suggestion script
- `.claude/turbo-search.json` - project-specific metadata (in each project)

**Note:** The setup scripts will warn you if existing configuration will be overwritten and create backups automatically.

## MCP Tools

After setup, these MCP tools are available:

| Tool | Description |
|------|-------------|
| `qmd_search` | Semantic search across indexed docs |
| `qmd_get` | Retrieve specific document by path/ID |
| `qmd_collections` | List all indexed projects |

## Troubleshooting

### Dependencies not installing

If automatic installation fails, you can install dependencies manually:

```bash
# macOS
brew install ripgrep fzf jq
brew tap oven-sh/bun && brew install bun
bun install -g https://github.com/tobi/qmd

# Ubuntu/Debian
sudo apt-get install ripgrep fzf jq
curl -fsSL https://bun.sh/install | bash
bun install -g https://github.com/tobi/qmd

# Fedora
sudo dnf install ripgrep fzf jq
curl -fsSL https://bun.sh/install | bash
bun install -g https://github.com/tobi/qmd
```

### QMD models downloading

On first use, QMD downloads ~1.7GB of models. This is normal and only happens once.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute.

## License

MIT - see [LICENSE](LICENSE) for details.
