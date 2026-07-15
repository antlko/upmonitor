-- +goose Up
CREATE TABLE incidents (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id  TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('ongoing', 'resolved')),
    source      TEXT NOT NULL CHECK (source IN ('auto', 'manual')),
    title       TEXT,
    started_at  INTEGER NOT NULL,
    resolved_at INTEGER,
    created_by  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
CREATE INDEX idx_incidents_service ON incidents (service_id, started_at);
CREATE INDEX idx_incidents_status ON incidents (status);
-- Enforce at most one ongoing incident per service.
CREATE UNIQUE INDEX idx_incidents_one_ongoing ON incidents (service_id) WHERE status = 'ongoing';

CREATE TABLE incident_comments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id INTEGER NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    user_id     INTEGER REFERENCES users (id) ON DELETE SET NULL,
    body        TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);
CREATE INDEX idx_incident_comments_incident ON incident_comments (incident_id, created_at);

-- +goose Down
DROP INDEX idx_incident_comments_incident;
DROP TABLE incident_comments;
DROP INDEX idx_incidents_one_ongoing;
DROP INDEX idx_incidents_status;
DROP INDEX idx_incidents_service;
DROP TABLE incidents;
