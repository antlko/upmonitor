package db

import "testing"

// A warning means the service answered (on a retry), so it belongs in the
// uptime numerator and not in the error count. This pins the `IN ('online',
// 'warning')` rule that every aggregate in checks.go shares.
func TestWarningCountsAsUptime(t *testing.T) {
	database := openTestDB(t)
	insert := func(ts int64, status string, latency *int, attempts int) {
		t.Helper()
		if err := database.InsertCheck("svc", ts, status, latency, nil, "", attempts); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert(1000, StatusOnline, ptr(20), 1)
	insert(1100, StatusWarning, ptr(50), 2)
	insert(1200, StatusOffline, ptr(10), 3)
	insert(1300, StatusOnline, ptr(30), 1)

	pct, n, err := database.UptimeSince("svc", 0)
	if err != nil {
		t.Fatalf("uptime: %v", err)
	}
	if n != 4 || pct != 75 {
		t.Errorf("uptime = %.2f%% over %d samples, want 75.00%% over 4", pct, n)
	}

	metrics, err := database.MetricsForAll(0, 10)
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	m := metrics["svc"]
	if m.ErrorCount != 1 {
		t.Errorf("error count = %d, want 1 (warnings are not errors)", m.ErrorCount)
	}
	if m.WarningCount != 1 {
		t.Errorf("warning count = %d, want 1", m.WarningCount)
	}
	if m.Uptime != 75 {
		t.Errorf("uptime = %.2f, want 75", m.Uptime)
	}
}

// LastSuccess must include a warning: the service did respond.
func TestLastSuccessIncludesWarning(t *testing.T) {
	database := openTestDB(t)
	if err := database.InsertCheck("svc", 1000, StatusOnline, ptr(20), nil, "", 1); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := database.InsertCheck("svc", 1100, StatusWarning, ptr(50), nil, "boom", 2); err != nil {
		t.Fatalf("insert: %v", err)
	}
	metrics, err := database.MetricsForAll(0, 10)
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	if got := metrics["svc"].LastSuccess; got == nil || *got != 1100 {
		t.Errorf("last success = %v, want 1100", got)
	}
}

// The warning's latency is real and must reach the average; the offline check's
// must not, even though it has one.
func TestSeriesForWarnings(t *testing.T) {
	database := openTestDB(t)
	if err := database.InsertCheck("svc", 1000, StatusWarning, ptr(50), nil, "", 2); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := database.InsertCheck("svc", 1050, StatusOffline, ptr(10), nil, "", 3); err != nil {
		t.Fatalf("insert: %v", err)
	}

	points, err := database.SeriesFor("svc", 0, 2000, 100)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("got %d points, want 1", len(points))
	}
	p := points[0]
	if p.AvgLatency == nil || *p.AvgLatency != 50 {
		t.Errorf("avg latency = %v, want 50 (the offline latency must not pollute it)", p.AvgLatency)
	}
	if p.Errors != 1 {
		t.Errorf("errors = %d, want 1", p.Errors)
	}
	if p.Warnings != 1 {
		t.Errorf("warnings = %d, want 1", p.Warnings)
	}
}

// HistoryStatus is what lets the UI colour a warning point amber: a warning
// keeps its latency, so History alone cannot distinguish it from online.
func TestMetricsForAllHistoryStatus(t *testing.T) {
	database := openTestDB(t)
	rows := []struct {
		ts      int64
		status  string
		latency *int
	}{
		{1000, StatusOnline, ptr(20)},
		{1100, StatusWarning, ptr(50)},
		{1200, StatusOffline, ptr(10)},
	}
	for _, r := range rows {
		if err := database.InsertCheck("svc", r.ts, r.status, r.latency, nil, "", 1); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	metrics, err := database.MetricsForAll(0, 10)
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	m := metrics["svc"]
	if len(m.History) != 3 || len(m.HistoryStatus) != 3 {
		t.Fatalf("history = %d entries, statuses = %d, want 3 and 3", len(m.History), len(m.HistoryStatus))
	}
	want := []string{StatusOnline, StatusWarning, StatusOffline}
	for i, w := range want {
		if m.HistoryStatus[i] != w {
			t.Errorf("history status[%d] = %q, want %q", i, m.HistoryStatus[i], w)
		}
	}
	if m.History[1] == nil || *m.History[1] != 50 {
		t.Errorf("warning latency = %v, want 50 (a warning did answer)", m.History[1])
	}
	if m.History[2] != nil {
		t.Errorf("offline latency = %v, want nil", m.History[2])
	}
}

func TestListChecksPagination(t *testing.T) {
	database := openTestDB(t)
	for i := int64(1); i <= 5; i++ {
		if err := database.InsertCheck("svc", 1000+i, StatusOnline, ptr(int(i)), nil, "", 1); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	var seen []int64
	var before int64
	for page := 0; page < 3; page++ {
		rows, err := database.ListChecks("svc", before, 2)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if page < 2 && len(rows) != 2 {
			t.Fatalf("page %d returned %d rows, want 2", page, len(rows))
		}
		for _, r := range rows {
			seen = append(seen, r.Ts)
		}
		if len(rows) == 0 {
			break
		}
		before = rows[len(rows)-1].ID
	}

	want := []int64{1005, 1004, 1003, 1002, 1001}
	if len(seen) != len(want) {
		t.Fatalf("saw %d rows across pages, want %d (no gaps, no overlap)", len(seen), len(want))
	}
	for i, w := range want {
		if seen[i] != w {
			t.Errorf("row %d ts = %d, want %d (newest first)", i, seen[i], w)
		}
	}

	empty, err := database.ListChecks("no-such-service", 0, 10)
	if err != nil {
		t.Fatalf("unknown service: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("unknown service returned %d rows, want 0", len(empty))
	}
}

// Migration 00005 rebuilds two tables. These are the three things the rebuild
// exists for; each one fails loudly against the old schema.
func TestMigration00005Shape(t *testing.T) {
	database := openTestDB(t)

	if err := database.InsertCheck("svc", 1000, StatusWarning, ptr(50), nil, "flaked", 2); err != nil {
		t.Fatalf("warning status rejected (CHECK not widened): %v", err)
	}
	rows, err := database.ListChecks("svc", 0, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list: %v (%d rows)", err, len(rows))
	}
	if rows[0].Attempts != 2 || rows[0].Error != "flaked" {
		t.Errorf("stored row = attempts %d, error %q; want 2, %q", rows[0].Attempts, rows[0].Error, "flaked")
	}

	in, err := database.CreateIntegration("webhook", "Hook", true, false, []byte(`{}`))
	if err != nil {
		t.Fatalf("create integration: %v", err)
	}
	if in.NotifyWarnings {
		t.Error("notifyWarnings defaults to true, want false (warnings are opt-in)")
	}
	if err := database.LogNotification(in.ID, nil, "warning", "sent", "", 1000); err != nil {
		t.Fatalf("warning log with NULL incident_id rejected: %v", err)
	}
}
