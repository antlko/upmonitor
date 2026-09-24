package db

// Check status values.
const (
	StatusOnline  = "online"
	StatusOffline = "offline"
	// StatusWarning is a cycle that failed at least once but succeeded on a
	// retry. The service answered, so it counts as uptime and opens no incident.
	StatusWarning = "warning"
	StatusUnknown = "unknown"
)

// upStatuses is the SQL list of statuses that count as "the service answered".
// Spelled positively rather than as `!= 'offline'` so a future status is not
// silently counted as uptime.
const upStatuses = `('online', 'warning')`

// ServiceMetrics is the aggregated runtime state for one service over a window.
type ServiceMetrics struct {
	Status     string
	LatencyMs  *int
	Uptime     float64
	ErrorCount int
	// WarningCount is how many cycles in the window only succeeded on a retry.
	WarningCount int
	LastCheck    *int64
	LastSuccess  *int64
	// History is the recent check latencies for the sparkline, chronological.
	// A nil entry means that check was offline, not that data is missing.
	History []*int
	// HistoryStatus parallels History element-for-element. A warning check
	// carries a latency just like an online one, so the status is the only way
	// the UI can tell them apart.
	HistoryStatus []string
}

// SeriesPoint is one bucketed sample for the metrics time series.
type SeriesPoint struct {
	Ts         int64    `json:"ts"`
	AvgLatency *float64 `json:"avgLatency"`
	Errors     int      `json:"errors"`
	Warnings   int      `json:"warnings"`
}

// Check is one stored check cycle, as shown in a service's ping console.
type Check struct {
	ID         int64
	Ts         int64
	Status     string
	LatencyMs  *int
	StatusCode *int
	Error      string
	Attempts   int
}

// InsertCheck records one completed check cycle. attempts is how many tries the
// cycle took; for a warning, latency/code come from the attempt that succeeded
// and errMsg is why the first one failed.
func (db *DB) InsertCheck(serviceID string, ts int64, status string, latency, code *int, errMsg string, attempts int) error {
	var errVal any
	if errMsg != "" {
		errVal = errMsg
	}
	if attempts < 1 {
		attempts = 1
	}
	_, err := db.Exec(
		`INSERT INTO checks (service_id, ts, status, latency_ms, status_code, error, attempts) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		serviceID, ts, status, latency, code, errVal, attempts,
	)
	return err
}

// ListChecks returns a service's stored cycles, newest first. beforeID is an
// exclusive keyset cursor (0 for the first page); ids are monotonic per service
// because inserts are append-only, so ordering by id matches ordering by ts.
// Runs of successful checks are returned as-is — collapsing them is a rendering
// concern, and doing it in SQL would break across page boundaries.
func (db *DB) ListChecks(serviceID string, beforeID int64, limit int) ([]Check, error) {
	rows, err := db.Query(`
		SELECT id, ts, status, latency_ms, status_code, COALESCE(error, ''), attempts
		FROM checks
		WHERE service_id = ? AND (? = 0 OR id < ?)
		ORDER BY id DESC LIMIT ?`, serviceID, beforeID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Check{}
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.ID, &c.Ts, &c.Status, &c.LatencyMs, &c.StatusCode, &c.Error, &c.Attempts); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MetricsForAll returns aggregated metrics for every service that has history,
// computed with three set-based queries (independent of the number of services).
// `since` bounds the aggregation window; `histLimit` caps the sparkline length.
func (db *DB) MetricsForAll(since int64, histLimit int) (map[string]*ServiceMetrics, error) {
	out := map[string]*ServiceMetrics{}

	// 1) Latest check per service (status is "current", regardless of window).
	rows, err := db.Query(`
		WITH latest AS (SELECT service_id, MAX(ts) AS mts FROM checks GROUP BY service_id)
		SELECT c.service_id, c.status, c.latency_ms, c.ts
		FROM checks c JOIN latest l ON c.service_id = l.service_id AND c.ts = l.mts`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid, status string
		var latency *int
		var ts int64
		if err := rows.Scan(&sid, &status, &latency, &ts); err != nil {
			rows.Close()
			return nil, err
		}
		t := ts
		out[sid] = &ServiceMetrics{Status: status, LatencyMs: latency, LastCheck: &t,
			History: []*int{}, HistoryStatus: []string{}}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2) Aggregates within the window: uptime %, error count, last success.
	rows, err = db.Query(`
		SELECT service_id,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status IN `+upStatuses+` THEN 1 ELSE 0 END) AS up,
		       SUM(CASE WHEN status = 'offline' THEN 1 ELSE 0 END) AS errs,
		       SUM(CASE WHEN status = 'warning' THEN 1 ELSE 0 END) AS warns,
		       MAX(CASE WHEN status IN `+upStatuses+` THEN ts END) AS last_success
		FROM checks WHERE ts >= ? GROUP BY service_id`, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid string
		var total, up, errs, warns int
		var lastSuccess *int64
		if err := rows.Scan(&sid, &total, &up, &errs, &warns, &lastSuccess); err != nil {
			rows.Close()
			return nil, err
		}
		m := out[sid]
		if m == nil {
			m = &ServiceMetrics{Status: StatusUnknown, History: []*int{}, HistoryStatus: []string{}}
			out[sid] = m
		}
		if total > 0 {
			m.Uptime = float64(up) / float64(total) * 100
		}
		m.ErrorCount = errs
		m.WarningCount = warns
		m.LastSuccess = lastSuccess
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 3) Recent checks per service for sparklines (chronological). Offline checks
	// are included so the sparkline can show an outage; they contribute a nil
	// rather than their latency, because an offline check often still has one (a
	// fast 500) and charting it would draw a healthy-looking line through a
	// failure. A warning kept its latency — the service did answer. Same rule as
	// SeriesFor's average.
	rows, err = db.Query(`
		SELECT service_id, status, latency_ms FROM (
			SELECT service_id, status, latency_ms, ts,
			       ROW_NUMBER() OVER (PARTITION BY service_id ORDER BY ts DESC) AS rn
			FROM checks
			WHERE ts >= ?
		) WHERE rn <= ? ORDER BY service_id, ts ASC`, since, histLimit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid, status string
		var latency *int
		if err := rows.Scan(&sid, &status, &latency); err != nil {
			rows.Close()
			return nil, err
		}
		m := out[sid]
		if m == nil {
			continue
		}
		if status == StatusOffline {
			latency = nil
		}
		m.History = append(m.History, latency)
		m.HistoryStatus = append(m.HistoryStatus, status)
	}
	rows.Close()
	return out, rows.Err()
}

// SeriesFor returns a latency/error time series for one service over
// [since, now], downsampled into fixed `bucketSeconds`-wide buckets aligned to
// the epoch. Buckets containing no checks produce no point at all, and a bucket
// whose checks were all offline yields a nil AvgLatency — callers must treat
// both as breaks rather than drawing through them.
//
// The caller picks the bucket width (see api.chooseBucket) because it depends on
// the service's check interval, which this layer doesn't know about.
func (db *DB) SeriesFor(serviceID string, since, now, bucketSeconds int64) ([]SeriesPoint, error) {
	if bucketSeconds < 1 {
		bucketSeconds = 1
	}
	rows, err := db.Query(`
		SELECT (ts / ?) * ? AS b,
		       AVG(CASE WHEN status IN `+upStatuses+` THEN latency_ms END) AS avg_lat,
		       SUM(CASE WHEN status = 'offline' THEN 1 ELSE 0 END) AS errs,
		       SUM(CASE WHEN status = 'warning' THEN 1 ELSE 0 END) AS warns
		FROM checks WHERE service_id = ? AND ts >= ? AND ts <= ?
		GROUP BY b ORDER BY b ASC`, bucketSeconds, bucketSeconds, serviceID, since, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []SeriesPoint
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(&p.Ts, &p.AvgLatency, &p.Errors, &p.Warnings); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// UptimeSince returns a single service's uptime percentage and the number of
// checks it is based on, over [since, now]. sampleCount is 0 when there is no
// history in the window (uptime is then 0).
func (db *DB) UptimeSince(serviceID string, since int64) (pct float64, sampleCount int, err error) {
	var total, up int
	err = db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN status IN `+upStatuses+` THEN 1 ELSE 0 END), 0)
		FROM checks WHERE service_id = ? AND ts >= ?`, serviceID, since).Scan(&total, &up)
	if err != nil {
		return 0, 0, err
	}
	if total > 0 {
		pct = float64(up) / float64(total) * 100
	}
	return pct, total, nil
}

// WarningServiceCountSince returns how many distinct services logged at least
// one warning cycle since the given time — the dashboard's configurable
// "Warning" tile, whose look-back window is settings.dashboard.warning_period_hours
// rather than the fixed retention window MetricsForAll uses.
func (db *DB) WarningServiceCountSince(since int64) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(DISTINCT service_id) FROM checks WHERE ts >= ? AND status = ?`,
		since, StatusWarning).Scan(&count)
	return count, err
}

// DeleteOlderThan removes checks older than cutoff and returns the row count.
func (db *DB) DeleteOlderThan(cutoff int64) (int64, error) {
	res, err := db.Exec(`DELETE FROM checks WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteServiceHistory removes all checks for a service (used when it's deleted).
func (db *DB) DeleteServiceHistory(serviceID string) error {
	_, err := db.Exec(`DELETE FROM checks WHERE service_id = ?`, serviceID)
	return err
}
