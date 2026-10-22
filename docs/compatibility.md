# Client compatibility and verification

Turbo Search uses one local memory CLI and shared skills. Claude Code and Codex registration are adapters; the memory database remains `.claude-memory/memory.db`. No database migration or cloud service is required.

## Verified on 2026-10-22

| Component | Version | Verification |
| --- | --- | --- |
| Claude Code | 2.1.308 | Isolated MCP registration/repeated setup and plugin validation |
| Codex CLI | 0.163.0 | Isolated MCP registration/repeated setup and local marketplace/plugin installation |
| QMD | 2.8.3 (`@tobilu/qmd`, published package) | Real indexing, scoped CLI search, MCP initialization/discovery/query and isolation between two same-named repositories |
| Legacy QMD | 1.0.0 (existing local package) | Real indexing, scoped CLI search and MCP search/isolation |
| Platform | macOS arm64 | Local regression tests and both Go test suites |

The live checks use temporary HOME/config/cache directories and synthetic documents. They do not make LLM requests or modify personal client profiles. They test the installed MCP server directly after registering it with both clients; they do not prove model tool selection or UI hook trust/dispatch. Linux/macOS CI runs the regression suite; record its actual run result before release.

Upgrade verification used the previous source at `b44e7f8`: its executable created a database with a session, knowledge and a fact. Running the new `init` and `init-metadata` twice preserved every row and passed SQLite integrity checks. Initialization also preserved a copy of this checkout's existing database. A copy of existing Claude settings migrated old hook timeouts to 10/5 seconds without changing unrelated settings, and repeated setup was idempotent.

In this checkout, rebuilding the index covered 31 Markdown files. Real QMD retrieval and both prompt-hook entry points returned bounded context in under one second each. These direct script checks do not verify interactive client hook dispatch. Updating the checkout alone does not migrate personal hook settings or register MCP: run the setup commands below for those steps.

QMD 2.8.3 exposes `query`, `get`, `multi_get`, and `status`; older QMD also exposes `search` and `vsearch`. Always discover schemas. For a model-free QMD 2.8.3 MCP query, use `searches: [{"type":"lex","query":"your term"}]` and `rerank: false`. Plain `query` may invoke local models. Installer examples pin QMD 2.8.3 so a dependency update is deliberate and testable.

## Installation and migration

From a stable checkout:

```bash
bash scripts/install-deps.sh --check-only
bash scripts/build-memory.sh
bash scripts/install-skills.sh codex  # standalone installation, or use a plugin
bash scripts/setup-mcp.sh --client both
```

For Codex plugin installation instead of standalone skills:

```bash
codex plugin marketplace add /absolute/path/to/claude-turbo-search
codex plugin add turbo-search@claude-turbo-search-dev
```

Claude's marketplace installation is documented in the README. Choose plugin installation or standalone symlinked skills, not both. The plugin name is now `turbo-search`: Claude Code 2.1.306 rejects the previous `claude-` prefix as reserved. The repository URL, marketplace name and memory storage stay unchanged. Skills resolve resources relative to their installed location, including caches.

Run `/turbo-index` inside each target checkout. QMD MCP startup requires a fresh index; after changing documentation, refresh the index and reconnect MCP. A running MCP server reads its indexed snapshot; it does not inspect source freshness on every tool call. The shared `turbo.py search` command and prompt hooks do check freshness on every search and fall back to current local Markdown matches.

Previously registered `qmd` servers are preserved. Inspect the registration before replacing it with `scripts/qmd-mcp.sh`. An existing direct `qmd mcp` registration still targets its old default index. Setup no longer writes MCP entries into Claude's `settings.json`.

Memory commands need the built executable; rebuild after updating the plugin. Build requires Go, but running the binary does not. Prompt hooks never compile or install software. Advanced embedding commands retain the existing configured provider behavior.

Optional hooks:

```bash
bash scripts/setup-hooks.sh --client both --rag --memory
bash scripts/setup-hooks.sh --client both --remove
```

The installers honor `CODEX_HOME` and `CLAUDE_CONFIG_DIR`. Hook setup preserves foreign handlers, backs up existing JSON and writes atomically. Native client hook trust still applies. Raw commands and tool output are not saved to activity logs. Session IDs are hashed into filenames; records are locked and bounded to 200 events per session. Older session files are not automatically deleted.

## Automated checks

```bash
bash tests/run_tests.sh
python3 tests/smoke_clients.py
```

The first command builds memory, runs Python regression tests and tests both Go modules. The Reliability workflow also has a manual `live-smoke` option that installs pinned clients on Linux. The opt-in second command requires installed Claude, Codex and QMD. It creates temporary profiles, tests real CLI configuration, QMD search and MCP, validates the Claude plugin and installs the Codex plugin from a clean local package.

Coverage includes:

- Malformed hook input, payload cwd, non-repositories and missing embedding providers.
- Git worktrees, nested directories and paths containing spaces.
- Missing binaries, concurrent database writes and subprocess-tree deadlines.
- Fresh/stale/missing indexes, repository identity, ignored/private filenames and symlinks.
- Concurrent session activity, Codex patch paths, command redaction and bounded context.
- Repeated MCP/hook/skill installation, foreign hooks and malformed configuration preservation.

## Release checklist

1. Record `claude --version`, `codex --version`, `qmd --version` and OS/architecture. For legacy QMD without a version flag, read its installed package manifest.
2. Run regression CI on macOS and Linux, and the isolated real-client smoke test.
3. In each client's interactive UI, approve/trust the installed hooks as required, open `/mcp`, and search a unique term in a synthetic document. Confirm the tool response comes from that checkout.
4. Exercise a prompt hook and a file edit, then read only that session's activity with `turbo.py activity-read --session ID`.
5. Change/delete the synthetic document. Confirm `turbo.py search` falls back to current local evidence; refresh `/turbo-index` and reconnect MCP.
6. Repeat from a Git worktree. Record UI results rather than assuming direct protocol checks cover them.

## Tracked work

- [#7](https://github.com/iagocavalcante/claude-turbo-search/issues/7): client MCP registration
- [#8](https://github.com/iagocavalcante/claude-turbo-search/issues/8): bounded hook execution
- [#9](https://github.com/iagocavalcante/claude-turbo-search/issues/9): worktree roots
- [#10](https://github.com/iagocavalcante/claude-turbo-search/issues/10): scoped/fresh retrieval
- [#11](https://github.com/iagocavalcante/claude-turbo-search/issues/11): built memory executable
- [#12](https://github.com/iagocavalcante/claude-turbo-search/issues/12): session activity isolation
- [#13](https://github.com/iagocavalcante/claude-turbo-search/issues/13): portable skills/plugin packaging
- [#14](https://github.com/iagocavalcante/claude-turbo-search/issues/14): trustworthy context, doctor and CI
