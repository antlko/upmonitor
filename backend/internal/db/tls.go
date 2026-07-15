package db

import (
	"database/sql"
	"errors"
)

// ServiceTLS is the current TLS-certificate snapshot for a service. Zero
// ValidFrom/ValidUntil with a non-empty Error means the last HTTPS check could
// not complete a handshake (e.g. an expired or invalid certificate).
type ServiceTLS struct {
	ServiceID  string
	CheckedAt  int64
	ValidFrom  int64
	ValidUntil int64
	Issuer     string
	Subject    string
	Error      string
}

// UpsertServiceTLS records (or replaces) the current cert snapshot for a service.
func (db *DB) UpsertServiceTLS(serviceID string, checkedAt, validFrom, validUntil int64, issuer, subject, errMsg string) error {
	_, err := db.Exec(`
		INSERT INTO service_tls (service_id, checked_at, valid_from, valid_until, issuer, subject, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (service_id) DO UPDATE SET
			checked_at  = excluded.checked_at,
			valid_from  = excluded.valid_from,
			valid_until = excluded.valid_until,
			issuer      = excluded.issuer,
			subject     = excluded.subject,
			error       = excluded.error`,
		serviceID, checkedAt, validFrom, validUntil, issuer, subject, errMsg,
	)
	return err
}

// GetServiceTLS returns the cert snapshot for a service, or (nil, nil) if none.
func (db *DB) GetServiceTLS(serviceID string) (*ServiceTLS, error) {
	row := db.QueryRow(`
		SELECT service_id, checked_at, valid_from, valid_until,
		       COALESCE(issuer, ''), COALESCE(subject, ''), COALESCE(error, '')
		FROM service_tls WHERE service_id = ?`, serviceID)
	var t ServiceTLS
	if err := row.Scan(&t.ServiceID, &t.CheckedAt, &t.ValidFrom, &t.ValidUntil, &t.Issuer, &t.Subject, &t.Error); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// DeleteServiceTLS removes the cert snapshot for a service.
func (db *DB) DeleteServiceTLS(serviceID string) error {
	_, err := db.Exec(`DELETE FROM service_tls WHERE service_id = ?`, serviceID)
	return err
}
