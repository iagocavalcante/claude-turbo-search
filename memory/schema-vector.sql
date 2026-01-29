-- Vector Search Schema Extension for Claude Code Memory
-- Requires sqlite-vector extension to be loaded
--
-- Usage:
--   SELECT load_extension('./vector');
--   .read schema-vector.sql

-- Add embedding columns to existing tables
ALTER TABLE sessions ADD COLUMN embedding BLOB;
ALTER TABLE knowledge ADD COLUMN embedding BLOB;
ALTER TABLE facts ADD COLUMN embedding BLOB;

-- Vector metadata table - tracks embedding state
CREATE TABLE IF NOT EXISTS vector_meta (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Store embedding configuration
INSERT OR REPLACE INTO vector_meta (key, value) VALUES
    ('provider', 'ollama'),
    ('model', 'bge-small-en'),
    ('dimension', '384'),
    ('version', '1');

-- Create indexes for vector columns (optional, improves search on large datasets)
-- Note: sqlite-vector can work without indexes for small datasets

-- Embedding queue - tracks items needing embeddings
CREATE TABLE IF NOT EXISTS embedding_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,  -- 'session', 'knowledge', 'fact'
    source_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    status TEXT DEFAULT 'pending',  -- 'pending', 'processing', 'done', 'error'
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP,
    UNIQUE(source_type, source_id)
);

-- Triggers to queue new items for embedding

CREATE TRIGGER IF NOT EXISTS sessions_embed_queue AFTER INSERT ON sessions
WHEN NEW.embedding IS NULL
BEGIN
    INSERT OR REPLACE INTO embedding_queue (source_type, source_id, content, status)
    VALUES ('session', NEW.id, NEW.summary || ' ' || COALESCE(NEW.topics, ''), 'pending');
END;

CREATE TRIGGER IF NOT EXISTS knowledge_embed_queue AFTER INSERT ON knowledge
WHEN NEW.embedding IS NULL
BEGIN
    INSERT OR REPLACE INTO embedding_queue (source_type, source_id, content, status)
    VALUES ('knowledge', NEW.id, NEW.area || ' ' || NEW.summary || ' ' || COALESCE(NEW.patterns, ''), 'pending');
END;

CREATE TRIGGER IF NOT EXISTS knowledge_embed_queue_update AFTER UPDATE ON knowledge
WHEN NEW.embedding IS NULL
BEGIN
    INSERT OR REPLACE INTO embedding_queue (source_type, source_id, content, status)
    VALUES ('knowledge', NEW.id, NEW.area || ' ' || NEW.summary || ' ' || COALESCE(NEW.patterns, ''), 'pending');
END;

CREATE TRIGGER IF NOT EXISTS facts_embed_queue AFTER INSERT ON facts
WHEN NEW.embedding IS NULL
BEGIN
    INSERT OR REPLACE INTO embedding_queue (source_type, source_id, content, status)
    VALUES ('fact', NEW.id, NEW.fact || ' ' || COALESCE(NEW.category, ''), 'pending');
END;

-- Index for queue processing
CREATE INDEX IF NOT EXISTS idx_embed_queue_status ON embedding_queue(status, created_at);
