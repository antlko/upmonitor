package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrOngoingExists is returned when creating an incident for a service that
// already has an ongoing one (enforced by a partial unique index).
var ErrOngoingExists = errors.New("an ongoing incident already exists for this service")

// Severity values for Incident.Severity.
const (
	SeverityOutage  = "outage"
	SeverityWarning = "warning"
)

// Incident is an outage (or warning) record for a service. ResolvedAt is nil
// while ongoing — which never happens for a SeverityWarning row: those are
// always recorded already resolved, since a warning is a momentary event, not
// something with a start and an end to track.
type Incident struct {
	ID        int64
	ServiceID string
	Status    string // "ongoing" | "resolved"
	Source    string // "auto" | "manual"
	// Severity distinguishes a real outage (subject to the one-ongoing-per-
	// service invariant) from a warning event (always resolved, no invariant).
	Severity   string
	Title      string
	StartedAt  int64
	ResolvedAt *int64
	CreatedBy  *int64
	CreatedAt  int64
	UpdatedAt  int64
}

// IncidentComment is a note left on an incident. Username is resolved for display.
type IncidentComment struct {
	ID         int64
	IncidentID int64
	UserID     *int64
	Username   string
	Body       string
	CreatedAt  int64
}

const incidentCols = `id, service_id, status, source, severity, COALESCE(title, ''), started_at, resolved_at, created_by, created_at, updated_at`

func scanIncident(row interface{ Scan(...any) error }) (*Incident, error) {
	var inc Incident
	var resolved, createdBy sql.NullInt64
	if err := row.Scan(&inc.ID, &inc.ServiceID, &inc.Status, &inc.Source, &inc.Severity, &inc.Title,
		&inc.StartedAt, &resolved, &createdBy, &inc.CreatedAt, &inc.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if resolved.Valid {
		inc.ResolvedAt = &resolved.Int64
	}
	if createdBy.Valid {
		inc.CreatedBy = &createdBy.Int64
	}
	return &inc, nil
}

// GetIncident returns an incident by id (ErrNotFound if absent).
func (db *DB) GetIncident(id int64) (*Incident, error) {
	return scanIncident(db.QueryRow(`SELECT `+incidentCols+` FROM incidents WHERE id = ?`, id))
}

// GetOngoingIncident returns the ongoing incident for a service, or (nil, nil).
func (db *DB) GetOngoingIncident(serviceID string) (*Incident, error) {
	inc, err := scanIncident(db.QueryRow(
		`SELECT `+incidentCols+` FROM incidents WHERE service_id = ? AND status = 'ongoing'`, serviceID))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return inc, err
}

// CreateIncident opens a new incident. Returns ErrOngoingExists if one is already
// ongoing for the service.
func (db *DB) CreateIncident(serviceID, source string, startedAt int64, title *string, createdBy *int64) (*Incident, error) {
	now := time.Now().Unix()
	res, err := db.Exec(
		`INSERT INTO incidents (service_id, status, source, title, started_at, resolved_at, created_by, created_at, updated_at)
		 VALUES (?, 'ongoing', ?, ?, ?, NULL, ?, ?, ?)`,
		serviceID, source, title, startedAt, createdBy, now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrOngoingExists
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetIncident(id)
}

// CreateWarningEvent records a momentary warning (a cycle that recovered on a
// retry) as an already-resolved, severity='warning' incident row, so it shows
// up alongside real outages in the incident feed. Unlike CreateIncident it
// never collides with the one-ongoing-per-service index: status is 'resolved'
// from the start.
func (db *DB) CreateWarningEvent(serviceID string, ts int64) (*Incident, error) {
	now := time.Now().Unix()
	res, err := db.Exec(
		`INSERT INTO incidents (service_id, status, source, severity, title, started_at, resolved_at, created_by, created_at, updated_at)
		 VALUES (?, 'resolved', 'auto', 'warning', NULL, ?, ?, NULL, ?, ?)`,
		serviceID, ts, ts, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetIncident(id)
}

// ResolveIncident marks an incident resolved at the given time.
func (db *DB) ResolveIncident(id, resolvedAt int64) (*Incident, error) {
	if _, err := db.Exec(
		`UPDATE incidents SET status = 'resolved', resolved_at = ?, updated_at = ? WHERE id = ?`,
		resolvedAt, time.Now().Unix(), id); err != nil {
		return nil, err
	}
	return db.GetIncident(id)
}

// ResolveOngoingIncident resolves a service's ongoing incident, if any. Returns
// (nil, nil) when there was nothing to resolve.
func (db *DB) ResolveOngoingIncident(serviceID string, resolvedAt int64) (*Incident, error) {
	inc, err := db.GetOngoingIncident(serviceID)
	if err != nil || inc == nil {
		return nil, err
	}
	return db.ResolveIncident(inc.ID, resolvedAt)
}

// UpdateIncident replaces an incident's mutable fields. A nil resolvedAt keeps
// it ongoing; a non-nil resolvedAt marks it resolved. Returns ErrOngoingExists
// if reopening would create a second ongoing incident for the service.
func (db *DB) UpdateIncident(id int64, title string, startedAt int64, resolvedAt *int64) (*Incident, error) {
	status := "ongoing"
	if resolvedAt != nil {
		status = "resolved"
	}
	_, err := db.Exec(
		`UPDATE incidents SET title = ?, started_at = ?, resolved_at = ?, status = ?, updated_at = ? WHERE id = ?`,
		nullStr(title), startedAt, resolvedAt, status, time.Now().Unix(), id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrOngoingExists
		}
		return nil, err
	}
	return db.GetIncident(id)
}

// DeleteIncident removes an incident (its comments cascade).
func (db *DB) DeleteIncident(id int64) error {
	_, err := db.Exec(`DELETE FROM incidents WHERE id = ?`, id)
	return err
}

// DeleteServiceIncidents removes all incidents for a service (comments cascade).
func (db *DB) DeleteServiceIncidents(serviceID string) error {
	_, err := db.Exec(`DELETE FROM incidents WHERE service_id = ?`, serviceID)
	return err
}

// incidentFilter builds the WHERE clause shared by ListIncidents and
// CountIncidents, so the two can never disagree about what a page counts.
func incidentFilter(serviceID, status, severity string) (string, []any) {
	var where []string
	var args []any
	if serviceID != "" {
		where = append(where, "service_id = ?")
		args = append(args, serviceID)
	}
	if status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if severity != "" {
		where = append(where, "severity = ?")
		args = append(args, severity)
	}
	if len(where) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

// CountIncidents returns how many incidents match the same filter ListIncidents
// would use, ignoring limit/offset — for building pagination.
func (db *DB) CountIncidents(serviceID, status, severity string) (int, error) {
	where, args := incidentFilter(serviceID, status, severity)
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM incidents`+where, args...).Scan(&count)
	return count, err
}

// ListIncidents returns incidents filtered by service, status and/or severity
// (empty string = no filter), newest first. limit <= 0 means no limit.
func (db *DB) ListIncidents(serviceID, status, severity string, limit, offset int) ([]Incident, error) {
	where, args := incidentFilter(serviceID, status, severity)
	q := `SELECT ` + incidentCols + ` FROM incidents` + where + ` ORDER BY started_at DESC, id DESC`
	if limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	incidents := []Incident{}
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, *inc)
	}
	return incidents, rows.Err()
}

// AddIncidentComment appends a comment and returns it (with username resolved).
func (db *DB) AddIncidentComment(incidentID int64, userID *int64, body string, ts int64) (*IncidentComment, error) {
	res, err := db.Exec(
		`INSERT INTO incident_comments (incident_id, user_id, body, created_at) VALUES (?, ?, ?, ?)`,
		incidentID, userID, body, ts)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.getComment(id)
}

func (db *DB) getComment(id int64) (*IncidentComment, error) {
	return scanComment(db.QueryRow(
		`SELECT c.id, c.incident_id, c.user_id, COALESCE(u.username, ''), c.body, c.created_at
		 FROM incident_comments c LEFT JOIN users u ON u.id = c.user_id WHERE c.id = ?`, id))
}

func scanComment(row interface{ Scan(...any) error }) (*IncidentComment, error) {
	var cm IncidentComment
	var userID sql.NullInt64
	if err := row.Scan(&cm.ID, &cm.IncidentID, &userID, &cm.Username, &cm.Body, &cm.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if userID.Valid {
		cm.UserID = &userID.Int64
	}
	return &cm, nil
}

// ListIncidentComments returns an incident's comments, oldest first.
func (db *DB) ListIncidentComments(incidentID int64) ([]IncidentComment, error) {
	rows, err := db.Query(
		`SELECT c.id, c.incident_id, c.user_id, COALESCE(u.username, ''), c.body, c.created_at
		 FROM incident_comments c LEFT JOIN users u ON u.id = c.user_id
		 WHERE c.incident_id = ? ORDER BY c.created_at ASC, c.id ASC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []IncidentComment{}
	for rows.Next() {
		cm, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, *cm)
	}
	return comments, rows.Err()
}

// AllIncidentComments returns every comment across all incidents (for export).
func (db *DB) AllIncidentComments() ([]IncidentComment, error) {
	rows, err := db.Query(
		`SELECT c.id, c.incident_id, c.user_id, COALESCE(u.username, ''), c.body, c.created_at
		 FROM incident_comments c LEFT JOIN users u ON u.id = c.user_id ORDER BY c.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []IncidentComment{}
	for rows.Next() {
		cm, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, *cm)
	}
	return comments, rows.Err()
}

// ReplaceIncidents deletes all incidents/comments and inserts the given ones,
// preserving IDs (for archive import). User references not present in the users
// table are nulled so foreign keys hold across instances.
func (db *DB) ReplaceIncidents(incidents []Incident, comments []IncidentComment) error {
	users, err := db.userIDSet()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM incident_comments`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM incidents`); err != nil {
		return err
	}
	for _, inc := range incidents {
		createdBy := inc.CreatedBy
		if createdBy != nil && !users[*createdBy] {
			createdBy = nil
		}
		// An archive from before severity existed decodes it as "" (the Go zero
		// value); those incidents were all real outages.
		severity := inc.Severity
		if severity == "" {
			severity = SeverityOutage
		}
		if _, err := tx.Exec(
			`INSERT INTO incidents (id, service_id, status, source, severity, title, started_at, resolved_at, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			inc.ID, inc.ServiceID, inc.Status, inc.Source, severity, nullStr(inc.Title),
			inc.StartedAt, inc.ResolvedAt, createdBy, inc.CreatedAt, inc.UpdatedAt); err != nil {
			return err
		}
	}
	for _, cm := range comments {
		userID := cm.UserID
		if userID != nil && !users[*userID] {
			userID = nil
		}
		if _, err := tx.Exec(
			`INSERT INTO incident_comments (id, incident_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`,
			cm.ID, cm.IncidentID, userID, cm.Body, cm.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) userIDSet() (map[int64]bool, error) {
	rows, err := db.Query(`SELECT id FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = true
	}
	return set, rows.Err()
}

// nullStr maps "" to a SQL NULL, otherwise the string.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// isUniqueViolation reports whether err is a SQLite UNIQUE-constraint failure.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
