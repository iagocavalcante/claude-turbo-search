# claude-turbo-search-web

A self-hosted dashboard that aggregates the per-repo `memory.db` from every project where you use [claude-turbo-search](../). One Fly.io deployment per developer, no shared state, no signups.

## What it gives you

- A single URL where you can browse all your synced repos.
- Per-repo views: recent sessions, knowledge areas, facts.
- An interactive D3 force-directed knowledge graph.
- A JSON API (`/api/repos`, `/api/repos/{slug}`, `/api/repos/{slug}/graph`) for whatever you want to wire into your own tools.

## Architecture in one paragraph

The Go binary in `cmd/server` listens on `:8080`, stores per-repo SQLite files at `/data/repos/<slug>.db`, and accepts gzipped pushes at `POST /api/repos/{slug}/push` with a `Bearer <TURBO_TOKEN>` header. HTML pages use HTTP Basic auth (browser-prompted). The slug is the SHA-256 of the local repo's `git remote get-url origin` truncated to 12 hex chars — automatic, stable, no per-repo config.

## Deploy to Fly.io

You need [`flyctl`](https://fly.io/docs/flyctl/install/) installed. The free tier covers everything (256MB shared VM, 3GB volume).

```bash
git clone https://github.com/iagocavalcante/claude-turbo-search.git
cd claude-turbo-search/web

# Create the app and provision the volume.
fly launch --copy-config --no-deploy
# (Accept the suggested name or pick your own; pick a region.)

# Set the bearer token. Treat it like a password — your dashboard URL is public.
fly secrets set TURBO_TOKEN=$(openssl rand -hex 32)

# Deploy.
fly deploy
```

Take note of:
- The app URL Fly prints (`https://<your-app>.fly.dev`).
- The token you just set — you'll need it once for `memorydb config`.

## Point the CLI at your deployment

From any repo where you use `/remember`:

```bash
~/claude-turbo-search/memory/memory-db.sh config set \
    --remote https://<your-app>.fly.dev \
    --token <your-token>

# Test it:
~/claude-turbo-search/memory/memory-db.sh push
```

After this, every `/remember` invocation auto-pushes the updated `memory.db` to your dashboard. No further config per repo. Each push also sends an auto-derived `X-Repo-Name` (`org/repo` from your git origin), so the dashboard shows readable names instead of raw slugs from day one.

### Custom name

If `org/repo` isn't the label you want:

- **Once via CLI**: `memory-db.sh config set --name "My Cool App"` — the CLI then sends this with every push and the server treats it as a manual rename (sticky).
- **From the browser**: click `rename` next to the heading on `/repos/<slug>` and type a new name. Same effect — manual rename, sticks against future auto pushes.

## Pull on a fresh clone

When you clone a repo on a new machine and want the accumulated memory back for RAG/context:

```bash
# In the freshly cloned repo:
~/claude-turbo-search/memory/memory-db.sh pull
# Then immediately use it:
~/claude-turbo-search/memory/memory-db.sh search "auth flow"
~/claude-turbo-search/memory/memory-db.sh context "JWT refresh"
```

The slug is computed from `git remote get-url origin`, so you don't need any per-repo config — the same GitHub URL → same slug on any machine. If a local `.claude-memory/memory.db` already exists, `pull` refuses to overwrite without `--force` (no merge across machines in v1).

## How auth works

- **`POST /api/repos/{slug}/push`**: Bearer token only. The CLI uses this.
- **`GET /api/repos/...`**: Bearer or HTTP Basic. Use Bearer from scripts, Basic from browsers.
- **`GET /` and `GET /repos/...`**: HTTP Basic only (browser-prompted). Username can be anything; password is the token.
- **`GET /health`**: open, returns `200 OK` for Fly's checks.

## What's not in v1

- **Multi-user / team sharing.** Each Fly app is a single-user dashboard. If you want a team view, deploy one app per person.
- **Bidirectional sync.** Local is always source of truth; the web is a read-only mirror. You can't edit memory from the browser.
- **Conflict resolution across machines.** If you push from a laptop and a desktop with the same repo, the second push overwrites the first.

## Operating notes

- **Resetting state**: `fly volumes destroy turbo_data && fly deploy` (with confirmation prompt). Or just delete a single repo: `fly ssh console -C "rm /data/repos/<slug>.db"`.
- **Backup**: the volume isn't replicated. Local `memory.db` files are the source of truth, so the only thing you'd lose is dashboard latency until you re-push.
- **Logs**: `fly logs`. Auth failures show as `401 unauthorized`; pushes log size and slug on success.

## Local development

```bash
TURBO_TOKEN=dev DATA_DIR=/tmp/turbo-data PORT=8080 go run ./cmd/server
```

Then point the CLI at `http://127.0.0.1:8080` with the matching token.

## See also

- [`docs/plans/web-sync.md`](../docs/plans/web-sync.md) — design decisions and deferred work
- [`skills/remember/SKILL.md`](../skills/remember/SKILL.md) — auto-push hook
