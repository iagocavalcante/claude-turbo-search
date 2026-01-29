-- Claude Code Memory Schema
-- Per-repo persistent memory for context-aware assistance

-- Session summaries - what was worked on
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    summary TEXT NOT NULL,              -- 1-3 sentence summary of work done
    files_touched TEXT,                 -- JSON array of file paths
    tools_used TEXT,                    -- JSON array of tools used
    topics TEXT                         -- comma-separated keywords for search
);

-- Code area knowledge - accumulated understanding of codebase areas
CREATE TABLE IF NOT EXISTS knowledge (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    area TEXT UNIQUE NOT NULL,          -- e.g., "src/auth", "api/routes", "database"
    summary TEXT NOT NULL,              -- what this area does, key patterns
    patterns TEXT,                      -- conventions, gotchas, important notes
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Quick facts - key decisions, conventions, important notes
CREATE TABLE IF NOT EXISTS facts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fact TEXT NOT NULL,                 -- e.g., "Uses Prisma for ORM", "Auth via JWT"
    category TEXT DEFAULT 'general',    -- architecture, convention, decision, dependency
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Full-text search virtual table for fast local search
CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
    content,
    source_type,    -- 'session', 'knowledge', 'fact'
    source_id,
    tokenize='porter unicode61'
);

-- Triggers to keep FTS in sync

-- Session FTS trigger
CREATE TRIGGER IF NOT EXISTS sessions_ai AFTER INSERT ON sessions BEGIN
    INSERT INTO memory_fts(content, source_type, source_id)
    VALUES (NEW.summary || ' ' || COALESCE(NEW.topics, ''), 'session', NEW.id);
END;

CREATE TRIGGER IF NOT EXISTS sessions_ad AFTER DELETE ON sessions BEGIN
    DELETE FROM memory_fts WHERE source_type = 'session' AND source_id = OLD.id;
END;

-- Knowledge FTS trigger
CREATE TRIGGER IF NOT EXISTS knowledge_ai AFTER INSERT ON knowledge BEGIN
    INSERT INTO memory_fts(content, source_type, source_id)
    VALUES (NEW.area || ' ' || NEW.summary || ' ' || COALESCE(NEW.patterns, ''), 'knowledge', NEW.id);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_au AFTER UPDATE ON knowledge BEGIN
    DELETE FROM memory_fts WHERE source_type = 'knowledge' AND source_id = OLD.id;
    INSERT INTO memory_fts(content, source_type, source_id)
    VALUES (NEW.area || ' ' || NEW.summary || ' ' || COALESCE(NEW.patterns, ''), 'knowledge', NEW.id);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_ad AFTER DELETE ON knowledge BEGIN
    DELETE FROM memory_fts WHERE source_type = 'knowledge' AND source_id = OLD.id;
END;

-- Facts FTS trigger
CREATE TRIGGER IF NOT EXISTS facts_ai AFTER INSERT ON facts BEGIN
    INSERT INTO memory_fts(content, source_type, source_id)
    VALUES (NEW.fact || ' ' || COALESCE(NEW.category, ''), 'fact', NEW.id);
END;

CREATE TRIGGER IF NOT EXISTS facts_ad AFTER DELETE ON facts BEGIN
    DELETE FROM memory_fts WHERE source_type = 'fact' AND source_id = OLD.id;
END;

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_area ON knowledge(area);
CREATE INDEX IF NOT EXISTS idx_facts_category ON facts(category);
