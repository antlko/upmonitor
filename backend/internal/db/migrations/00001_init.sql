-- +goose Up
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin', 'readonly')),
    created_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE checks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id  TEXT NOT NULL,
    ts          INTEGER NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('online', 'offline', 'unknown')),
    latency_ms  INTEGER,
    status_code INTEGER,
    error       TEXT
);

CREATE INDEX idx_checks_service_ts ON checks (service_id, ts);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

-- +goose Down
DROP INDEX idx_sessions_expires;
DROP INDEX idx_checks_service_ts;
DROP TABLE checks;
DROP TABLE sessions;
DROP TABLE users;
