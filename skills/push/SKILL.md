---
name: push
description: Manually upload the persistent memory database to your self-hosted dashboard. Useful after directly running add-fact or add-knowledge — for normal session work, /remember already auto-pushes.
---

# /push - Upload Memory To Dashboard

Send the current repo's `.claude-memory/memory.db` to your self-hosted Fly.io dashboard. Slug is derived from `git remote get-url origin`, so it lands on the right per-repo storage automatically.

## Instructions

When the user invokes `/push`, run:

```bash
PLUGIN_DIR="${PLUGIN_DIR:-$HOME/claude-turbo-search}"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"

"$MEMORY_SCRIPT" push
```

### Common outcomes

**Happy path**:
```
Pushed .../memory.db to https://<dashboard>/api/repos/<slug>/push
```
Tell the user the dashboard now reflects this push.

**No config** — message will be:
```
no sync remote configured. run `memorydb config set --remote URL --token TOKEN`
```
Tell the user how to configure it.

**No local DB** — message will be:
```
no memory database at <path> — nothing to push
```
Suggest running `/remember` first (which creates the DB if needed).

## When to use

- **After manually editing memory** — running `add-fact`, `add-knowledge`, or `consolidate` directly via `memory-db.sh` doesn't trigger an auto-push. Use `/push` to sync.
- **Forcing a sync mid-session** — when you want the dashboard fresh without waiting for `/remember`.
- **Verifying connectivity** — quick way to confirm the dashboard is reachable and your token works.

## When NOT to use

- **After `/remember`** — that skill auto-pushes already.
- **Routine workflow** — let `/remember` handle pushes at the natural session boundary.

## See also

- [`/pull`](../pull/SKILL.md) — the inverse operation (download from dashboard).
- [`/remember`](../remember/SKILL.md) — auto-pushes after saving the session.
- [`web/README.md`](../../web/README.md) — full architecture.
