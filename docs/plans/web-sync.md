# Web Sync — Self-Hosted Memory Dashboard

**Status**: Draft / for discussion
**Date**: 2026-05-02

## Goal

Let each developer deploy a personal web dashboard (one-click on Fly.io) that aggregates the `.claude-memory/memory.db` from all their repos. Browse sessions, knowledge, facts, and the knowledge graph in a browser instead of only via TUI / CLI.

## Non-goals (v1)

- Multi-user / team sharing — each deployment is a single user.
- Editing memory from the browser — push-only, web is read-only.
- Real-time bidirectional sync — pushes are explicit or hook-triggered.
- Conflict resolution — local DB is always source of truth, remote is a snapshot mirror.

## Architecture

### Repo layout

Add `web/` at the project root — keeps the deploy artifact next to the source:

```
web/
├── Dockerfile          # multi-stage Go build
├── fly.toml            # app config, volume mount at /data
├── main.go             # HTTP server entrypoint
├── handlers/           # API + UI handlers
├── templates/          # HTMX templates (server-rendered)
└── static/             # CSS/JS assets
```

### Web service (Go + HTMX)

Single Go binary, no JS build step.

**Endpoints**:

| Method | Path                          | Purpose                                              |
| ------ | ----------------------------- | ---------------------------------------------------- |
| POST   | `/api/repos/:slug/push`       | Raw gzipped `memory.db` (`Content-Encoding: gzip`, `Content-Type: application/octet-stream`). Bearer auth.|
| GET    | `/api/repos`                  | JSON list of synced repos.                           |
| GET    | `/`                           | Repo list (HTML).                                    |
| GET    | `/repos/:slug`                | Sessions / facts / knowledge tabs.                   |
| GET    | `/repos/:slug/graph`          | Knowledge graph viewer (web port of TUI).            |
| GET    | `/repos/:slug/sessions/:id`   | Session detail.                                      |
| DELETE | `/api/repos/:slug`            | Remove a synced repo.                                |

**Storage**: one SQLite file per repo at `/data/repos/<slug>.db` on a Fly volume.
- Avoids cross-repo joins.
- Trivial deletion.
- Each request opens the relevant `*.db` read-only.

**Auth**: single `TURBO_TOKEN` env var set at deploy time. Bearer token on API; cookie session for UI (token entered once, signed cookie afterward).

### Local CLI (extend `memory/` Go binary)

```bash
memorydb config set --remote https://my-app.fly.dev --token <token>
memorydb push                  # one-shot: gzip + POST
memorydb push --watch          # daemon mode, debounced re-push on .db change
```

Config stored at `~/.config/claude-turbo-search/config.toml`.

Auto-push hook: the `/remember` skill calls `memorydb push` after writing locally, if a remote is configured. No background watcher in v1.

### One-click Fly button

In root `README.md`:

```markdown
[![Deploy on Fly.io](https://fly.io/static/images/launch/deploy-on-fly.svg)](https://fly.io/launch?repo=https://github.com/iagocavalcante/claude-turbo-search)
```

Fly's launch flow:
1. Detects `web/fly.toml`.
2. Creates the app and provisions the volume declared in `[mounts]`.
3. Prompts for the `TURBO_TOKEN` secret (or generates one and shows it once).
4. Deploys.

User then copies the URL + token into `memorydb config set`.

## Free-tier fit

- 1× shared-cpu-1x VM (256MB) — plenty for SQLite + HTMX.
- 3GB persistent volume — hundreds of repo DBs.
- Free `*.fly.dev` subdomain with TLS.

## Deploy flow (user perspective)

1. Click "Deploy on Fly.io" in the README.
2. Fly creates `https://<app>.fly.dev`, generates `TURBO_TOKEN`, shows it once.
3. Locally: `memorydb config set --remote <url> --token <token>`.
4. Locally: `memorydb push` (or wait for auto-push).
5. Browse `https://<app>.fly.dev`.

## Implementation phases

### Phase 1 — Local CLI push command (no server yet) ✅
- Add `memorydb config` subcommand (read/write JSON config at `~/.config/claude-turbo-search/config.json`, mode 0600).
- Add `memorydb push` subcommand (gzip + POST as raw body, slug derived from `git remote get-url origin`).
- Manual smoke test against a local HTTP echo server.
- Implemented in `memory/internal/sync/{config,slug,push}.go` and `memory/internal/commands/sync.go`. 17 unit tests passing.

### Phase 2 — Web service skeleton ✅
- `web/` is a separate Go module (`claude-turbo-search/web`).
- `POST /api/repos/{slug}/push` (bearer auth, gzip body, atomic write to `{DATA_DIR}/repos/{slug}.db`).
- `GET /health` for Fly.io checks.
- `Dockerfile` (multi-stage, distroless-style alpine, non-root user) + `fly.toml` (256MB shared VM, persistent volume, auto-stop).
- 13 unit tests + manual local E2E (memorydb push → server, file lands with SQLite magic).
- Repo registry index and human-readable display name deferred to Phase 3 (read UI).

### Phase 3 — Read UI ✅
- `web/internal/store/` — pure-Go SQLite reads (`modernc.org/sqlite`), no CGO required.
- `web/internal/server/views.go` + `templates/` — `GET /` (repo list) and `GET /repos/{slug}` (sessions, knowledge, facts on one page).
- `web/internal/server/api.go` — `GET /api/repos`, `GET /api/repos/{slug}` JSON endpoints.
- Auth: HTML routes require HTTP Basic auth (browser-friendly), API routes accept Bearer or Basic, push endpoint stays Bearer-only.
- Decision: server-rendered HTML with stdlib `html/template` (no HTMX in Phase 3 — defer to Phase 4 graph).
- 21 server tests + 9 store tests all passing. Local E2E confirmed: push + JSON + HTML pages all functional.

### Phase 4 — Graph viewer ✅
- `store.GraphData` builds nodes (entities + sources) and edges from `entity_metadata`. Caps at top-50 entities by ref count.
- `GET /api/repos/{slug}/graph` returns `{nodes, edges}` JSON.
- `GET /repos/{slug}/graph` renders D3 v7 force-directed layout (CDN), drag/zoom/tooltip, color-coded legend (session/knowledge/fact + file/concept/package).
- "View graph →" link wired into the repo detail page.
- 5 graph store tests + 2 graph API tests + manual E2E run all green.

### Phase 5 — Auto-push + one-click button ✅
- `memory/memory-db.sh` routes `config` and `push` to the Go binary alongside the other core commands.
- `skills/remember/SKILL.md` step 6.6 calls `"$MEMORY_SCRIPT" push` non-blocking after the session save.
- Root `README.md` has the Fly.io deploy badge and a 3-step setup snippet.
- `web/README.md` covers deploy, auth model, scope cuts, and operating notes.

## Decisions

1. **Repo slug**: SHA-256 hash of `git remote get-url origin`, truncated to 12 hex chars. Display the human-readable repo name (derived from the remote URL) in the UI; the hash is just the URL/storage key.
2. **Auto-push trigger**: every `/remember` invocation. The remember skill calls `memorydb push` after writing locally. No background watcher in v1.
3. **Graph rendering**: D3. Worth the extra code for the layout control we'll want as the graph grows.

## Open questions (deferred)

1. **Schema drift** — what happens if local schema is newer than the deployed web service expects? Versioned schema check on push, reject with a clear "upgrade your web app" error.
2. **Auth for shared deployments** — out of scope for v1, but if someone wants team sharing later: per-user tokens scoped to repo slugs.

## v1.2.0 — Pull (shipped)

- `GET /api/repos/{slug}/db` returns the gzipped SQLite file behind Bearer auth (404 on unknown slug).
- `memorydb pull` (and `memorydb pull --force`) downloads it, decodes, and writes atomically to `.claude-memory/memory.db`.
- Use case: fresh clone of a repo on a new machine — point `memorydb config` at the dashboard once, run `memorydb pull`, get the full memory back for RAG/context.
- Refuses to overwrite a non-empty local DB unless `--force` is passed (no merge logic; see scope cuts).
- Slug is deterministic from `git remote get-url origin`, so the same repo on any machine ends up at the same path on the server.
- 9 sync.Pull unit tests + 4 server-side endpoint tests + production round-trip (push → fresh clone → pull) verified byte-equal SHA-256.

## Risks / things to watch

- **Push size** — `memory.db` could grow large for active repos. Need gzip + size limit; consider incremental sync (WAL shipping) if files exceed ~50MB.
- **Schema coupling** — web service must know the SQLite schema. Either vendor it from `memory/internal/db/` or expose a thin metadata API. Vendoring is simpler.
- **Concurrent push from multiple machines** — same dev pushing from laptop + desktop overwrites. Solve with `If-Match` ETag (last-known-mtime) + 409 conflict response. Out of scope for v1 but easy to retrofit.
