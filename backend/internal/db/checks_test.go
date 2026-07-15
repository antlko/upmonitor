package db

import "testing"

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
