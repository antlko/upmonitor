-- +goose Up
-- Incidents gain a severity so a momentary "warning" event (a cycle that only
-- recovered on a retry) can be recorded alongside real outages instead of
-- being invisible everywhere but the notification log. Existing rows are all
-- real outages, hence the default.
ALTER TABLE incidents ADD COLUMN severity TEXT NOT NULL DEFAULT 'outage'
    CHECK (severity IN ('outage', 'warning'));

-- The dashboard's configurable "Warning" tile counts warning cycles across
-- ALL services within a look-back window (WarningServiceCountSince), unlike
-- every other checks query which is scoped to one service_id — so it can't
-- use the existing (service_id, ts) indexes. It's polled every 10s per open
-- dashboard tab against a single shared DB connection (SetMaxOpenConns(1)),
-- so a full table scan there is worth avoiding from the start.
CREATE INDEX idx_checks_status_ts ON checks (status, ts);

-- +goose Down
DROP INDEX idx_checks_status_ts;
ALTER TABLE incidents DROP COLUMN severity;
