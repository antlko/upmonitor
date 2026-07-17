package db

import "testing"

func TestSeriesFor(t *testing.T) {
	database := openTestDB(t)
	lat := 10

	insert := func(ts int64, status string, latency *int) {
		t.Helper()
		if err := database.InsertCheck("svc", ts, status, latency, nil, ""); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Buckets are epoch-aligned, so with bucketSeconds=100 these land in
	// 1000 (two online), 1200 (all offline) and 1400 (one online). 1300 is
	// deliberately left empty.
	insert(1000, StatusOnline, ptr(20))
	insert(1050, StatusOnline, ptr(40))
	insert(1200, StatusOffline, &lat) // an offline check may still carry a latency
	insert(1250, StatusOffline, nil)
	insert(1400, StatusOnline, ptr(60))

	pts, err := database.SeriesFor("svc", 900, 1500, 100)
	if err != nil {
		t.Fatalf("series: %v", err)
	}

	// The 1300 bucket has no checks, so it must be absent rather than zero —
	// the chart relies on the gap to break its line instead of drawing through.
	if len(pts) != 3 {
		t.Fatalf("len(points) = %d, want 3 (empty buckets emit nothing): %+v", len(pts), pts)
	}

	want := []struct {
		ts     int64
		avg    *float64 // nil ⇒ no online check in the bucket
		errors int
	}{
		{1000, ptrF(30), 0}, // (20+40)/2
		{1200, nil, 2},      // all-offline bucket: latency is nil despite lat being set
		{1400, ptrF(60), 0},
	}
	for i, w := range want {
		got := pts[i]
		if got.Ts != w.ts {
			t.Errorf("point %d ts = %d, want %d", i, got.Ts, w.ts)
		}
		if got.Errors != w.errors {
			t.Errorf("point %d errors = %d, want %d", i, got.Errors, w.errors)
		}
		switch {
		case w.avg == nil && got.AvgLatency != nil:
			t.Errorf("point %d avgLatency = %v, want nil", i, *got.AvgLatency)
		case w.avg != nil && got.AvgLatency == nil:
			t.Errorf("point %d avgLatency = nil, want %v", i, *w.avg)
		case w.avg != nil && *got.AvgLatency != *w.avg:
			t.Errorf("point %d avgLatency = %v, want %v", i, *got.AvgLatency, *w.avg)
		}
	}

	// now bounds the query: the 1400 bucket falls outside [900, 1300].
	pts, err = database.SeriesFor("svc", 900, 1300, 100)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(pts) != 2 {
		t.Errorf("len(points) = %d, want 2 (now must bound the window): %+v", len(pts), pts)
	}

	if pts, err := database.SeriesFor("nope", 0, 9999, 100); err != nil || len(pts) != 0 {
		t.Errorf("missing service = (%+v, %v), want (empty, nil)", pts, err)
	}
}

func TestMetricsForAllHistory(t *testing.T) {
	database := openTestDB(t)

	insert := func(ts int64, status string, latency *int) {
		t.Helper()
		if err := database.InsertCheck("svc", ts, status, latency, nil, ""); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// The offline check at 1200 carries a latency (a fast error response). It
	// must still surface as nil — charting it would draw a healthy line through
	// a failure.
	insert(1000, StatusOnline, ptr(20))
	insert(1100, StatusOffline, ptr(45))
	insert(1200, StatusOffline, nil)
	insert(1300, StatusOnline, ptr(30))

	m, err := database.MetricsForAll(0, 24)
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	got := m["svc"]
	if got == nil {
		t.Fatal("no metrics for svc")
	}

	// Offline checks now occupy slots in the history window, where previously
	// the query filtered them out entirely.
	want := []*int{ptr(20), nil, nil, ptr(30)}
	if len(got.History) != len(want) {
		t.Fatalf("len(history) = %d, want %d: %v", len(got.History), len(want), got.History)
	}
	for i := range want {
		switch {
		case want[i] == nil && got.History[i] != nil:
			t.Errorf("history[%d] = %d, want nil (check was offline)", i, *got.History[i])
		case want[i] != nil && got.History[i] == nil:
			t.Errorf("history[%d] = nil, want %d", i, *want[i])
		case want[i] != nil && *got.History[i] != *want[i]:
			t.Errorf("history[%d] = %d, want %d", i, *got.History[i], *want[i])
		}
	}

	// histLimit keeps the most recent N checks but still reports them oldest-first.
	m, _ = database.MetricsForAll(0, 2)
	trimmed := m["svc"].History
	if len(trimmed) != 2 {
		t.Fatalf("len(history) = %d, want 2", len(trimmed))
	}
	if trimmed[0] != nil {
		t.Errorf("history[0] = %d, want nil (the offline check at ts=1200)", *trimmed[0])
	}
	if trimmed[1] == nil || *trimmed[1] != 30 {
		t.Errorf("history[1] = %v, want 30 (the newest check)", trimmed[1])
	}
}

func ptr(v int) *int { return &v }

func ptrF(v float64) *float64 { return &v }

func TestUptimeSince(t *testing.T) {
	database := openTestDB(t)
	lat := 10

	// 3 online, 1 offline within the window → 75%.
	for _, row := range []struct {
		ts     int64
		status string
	}{
		{1000, StatusOnline},
		{1100, StatusOnline},
		{1200, StatusOffline},
		{1300, StatusOnline},
	} {
		if err := database.InsertCheck("svc", row.ts, row.status, &lat, nil, ""); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	pct, n, err := database.UptimeSince("svc", 900)
	if err != nil {
		t.Fatalf("uptime: %v", err)
	}
	if n != 4 {
		t.Errorf("sampleCount = %d, want 4", n)
	}
	if pct != 75 {
		t.Errorf("uptime = %v, want 75", pct)
	}

	// A window that excludes all rows → 0%% over 0 samples.
	pct, n, _ = database.UptimeSince("svc", 5000)
	if n != 0 || pct != 0 {
		t.Errorf("empty window = (%v, %d), want (0, 0)", pct, n)
	}

	// Unknown service → 0/0, no error (COALESCE guards the NULL SUM).
	if pct, n, err := database.UptimeSince("nope", 0); err != nil || pct != 0 || n != 0 {
		t.Errorf("missing service = (%v, %d, %v)", pct, n, err)
	}
}
