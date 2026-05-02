---
name: pull
description: Download the persistent memory database from your self-hosted dashboard into the current repo. Use after cloning a repo on a new machine to restore accumulated sessions, knowledge, facts, and embeddings for RAG/context.
---

# /pull - Download Memory From Dashboard

Restore the per-repo `memory.db` from your self-hosted Fly.io dashboard. The slug is computed from `git remote get-url origin`, so the same GitHub URL → same data on any machine. No per-repo config needed beyond the global `memory-db.sh config set` you ran once.

## Instructions

When the user invokes `/pull`, run:

```bash
PLUGIN_DIR="${PLUGIN_DIR:-$HOME/claude-turbo-search}"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"

# Pass through any flags the user provided (e.g. --force).
"$MEMORY_SCRIPT" pull "$@"
```

### Common outcomes

**Happy path** (fresh clone, no local DB yet):
```
Pulled https://<dashboard>/api/repos/<slug>/db -> .claude-memory/memory.db
```
Then suggest the user run `/memory-stats` to see what was restored, and `/memory-stats search <query>` to retrieve specific context.

**No config** — message will be:
```
no sync remote configured. run `memorydb config set --remote URL --token TOKEN`
```
Tell the user how to configure it. Example:
```bash
"$MEMORY_SCRIPT" config set --remote https://your-app.fly.dev --token <token>
```

**Nothing on remote** — message will be:
```
no database for this repo on the remote — has it been pushed yet?
```
Explain that this is normal for a repo that's never been pushed to the dashboard before. Suggest creating memory locally with `/remember` (which auto-pushes when configured).

**Existing local DB without --force** — message will be:
```
local memory.db already exists at <path> (<N> bytes). pass --force to overwrite
```
Tell the user that pull refuses to clobber non-empty local memory by default. If they really want to overwrite, run `/pull --force`. Note that there's no merge across machines in v1 — last-write-wins.

## When to use

- **Cloning a repo on a new machine** — primary use case. Get accumulated context back instantly.
- **Resetting after corruption** — if `.claude-memory/memory.db` got mangled, pull from remote to recover.
- **Catching up on another machine's work** — if you `/remember`-ed something on a laptop and want it on a desktop. (Caveat: only works one-way; pulling discards any unique local memory.)

## When NOT to use

- **Day-to-day** — `/remember` auto-pushes after each session, so the dashboard stays current without manual pulls.
- **When you have unique local sessions** — pull will overwrite them. If unsure, run `/memory-stats` first to see what's local.

## See also

- [`/push`](../push/SKILL.md) — the inverse operation (manual push without a `/remember`).
- [`web/README.md`](../../web/README.md) — full architecture and operating notes.
