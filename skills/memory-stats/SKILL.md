---
name: memory-stats
description: View statistics and recent entries from the persistent memory database. Shows session count, knowledge areas, facts, and recent activity.
---

## Installed paths

Resolve `PLUGIN_DIR` to the absolute directory two levels above this `SKILL.md` file.
Use the skill's actual installed location, including plugin caches or symlinks; do not infer it from the project's working directory or a home-directory checkout.
Set `PLUGIN_DIR` explicitly in each shell invocation below, keeping the working directory in the user's target repository.


# /memory-stats - View Memory Statistics

Show statistics and recent entries from the persistent memory database.

## Instructions

When the user invokes `/memory-stats`, run these commands to show memory information:

### 1. Show Statistics

```bash
: "${PLUGIN_DIR:?Set PLUGIN_DIR from this skill file location}"
"$PLUGIN_DIR/memory/memory-db.sh" stats
```

### 2. Show Recent Sessions

```bash
"$PLUGIN_DIR/memory/memory-db.sh" recent 5
```

### 3. Format Output

Present the information clearly:

```
## Memory Database

| Type | Count |
|------|-------|
| Sessions | X |
| Knowledge areas | Y |
| Facts | Z |

## Recent Sessions

1. [date] - Summary of session 1
2. [date] - Summary of session 2
...
```

## Optional Flags

The user might ask for specific views:
- `/memory-stats facts` - Show all facts
- `/memory-stats knowledge` - Show all knowledge areas
- `/memory-stats search <query>` - Search memory

For these, use:

```bash
# Search
"$PLUGIN_DIR/memory/memory-db.sh" search "query"
```
