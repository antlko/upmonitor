package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Integration is a configured notification channel. Config is a JSON blob whose
// shape depends on Type (see internal/notify).
type Integration struct {
	ID        int64
	Type      string
	Name      string
	Enabled   bool
	Config    json.RawMessage
	CreatedAt int64
	UpdatedAt int64
}

const integrationCols = `id, type, name, enabled, config, created_at, updated_at`

func scanIntegration(row interface{ Scan(...any) error }) (*Integration, error) {
	var in Integration
	var cfg []byte
	if err := row.Scan(&in.ID, &in.Type, &in.Name, &in.Enabled, &cfg, &in.CreatedAt, &in.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	in.Config = json.RawMessage(cfg)
	return &in, nil
}

// CreateIntegration inserts a new notification channel and returns it.
func (db *DB) CreateIntegration(kind, name string, enabled bool, config json.RawMessage) (*Integration, error) {
	now := time.Now().Unix()
	res, err := db.Exec(
		`INSERT INTO integrations (type, name, enabled, config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		kind, name, enabled, []byte(config), now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetIntegration(id)
}

// UpdateIntegration replaces a channel's name, enabled flag and config.
func (db *DB) UpdateIntegration(id int64, name string, enabled bool, config json.RawMessage) (*Integration, error) {
	if _, err := db.Exec(
		`UPDATE integrations SET name = ?, enabled = ?, config = ?, updated_at = ? WHERE id = ?`,
		name, enabled, []byte(config), time.Now().Unix(), id); err != nil {
		return nil, err
	}
	return db.GetIntegration(id)
}

// DeleteIntegration removes a channel (its notification log cascades).
func (db *DB) DeleteIntegration(id int64) error {
	_, err := db.Exec(`DELETE FROM integrations WHERE id = ?`, id)
	return err
}

// GetIntegration returns a channel by id (ErrNotFound if absent).
func (db *DB) GetIntegration(id int64) (*Integration, error) {
	return scanIntegration(db.QueryRow(`SELECT `+integrationCols+` FROM integrations WHERE id = ?`, id))
}

// ListIntegrations returns all channels, oldest first.
func (db *DB) ListIntegrations() ([]Integration, error) {
	return db.queryIntegrations(`SELECT ` + integrationCols + ` FROM integrations ORDER BY created_at ASC, id ASC`)
}

// ListEnabledIntegrations returns only enabled channels.
func (db *DB) ListEnabledIntegrations() ([]Integration, error) {
	return db.queryIntegrations(`SELECT ` + integrationCols + ` FROM integrations WHERE enabled = 1 ORDER BY id ASC`)
}

func (db *DB) queryIntegrations(q string, args ...any) ([]Integration, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Integration{}
	for rows.Next() {
		in, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *in)
	}
	return out, rows.Err()
}

// LogNotification records a delivery attempt for auditing/debugging.
func (db *DB) LogNotification(integrationID, incidentID int64, event, status, errMsg string, ts int64) error {
	_, err := db.Exec(
		`INSERT INTO notification_log (integration_id, incident_id, event, status, error, sent_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		integrationID, incidentID, event, status, nullStr(errMsg), ts)
	return err
}
