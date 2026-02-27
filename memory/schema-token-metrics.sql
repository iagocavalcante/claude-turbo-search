-- Token metrics tracking - persistent per-session token savings
-- Safe to run on existing databases (IF NOT EXISTS)
CREATE TABLE IF NOT EXISTS token_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    searches_count INTEGER DEFAULT 0,
    files_read_count INTEGER DEFAULT 0,
    files_edited_count INTEGER DEFAULT 0,
    estimated_tokens_used INTEGER DEFAULT 0,      -- with plugin
    estimated_tokens_without INTEGER DEFAULT 0,   -- blind exploration estimate
    tokens_saved INTEGER DEFAULT 0,               -- difference
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
CREATE INDEX IF NOT EXISTS idx_token_metrics_session ON token_metrics(session_id);
