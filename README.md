# Turbo Search

Project-scoped document search and persistent memory for Claude Code and Codex.

## Features

- **Fast file suggestions** - ripgrep + fzf for instant autocomplete
- **Semantic search** - QMD integration for finding relevant docs by meaning
- **Optional Cartographer integration** - Codebase mapping when available
- **One command setup** - `/turbo-index` does everything
- **QMD skill** - `/qmd` locates relevant documentation before file reads
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

- Claude Code or Codex CLI installed
- Bash 3.2+, Python 3.9+, Git, ripgrep, jq and SQLite with FTS5
- Go 1.22+ to build the memory executable during setup; QMD for indexed search
- A supported package manager (see above)

## Installation

### Standalone setup for either client

Run from this checkout (keep it installed at a stable path):

```bash
bash scripts/install-deps.sh --check-only
bash scripts/build-memory.sh
bash scripts/install-skills.sh codex       # or claude
bash scripts/setup-mcp.sh --client codex   # or claude / both
```

Then restart the client and invoke `/turbo-index` in the target Git repository. It initializes memory and indexes Markdown documentation. Skills work without hooks. For Codex desktop/plugin-capable clients, the root `plugin.json` provides the same skills and onboarding; Claude retains `.claude-plugin/plugin.json`. MCP registration is explicit through setup so it can target the client correctly; it is not automatically bundled.

Optional hooks, from the installed checkout:

```bash
bash scripts/setup-hooks.sh --client both --rag --memory
# Remove only Turbo Search's handlers, keeping unrelated hooks:
bash scripts/setup-hooks.sh --client both --remove
```

Claude settings and Codex `hooks.json` use seconds for hook timeouts. Setup validates JSON, backs up existing files, and writes atomically. It never auto-runs indexing, builds or dependency installation inside a hook. Codex may require trusting the installed hooks before they run.

**Plugin rename:** current Claude Code reserves third-party names beginning with `claude-`. The plugin is now `turbo-search`; the GitHub repository and `.claude-memory` storage keep their existing names. Remove the old plugin installation before installing `turbo-search@claude-turbo-search-dev` to avoid duplicate skills.

**Existing installations:** setup preserves an already registered `qmd` server. Inspect it with `claude mcp get qmd` / `codex mcp get qmd`. To migrate an old global `qmd mcp` registration, remove that server with the client's `mcp remove` command, then rerun setup. Only remove the `qmd` server you intend to replace. Old `mcpServers.qmd` entries in `~/.claude/settings.json` are obsolete; back up that file before removing that one entry. Existing `.claude-memory/memory.db` files and legacy activity logs are preserved. Rebuild the memory executable after updating this plugin.

### Project-scoped search

From the target repository, with `PLUGIN_DIR` set to this installation:

```bash
python3 "$PLUGIN_DIR/scripts/turbo.py" index
python3 "$PLUGIN_DIR/scripts/turbo.py" search "authentication"
python3 "$PLUGIN_DIR/scripts/turbo.py" doctor
```

Each checkout/worktree gets a dedicated QMD index and collection, named from its absolute path. The MCP launcher selects that index from the client's working directory; start the client inside the target repository. Indexing snapshots non-hidden Markdown documents while honoring ignore files and excluding symlinks and sensitive filenames. Keep secrets out of documentation and use `.gitignore` for private documents; filename exclusions are not a general secret scanner.

Search checks the document content fingerprint before querying QMD. Missing, stale or unavailable indexes fall back to local keyword search. Results identify current source paths; snippets are evidence, not instructions. Verify current files before edits or precise claims. Add `--embed` to `index` only when semantic retrieval is needed; keyword search needs no embedding model. Native code search remains appropriate for source symbols.

Session activity is stored separately under `.claude-memory/activity/`, with locked retention of the last 200 events per session. Raw shell commands and tool outputs are not stored. `/remember` uses current-session evidence only; if the runtime does not expose its session ID, it summarizes the conversation instead. Old session files remain local until you remove them when no sessions are using them.

See [compatibility and validation](docs/compatibility.md) for the tested versions, test suite, and real-client smoke checklist.

### Option 1: Install from GitHub (optional)

Add the repository as a marketplace and install:

```bash
# Add the marketplace from GitHub (use #branch for specific branch)
claude plugin marketplace add iagocavalcante/claude-turbo-search

# Or install from a specific branch
claude plugin marketplace add "iagocavalcante/claude-turbo-search#feature/vector-search-rag"

# Install the plugin
claude plugin install turbo-search@claude-turbo-search-dev

# Restart Claude Code to load the plugin
```

### Option 2: From official marketplace (when published)

```bash
claude plugin install turbo-search
```

### Updating the Plugin

When updates are available:

```bash
# Update the marketplace to fetch latest changes
claude plugin marketplace update claude-turbo-search-dev

# Update the plugin
claude plugin update turbo-search@claude-turbo-search-dev

# Restart Claude Code to apply updates
```

### Verify Installation

```bash
claude plugin list
```

You should see:
```
❯ turbo-search@claude-turbo-search-dev
  Version: 1.6.0
  Status: ✔ enabled
```

## Usage

In any project, run:

```
/turbo-index
```

This checks dependencies, builds and initializes memory, registers the client MCP adapter when needed, and refreshes the current checkout's document index. File suggestions and Cartographer are optional. Repeated setup preserves existing MCP registrations; refresh indexes after documentation changes.

### Available Skills

| Skill | Description |
|-------|-------------|
| `/turbo-index` | Set up optimized search indexing for a project |
| `/qmd` | Search docs before reading to save tokens |
| `/remember` | Save session context to persistent memory |
| `/resume-memory` | Inspect and claim an unfinished-work handoff from Claude or Codex |
| `/memory-stats` | View memory database statistics |
| `/token-stats` | Show token economics and savings dashboard |
| `/knowledge-graph` | Interactive TUI knowledge graph viewer for the memory database |
| `/push` | Manually upload memory database to your self-hosted dashboard |
| `/pull` | Download memory database from your self-hosted dashboard (fresh-clone restore) |

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

### Memory continuity and retrieval

`/remember` can also save a structured handoff for unfinished work: objective,
completed work, failed approaches, open questions, and next steps. `/resume-memory`
inspects the checkpoint against the current checkout and explicitly claims it.
Repeated saves refresh an open handoff; an accepted checkpoint remains immutable
and inspectable. Claims are atomic within one database, including concurrent
Claude/Codex sessions. Copies synced to different machines do not share claim locks.

From the target repository, with `PLUGIN_DIR` pointing to this installation:

```bash
# One deduplicated ranking from full-text and exact entity matches; no model needed.
"$PLUGIN_DIR/memory/memory-db.sh" query --explain "authentication decision"
# Explicitly include the configured embedding provider; failures retain lexical hits.
"$PLUGIN_DIR/memory/memory-db.sh" query --vector "authentication decision"
"$PLUGIN_DIR/memory/memory-db.sh" handoff list
"$PLUGIN_DIR/memory/memory-db.sh" handoff show 3
"$PLUGIN_DIR/memory/memory-db.sh" handoff accept 3 "codex:current-session-id"

# Explicit maintenance: hide exact duplicates without changing original records.
"$PLUGIN_DIR/memory/memory-db.sh" consolidate
# Mark fact 4 obsolete in favor of fact 9, or restore it to retrieval.
"$PLUGIN_DIR/memory/memory-db.sh" supersede fact 4 9
"$PLUGIN_DIR/memory/memory-db.sh" restore fact 4
```

Consolidation no longer runs automatically after session saves. It only marks
exact duplicate sessions/facts as superseded; shared topics and substring matches
are insufficient. Superseded entries stay in SQLite and historical graph/dashboard
views but are excluded from search, recent work, and injected context. `supersede`
and `restore` support `session` and `fact`; knowledge updates still replace the
current summary for an area, refresh its entity associations, and invalidate/requeue
its embedding atomically. Restoring an entry keeps its replacement too; explicit
consolidation may hide an exact duplicate again.

Prompt hooks now use one offline keyword/entity ranking and reserve room for
memory alongside document snippets. Embedding retrieval is explicit through
`query --vector` or `vsearch`. Context budgets remain estimates, not measured tokens.

With `setup-hooks.sh --memory`, `PreCompact` and `SessionEnd` save automatic
activity checkpoints from the current session's local log. Each snapshot covers
at most 200 events and lists up to 32 files and 32 tools. It records observations,
not accomplishments or a guessed plan, and creates no pending handoff. Prompts,
transcripts, command bodies and tool outputs are not read into checkpoints;
hidden paths, sensitive filenames and private-key filenames are excluded.

Unchanged activity is saved once, even when a boundary is retried. New snapshots
supersede older snapshots from the same agent/session while retaining originals.
Hooks have a 2.5-second internal deadline and a 3-second configured timeout. They
never build, initialize a missing database, call an LLM, or push to the dashboard.
A failed checkpoint leaves the local activity log available for retry or `/remember`.
As with ordinary sessions, stored snapshots accumulate until explicitly managed.

Export portable Markdown from the target repository:

```bash
"$PLUGIN_DIR/memory/memory-db.sh" export "$HOME/Downloads/project-memory.md"
```

The destination directory must already exist and the filename must be new.
Export writes a complete file with owner-only permissions and never overwrites
an existing file or symlink. It includes sessions, knowledge, facts, relationships,
handoffs and checkpoint provenance with stable database IDs; superseded entries
are labeled. Embeddings and activity logs are omitted. The export is an explicit,
read-only snapshot for inspection and sharing, not a replacement for the SQLite
backup or a format that can currently be imported. Stored memory content is
preserved literally rather than interpreted as Markdown instructions.

After updating, rebuild the binary and run `memory-db.sh init` in each target
checkout. Initialization adds the handoff and checkpoint tables without replacing legacy data.
Rerun hook setup with `--memory` to register the new lifecycle handlers.
Existing command syntax and database paths are retained. Old binaries do not
honor supersession; do not use their destructive `consolidate` command. History
deleted by older releases cannot be recovered without a backup.

### Knowledge Graph

Visualize entity relationships in your memory database:

```bash
# Full dashboard (stats + graph + timeline)
/knowledge-graph

# Individual views
/knowledge-graph stats           # Counts, categories, top entities bar chart
/knowledge-graph graph           # Entity tree + relation edges + co-occurrences
/knowledge-graph timeline        # Chronological session/knowledge entries
/knowledge-graph explore auth    # Drill into a specific entity
```

Uses Rich for colored TUI output, with automatic plain-text fallback. See [docs/knowledge-graph-setup.md](docs/knowledge-graph-setup.md) for detailed setup.

### Manual project search

```bash
python3 "$PLUGIN_DIR/scripts/turbo.py" search "your query"
```

For semantic search, first create embeddings with `index --embed`, then use the QMD MCP tools registered by the project adapter. Bare `qmd search` targets QMD's default index rather than this checkout's isolated index.

### Optional: Auto-Context Hooks

Enable automatic context injection that searches QMD before each prompt:

```bash
# Simple mode - suggests relevant file paths (lightweight)
bash "$PLUGIN_DIR/scripts/setup-hooks.sh" --client claude

# RAG mode - injects actual content snippets (optional)
bash "$PLUGIN_DIR/scripts/setup-hooks.sh" --client claude --rag

# Remove hooks
bash "$PLUGIN_DIR/scripts/setup-hooks.sh" --client claude --remove
```

#### Hook Modes Comparison

| Mode | Token Cost | How It Works |
|------|------------|--------------|
| Simple | ~50-100/prompt | Suggests file paths, Claude decides what to read |
| RAG | ~500-2000/prompt | Injects content snippets, Claude often needs no file reads |

RAG mode is optional. Measure its latency and usefulness on your own projects.

#### How RAG Mode Works

```
1. You submit: "How does authentication work?"
2. Hook extracts: "authentication work"
3. QMD searches indexed docs
4. Hook injects relevant snippets into context
5. The agent uses the context to locate relevant source files
6. The agent verifies current source before edits or precise claims
```

## Dependencies

| Tool | Purpose |
|------|---------|
| [ripgrep](https://github.com/BurntSushi/ripgrep) | Fast file search |
| [fzf](https://github.com/junegunn/fzf) | Fuzzy finder |
| [jq](https://github.com/stedolan/jq) | JSON parsing |
| Python 3.9+ | Portable hooks, indexing and diagnostics (standard library only) |
| Go 1.22+ | Build the memory executable during setup |
| SQLite with FTS5 | Persistent memory search |
| [bun](https://bun.sh) | QMD package installation |
| [qmd](https://github.com/tobi/qmd) | Semantic search engine |

The dependency installer checks requirements and asks before using your package manager. Run `doctor` to check the memory binary and SQLite FTS5 capability.

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

Savings depend on the task; these examples are illustrative, not benchmark results.

## Sync to a personal web dashboard

Run a single-user dashboard on Fly.io and have every `/remember` invocation auto-push your `memory.db` to it. You then get a browser view of every repo you work in: recent sessions, knowledge areas, facts, and an interactive D3 force-directed knowledge graph.

[![Deploy on Fly.io](https://fly.io/static/images/launch/deploy-on-fly.svg)](https://fly.io/launch?repo=https://github.com/iagocavalcante/claude-turbo-search)

The button takes you to Fly's launch flow. Because the deploy artifact lives at `web/`, the launch command is:

```bash
git clone https://github.com/iagocavalcante/claude-turbo-search.git
cd claude-turbo-search/web
fly launch --copy-config --no-deploy
fly secrets set TURBO_TOKEN=$(openssl rand -hex 32)
fly deploy
```

Then point the CLI at it once:

```bash
~/claude-turbo-search/memory/memory-db.sh config set \
    --remote https://<your-app>.fly.dev \
    --token <your-token>
```

After this, `/remember` will silently push to your dashboard whenever it runs.

When you clone the same repo on a new machine, run `memory/memory-db.sh pull` once to download the accumulated memory — slug is derived from the git origin URL, so no per-repo config is needed. The full architecture, deferred work, and operating notes live in [`docs/plans/web-sync.md`](docs/plans/web-sync.md) and [`web/README.md`](web/README.md).

## Configuration

After running `/turbo-index`, these files are modified:

- `~/.claude/settings.json` - optional fileSuggestion and hook settings
- Client-managed MCP configuration - registered through `claude mcp add` / `codex mcp add`
- `~/.codex/hooks.json` - optional Codex hooks
- `~/.claude/file-suggestion.sh` - turbo file suggestion script
- `.claude-memory/index.json` - document fingerprint and collection metadata
- `.claude-memory/documents/` - filtered document snapshot

**Note:** The setup scripts will warn you if existing configuration will be overwritten and create backups automatically.

## MCP Tools

Use `/mcp` in the client to check the connection and discover the installed QMD tool names and schemas. Names depend on the QMD/client version. The adapter launches QMD in the current repository's dedicated index. Start the client inside that repository; indexing remains an explicit `/turbo-index` step.

## Troubleshooting

### Dependencies not installing

If automatic installation fails, you can install dependencies manually:

```bash
# macOS
brew install ripgrep fzf jq python go sqlite
brew tap oven-sh/bun && brew install bun
bun install -g @tobilu/qmd@2.8.3

# Ubuntu/Debian
sudo apt-get install ripgrep fzf jq python3 golang-go sqlite3
curl -fsSL https://bun.sh/install | bash
bun install -g @tobilu/qmd@2.8.3

# Fedora
sudo dnf install ripgrep fzf jq python3 golang sqlite
curl -fsSL https://bun.sh/install | bash
bun install -g @tobilu/qmd@2.8.3
```

### QMD models downloading

Embedding and semantic-query commands may download large models on first use. Plain keyword search and local fallback do not require those models.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute.

## License

MIT - see [LICENSE](LICENSE) for details.
