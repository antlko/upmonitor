package db

// Check status values.
const (
	StatusOnline  = "online"
	StatusOffline = "offline"
	StatusUnknown = "unknown"
)

// ServiceMetrics is the aggregated runtime state for one service over a window.
type ServiceMetrics struct {
	Status      string
	LatencyMs   *int
	Uptime      float64
	ErrorCount  int
	LastCheck   *int64
	LastSuccess *int64
	History     []int
}

// SeriesPoint is one bucketed sample for the metrics time series.
type SeriesPoint struct {
	Ts         int64    `json:"ts"`
	AvgLatency *float64 `json:"avgLatency"`
	Errors     int      `json:"errors"`
}

// InsertCheck records a single health-check result.
func (db *DB) InsertCheck(serviceID string, ts int64, status string, latency, code *int, errMsg string) error {
	var errVal any
	if errMsg != "" {
		errVal = errMsg
	}
	_, err := db.Exec(
		`INSERT INTO checks (service_id, ts, status, latency_ms, status_code, error) VALUES (?, ?, ?, ?, ?, ?)`,
		serviceID, ts, status, latency, code, errVal,
	)
	return err
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
		out[sid] = &ServiceMetrics{Status: status, LatencyMs: latency, LastCheck: &t, History: []int{}}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2) Aggregates within the window: uptime %, error count, last success.
	rows, err = db.Query(`
		SELECT service_id,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END) AS up,
		       SUM(CASE WHEN status = 'offline' THEN 1 ELSE 0 END) AS errs,
		       MAX(CASE WHEN status = 'online' THEN ts END) AS last_success
		FROM checks WHERE ts >= ? GROUP BY service_id`, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid string
		var total, up, errs int
		var lastSuccess *int64
		if err := rows.Scan(&sid, &total, &up, &errs, &lastSuccess); err != nil {
			rows.Close()
			return nil, err
		}
		m := out[sid]
		if m == nil {
			m = &ServiceMetrics{Status: StatusUnknown, History: []int{}}
			out[sid] = m
		}
		if total > 0 {
			m.Uptime = float64(up) / float64(total) * 100
		}
		m.ErrorCount = errs
		m.LastSuccess = lastSuccess
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 3) Recent online latencies per service for sparklines (chronological).
	rows, err = db.Query(`
		SELECT service_id, latency_ms FROM (
			SELECT service_id, latency_ms, ts,
			       ROW_NUMBER() OVER (PARTITION BY service_id ORDER BY ts DESC) AS rn
			FROM checks
			WHERE ts >= ? AND status = 'online' AND latency_ms IS NOT NULL
		) WHERE rn <= ? ORDER BY service_id, ts ASC`, since, histLimit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid string
		var latency int
		if err := rows.Scan(&sid, &latency); err != nil {
			rows.Close()
			return nil, err
		}
		if m := out[sid]; m != nil {
			m.History = append(m.History, latency)
		}
	}
	rows.Close()
	return out, rows.Err()
}

// SeriesFor returns a bucketed latency/error time series for one service,
// downsampled to at most `buckets` points across [since, now].
func (db *DB) SeriesFor(serviceID string, since, now int64, buckets int) ([]SeriesPoint, error) {
	span := now - since
	if span < 1 {
		span = 1
	}
	bucket := span / int64(buckets)
	if bucket < 1 {
		bucket = 1
	}
	rows, err := db.Query(`
		SELECT (ts / ?) * ? AS b,
		       AVG(CASE WHEN status = 'online' THEN latency_ms END) AS avg_lat,
		       SUM(CASE WHEN status = 'offline' THEN 1 ELSE 0 END) AS errs
		FROM checks WHERE service_id = ? AND ts >= ?
		GROUP BY b ORDER BY b ASC`, bucket, bucket, serviceID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []SeriesPoint
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(&p.Ts, &p.AvgLatency, &p.Errors); err != nil {
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
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END), 0)
		FROM checks WHERE service_id = ? AND ts >= ?`, serviceID, since).Scan(&total, &up)
	if err != nil {
		return 0, 0, err
	}
	if total > 0 {
		pct = float64(up) / float64(total) * 100
	}
	return pct, total, nil
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
