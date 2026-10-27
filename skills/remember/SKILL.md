---
name: remember
description: Save the current work session to persistent memory for future context. Summarizes accomplishments, tracks files modified, and stores learnings for cross-session continuity.
---

## Installed paths

Resolve `PLUGIN_DIR` to the absolute directory two levels above this `SKILL.md` file.
Use the skill's actual installed location, including plugin caches or symlinks; do not infer it from the project's working directory or a home-directory checkout.
Set `PLUGIN_DIR` explicitly in each shell invocation below, keeping the working directory in the user's target repository.


# /remember - Save Session to Memory

Summarize the current work session and save it to persistent memory for future context.

## Instructions

When the user invokes `/remember`, you should:

### 1. Check session activity

If the current runtime exposes its session ID, use it explicitly:

```bash
python3 "$PLUGIN_DIR/scripts/turbo.py" activity-read --session "$SESSION_ID"
```

If the ID is unavailable, summarize the current conversation. Do not guess an ID, read another session's activity, or combine logs. Legacy `.claude-memory/activity.log` remains untouched and must not be treated as current-session evidence.

Automatic lifecycle checkpoints, when enabled, contain only bounded file/tool observations. They do not establish completed work, failed approaches, or next steps; use this conversation to write those conclusions.

### 2. Analyze the Session

Review what was accomplished in this conversation:
- What files were read, created, or modified?
- What was the main goal or task?
- What key decisions were made?
- Were there any important patterns or learnings?

### 2.5. Compression Guidelines

When writing summaries, facts, or knowledge entries, follow these rules to maximize information density:

- **Resolve pronouns to concrete entities**: Write "JWT middleware validates tokens" not "it validates them"
- **Use absolute file paths**: Write "src/auth/middleware.ts" not "this file" or "the auth file"
- **Write dense, self-contained summaries**: Each memory entry should be understandable without the original conversation context
- **Avoid filler phrases**: Do not use "I think", "basically", "sort of", "actually" — state facts directly
- **Use ISO dates**: Write "2026-02-11" not "today" or "yesterday" — temporal references decay quickly

The system automatically compresses text at write time (normalizing dates, stripping filler), but writing clean summaries upfront produces better results.

### 3. Generate Summary

Create a concise summary (2-3 sentences) that captures:
- **What** was done (the main accomplishment)
- **Why** it matters (the purpose or problem solved)
- **Key details** (important files, patterns, or decisions)

### 4. Extract Metadata

Identify:
- **Files touched**: List of file paths worked on
- **Topics**: 3-5 keywords for searchability (e.g., "auth, jwt, middleware, security")
- **Tools used**: Main tools used (Read, Write, Edit, Bash, etc.)

### 5. Save to Memory

Use the memory-db.sh script to save:

```bash
: "${PLUGIN_DIR:?Set PLUGIN_DIR from this skill file location}"
MEMORY_SCRIPT="$PLUGIN_DIR/memory/memory-db.sh"

# Initialize if needed
"$MEMORY_SCRIPT" init

# Save session
SAVED_SESSION=$("$MEMORY_SCRIPT" add-session \
    "YOUR_SUMMARY_HERE" \
    '["file1.ts", "file2.ts"]' \
    '["Read", "Edit", "Bash"]' \
    "topic1, topic2, topic3")
printf '%s\n' "$SAVED_SESSION"
MEMORY_SESSION_ID=$(printf '%s\n' "$SAVED_SESSION" | sed -n 's/^Session ID: //p')
```

### 6. Optionally Add Knowledge or Facts

If during the session you learned something important about the codebase that should be remembered:

**For code area knowledge:**
```bash
"$MEMORY_SCRIPT" add-knowledge \
    "src/auth" \
    "Authentication module using JWT tokens with refresh token rotation" \
    "Tokens expire in 15min, refresh tokens in 7 days"
```

**For project facts:**
```bash
"$MEMORY_SCRIPT" add-fact "Uses PostgreSQL with Prisma ORM" "architecture"
"$MEMORY_SCRIPT" add-fact "All API routes require authentication except /health" "convention"
```

### 6.4. Save an unfinished-work handoff

When work remains, save a structured checkpoint for another Claude/Codex session.
Use `handoff save`, which reads one JSON object from stdin. Serialize the actual
values as JSON; do not interpolate conversation text into executable shell code.
The payload has these fields:

```json
{
  "source_agent": "codex",
  "source_session": "ACTUAL_RUNTIME_SESSION_ID",
  "objective": "Complete the authentication migration",
  "completed": ["Added authorization code flow with PKCE"],
  "failed_approaches": ["Cookie-only authentication did not support mobile callbacks"],
  "open_questions": ["Does the mobile callback need a separate allowlist?"],
  "next_steps": ["Verify the callback tests"]
}
```

Send the serialized payload to `"$MEMORY_SCRIPT" handoff save`. Identify the
actual runtime agent. If the runtime session ID is unavailable, use
`memory:<MEMORY_SESSION_ID>` with the ID returned by this save, never an ID
looked up from another session. Skip handoff saving if neither ID is available.
Use only this conversation's evidence, omit secrets and raw tool output, and
leave unknown fields as empty lists instead of guessing. The CLI records the
checkout path, branch and commit itself. Payloads are limited to 16 KiB.

Saving again for the same source agent/session refreshes its open handoff.
Accepted handoffs cannot be overwritten. Report failures without undoing the
session summary. For completed work, do not create a pending handoff.
Mention the saved handoff ID and `/resume-memory` in the confirmation.

### 6.5. Token estimates

If the current runtime session ID is known and the save returned a numeric `MEMORY_SESSION_ID`, persist estimates from the observed events. Never find the saved ID with `recent 1` or `MAX(id)`; another session may have written concurrently.

```bash
ACTIVITY=$(python3 "$PLUGIN_DIR/scripts/turbo.py" activity-read --session "$SESSION_ID")
READS=$(printf '%s' "$ACTIVITY" | jq '[.[] | select(.tool == "Read")] | length')
EDITS=$(printf '%s' "$ACTIVITY" | jq '[.[] | select(.tool == "Edit" or .tool == "Write" or .tool == "apply_patch")] | length')
SEARCHES=$(printf '%s' "$ACTIVITY" | jq '[.[] | select(.tool | test("mcp__.*qmd.*__(search|vsearch|query)$"))] | length')
"$MEMORY_SCRIPT" add-token-metrics "$MEMORY_SESSION_ID" "$SEARCHES" "$READS" "$EDITS"
```

Skip this step when either ID is unavailable or there are no observed events. These are estimates from a bounded activity window, not measured token usage. Report metric-save failures without undoing the saved memory. Historical metrics remain available via `/token-stats`.

### 6.6. Push to Remote Dashboard (Optional)

If the user has configured a personal web dashboard with `memory-db.sh config set`, sync the updated database to it:

```bash
"$MEMORY_SCRIPT" push 2>/dev/null || true
```

This step is non-blocking — failures are silent. When no remote is configured, `push` exits with a clear error that we discard. When a remote *is* configured, the gzipped database is uploaded so the dashboard reflects this session immediately.

See [`docs/plans/web-sync.md`](../../docs/plans/web-sync.md) for the dashboard architecture and `web/README.md` for deploy instructions.

### 7. Retain session activity

Do not delete shared or active logs. Each session log is bounded to its last 200 events under `.claude-memory/activity/`. Retention avoids racing concurrent hook writes.

### 8. Confirm to User

Report what was saved:

```
Saved to memory:
- Summary: [your summary]
- Topics: [topics]
- Files: [count] files tracked

Memory now contains [X] sessions. Use /memory-stats to see details.
```

## Example Output

```
Saved to memory:
- Summary: Implemented JWT authentication with refresh token rotation. Added middleware for protected routes and updated user model with token fields.
- Topics: auth, jwt, middleware, tokens, security
- Files: 4 files tracked

Memory now contains 12 sessions.
```

## Additional Commands

If the user wants to add specific knowledge or facts, they can say:
- "Remember that this project uses X" -> Add as fact
- "Remember how the auth module works" -> Add as knowledge

Ask clarifying questions if the summary scope is unclear.
