-- +goose Up
CREATE TABLE integrations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    type       TEXT NOT NULL CHECK (type IN ('telegram', 'slack', 'email', 'webhook')),
    name       TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    config     TEXT NOT NULL, -- JSON; shape depends on type (see internal/notify)
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE notification_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    integration_id INTEGER NOT NULL REFERENCES integrations (id) ON DELETE CASCADE,
    incident_id    INTEGER NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    event          TEXT NOT NULL CHECK (event IN ('incident_start', 'incident_resolve')),
    status         TEXT NOT NULL CHECK (status IN ('sent', 'failed')),
    error          TEXT,
    sent_at        INTEGER NOT NULL
);
CREATE INDEX idx_notification_log_integration ON notification_log (integration_id, sent_at);
CREATE INDEX idx_notification_log_incident ON notification_log (incident_id);

-- +goose Down
DROP INDEX idx_notification_log_incident;
DROP INDEX idx_notification_log_integration;
DROP TABLE notification_log;
DROP TABLE integrations;
