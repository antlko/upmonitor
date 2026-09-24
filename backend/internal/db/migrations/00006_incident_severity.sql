-- +goose Up
-- Incidents gain a severity so a momentary "warning" event (a cycle that only
-- recovered on a retry) can be recorded alongside real outages instead of
-- being invisible everywhere but the notification log. Existing rows are all
-- real outages, hence the default.
ALTER TABLE incidents ADD COLUMN severity TEXT NOT NULL DEFAULT 'outage'
    CHECK (severity IN ('outage', 'warning'));

-- +goose Down
ALTER TABLE incidents DROP COLUMN severity;
