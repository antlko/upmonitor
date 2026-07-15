-- +goose Up
-- One current-cert snapshot per service, upserted on each HTTPS check.
CREATE TABLE service_tls (
    service_id  TEXT PRIMARY KEY,
    checked_at  INTEGER NOT NULL,
    valid_from  INTEGER,
    valid_until INTEGER,
    issuer      TEXT,
    subject     TEXT,
    error       TEXT
);

-- +goose Down
DROP TABLE service_tls;
