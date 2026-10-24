PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at    DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id               TEXT PRIMARY KEY,
    admin_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_title      TEXT NOT NULL DEFAULT '',
    duration_seconds REAL NOT NULL DEFAULT 0,
    source_path      TEXT NOT NULL DEFAULT '',
    video_path       TEXT NOT NULL DEFAULT '',
    video_status     TEXT NOT NULL DEFAULT 'uploading',
    error            TEXT NOT NULL DEFAULT '',
    created_at       DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_admin ON sessions(admin_id);

-- Multi-use invite links with 5-minute expiration time.
CREATE TABLE IF NOT EXISTS session_invites (
    token      TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_invites_session ON session_invites(session_id);
