package incident

import (
	"context"
	"path/filepath"
	"testing"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
)

func openDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestInitialStatus(t *testing.T) {
	database := openDB(t)

	if got := InitialStatus(database, "svc"); got != db.StatusOnline {
		t.Errorf("fresh service InitialStatus = %q, want online", got)
	}

	if _, err := database.CreateIncident("svc", "auto", 1000, nil, nil); err != nil {
		t.Fatalf("seed incident: %v", err)
	}
	if got := InitialStatus(database, "svc"); got != db.StatusOffline {
		t.Errorf("service with ongoing incident InitialStatus = %q, want offline", got)
	}
}

func TestOnTransition(t *testing.T) {
	database := openDB(t)
	svc := config.Service{ID: "svc", Name: "Svc", URL: "https://svc.example"}
	ctx := context.Background()

	// No change: no incident created.
	OnTransition(ctx, database, nil, svc, db.StatusOnline, db.StatusOnline, 100)
	if list, _ := database.ListIncidents("svc", "", 0, 0); len(list) != 0 {
		t.Fatalf("no-transition created %d incidents, want 0", len(list))
	}

	// Down: opens exactly one ongoing incident.
	OnTransition(ctx, database, nil, svc, db.StatusOnline, db.StatusOffline, 200)
	ongoing, _ := database.GetOngoingIncident("svc")
	if ongoing == nil || ongoing.StartedAt != 200 {
		t.Fatalf("down should open an incident at ts 200, got %+v", ongoing)
	}

	// Still down: no new incident.
	OnTransition(ctx, database, nil, svc, db.StatusOffline, db.StatusOffline, 250)
	if list, _ := database.ListIncidents("svc", "", 0, 0); len(list) != 1 {
		t.Fatalf("still-down created extra incidents: %d", len(list))
	}

	// Recover: resolves it.
	OnTransition(ctx, database, nil, svc, db.StatusOffline, db.StatusOnline, 300)
	if ongoing, _ := database.GetOngoingIncident("svc"); ongoing != nil {
		t.Fatalf("recover should resolve the incident, still ongoing: %+v", ongoing)
	}

	// A second down→up cycle creates a distinct incident.
	OnTransition(ctx, database, nil, svc, db.StatusOnline, db.StatusOffline, 400)
	OnTransition(ctx, database, nil, svc, db.StatusOffline, db.StatusOnline, 500)
	all, _ := database.ListIncidents("svc", "", 0, 0)
	if len(all) != 2 {
		t.Fatalf("expected 2 incidents after two cycles, got %d", len(all))
	}
	for _, inc := range all {
		if inc.Status != "resolved" || inc.ResolvedAt == nil {
			t.Errorf("incident %d should be resolved, got %+v", inc.ID, inc)
		}
	}
}
