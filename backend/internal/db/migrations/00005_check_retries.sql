-- +goose Up
-- Retry-aware checks: one row still means one scheduled cycle, but a cycle may
-- now take several attempts. A cycle that only succeeded on a retry is recorded
-- as 'warning' — the service answered, so it counts as uptime and opens no
-- incident. SQLite cannot ALTER a CHECK or a NOT NULL, so the two affected
-- tables are rebuilt; indexes follow their table through RENAME, hence the
-- create-new / copy / drop-old / rename / recreate-indexes order.

CREATE TABLE checks_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id  TEXT NOT NULL,
    ts          INTEGER NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('online', 'offline', 'warning', 'unknown')),
    latency_ms  INTEGER,
    status_code INTEGER,
    error       TEXT,
    -- Attempts made in this cycle; 1 = decided on the first try.
    attempts    INTEGER NOT NULL DEFAULT 1
);
INSERT INTO checks_new (id, service_id, ts, status, latency_ms, status_code, error, attempts)
SELECT id, service_id, ts, status, latency_ms, status_code, error, 1 FROM checks;
DROP TABLE checks;
ALTER TABLE checks_new RENAME TO checks;
CREATE INDEX idx_checks_service_ts ON checks (service_id, ts);
-- Keyset pagination for the per-service ping console (newest-first by id).
CREATE INDEX idx_checks_service_id ON checks (service_id, id);

-- A warning notification has no incident to point at, so incident_id becomes
-- nullable and 'warning' joins the event enum.
CREATE TABLE notification_log_new (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    integration_id INTEGER NOT NULL REFERENCES integrations (id) ON DELETE CASCADE,
    incident_id    INTEGER REFERENCES incidents (id) ON DELETE CASCADE,
    event          TEXT NOT NULL CHECK (event IN ('incident_start', 'incident_resolve', 'warning')),
    status         TEXT NOT NULL CHECK (status IN ('sent', 'failed')),
    error          TEXT,
    sent_at        INTEGER NOT NULL
);
INSERT INTO notification_log_new (id, integration_id, incident_id, event, status, error, sent_at)
SELECT id, integration_id, incident_id, event, status, error, sent_at FROM notification_log;
DROP TABLE notification_log;
ALTER TABLE notification_log_new RENAME TO notification_log;
CREATE INDEX idx_notification_log_integration ON notification_log (integration_id, sent_at);
CREATE INDEX idx_notification_log_incident ON notification_log (incident_id);

-- Opt-in: existing channels keep their current incident-only behaviour.
ALTER TABLE integrations ADD COLUMN notify_warnings INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Lossy by necessity: 'warning' rows violate the old CHECK, so fold them into
-- 'online' (they were successful cycles) and drop the attempt counts.
CREATE TABLE checks_old (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id  TEXT NOT NULL,
    ts          INTEGER NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('online', 'offline', 'unknown')),
    latency_ms  INTEGER,
    status_code INTEGER,
    error       TEXT
);
INSERT INTO checks_old (id, service_id, ts, status, latency_ms, status_code, error)
SELECT id, service_id, ts,
       CASE WHEN status = 'warning' THEN 'online' ELSE status END,
       latency_ms, status_code, error
FROM checks;
DROP TABLE checks;
ALTER TABLE checks_old RENAME TO checks;
CREATE INDEX idx_checks_service_ts ON checks (service_id, ts);

-- Warning log rows have no incident and no valid event under the old schema.
DELETE FROM notification_log WHERE event = 'warning' OR incident_id IS NULL;
CREATE TABLE notification_log_old (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    integration_id INTEGER NOT NULL REFERENCES integrations (id) ON DELETE CASCADE,
    incident_id    INTEGER NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    event          TEXT NOT NULL CHECK (event IN ('incident_start', 'incident_resolve')),
    status         TEXT NOT NULL CHECK (status IN ('sent', 'failed')),
    error          TEXT,
    sent_at        INTEGER NOT NULL
);
INSERT INTO notification_log_old (id, integration_id, incident_id, event, status, error, sent_at)
SELECT id, integration_id, incident_id, event, status, error, sent_at FROM notification_log;
DROP TABLE notification_log;
ALTER TABLE notification_log_old RENAME TO notification_log;
CREATE INDEX idx_notification_log_integration ON notification_log (integration_id, sent_at);
CREATE INDEX idx_notification_log_incident ON notification_log (incident_id);

ALTER TABLE integrations DROP COLUMN notify_warnings;
