CREATE TABLE IF NOT EXISTS checkpoints (
    session_id INTEGER PRIMARY KEY REFERENCES sessions(id),
    source_agent TEXT NOT NULL,
    source_session TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    capture_event TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_agent, source_session, fingerprint)
);
