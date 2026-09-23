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
	OnTransition(ctx, database, nil, svc, db.StatusOnline, Outcome{Status: db.StatusOnline}, 100)
	if list, _ := database.ListIncidents("svc", "", 0, 0); len(list) != 0 {
		t.Fatalf("no-transition created %d incidents, want 0", len(list))
	}

	// Down: opens exactly one ongoing incident.
	OnTransition(ctx, database, nil, svc, db.StatusOnline, Outcome{Status: db.StatusOffline}, 200)
	ongoing, _ := database.GetOngoingIncident("svc")
	if ongoing == nil || ongoing.StartedAt != 200 {
		t.Fatalf("down should open an incident at ts 200, got %+v", ongoing)
	}

	// Still down: no new incident.
	OnTransition(ctx, database, nil, svc, db.StatusOffline, Outcome{Status: db.StatusOffline}, 250)
	if list, _ := database.ListIncidents("svc", "", 0, 0); len(list) != 1 {
		t.Fatalf("still-down created extra incidents: %d", len(list))
	}

	// Recover: resolves it.
	OnTransition(ctx, database, nil, svc, db.StatusOffline, Outcome{Status: db.StatusOnline}, 300)
	if ongoing, _ := database.GetOngoingIncident("svc"); ongoing != nil {
		t.Fatalf("recover should resolve the incident, still ongoing: %+v", ongoing)
	}

	// A second down→up cycle creates a distinct incident.
	OnTransition(ctx, database, nil, svc, db.StatusOnline, Outcome{Status: db.StatusOffline}, 400)
	OnTransition(ctx, database, nil, svc, db.StatusOffline, Outcome{Status: db.StatusOnline}, 500)
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

// The whole rule in one table: an incident is open exactly while the service is
// offline, and a warning counts as up. The cells that would regress silently
// are offline→warning (must resolve) and warning→offline (must open).
func TestOnTransitionMatrix(t *testing.T) {
	for _, tt := range []struct {
		name        string
		seedOngoing bool
		prev        string
		current     string
		wantOngoing bool
		wantCount   int
	}{
		{"online→online", false, db.StatusOnline, db.StatusOnline, false, 0},
		{"online→warning opens nothing", false, db.StatusOnline, db.StatusWarning, false, 0},
		{"online→offline opens", false, db.StatusOnline, db.StatusOffline, true, 1},
		{"warning→warning", false, db.StatusWarning, db.StatusWarning, false, 0},
		{"warning→online", false, db.StatusWarning, db.StatusOnline, false, 0},
		{"warning→offline opens", false, db.StatusWarning, db.StatusOffline, true, 1},
		{"offline→online resolves", true, db.StatusOffline, db.StatusOnline, false, 1},
		{"offline→warning resolves", true, db.StatusOffline, db.StatusWarning, false, 1},
		{"offline→offline", true, db.StatusOffline, db.StatusOffline, true, 1},
		{"unknown→offline opens", false, db.StatusUnknown, db.StatusOffline, true, 1},
		{"unknown→warning is silent", false, db.StatusUnknown, db.StatusWarning, false, 0},
		{"unknown→online is silent", false, db.StatusUnknown, db.StatusOnline, false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			database := openDB(t)
			svc := config.Service{ID: "svc", Name: "Svc", URL: "https://svc.example"}
			if tt.seedOngoing {
				if _, err := database.CreateIncident("svc", "auto", 100, nil, nil); err != nil {
					t.Fatalf("seed: %v", err)
				}
			}

			OnTransition(context.Background(), database, nil, svc, tt.prev,
				Outcome{Status: tt.current, Attempts: 2, Error: "boom"}, 300)

			ongoing, _ := database.GetOngoingIncident("svc")
			if (ongoing != nil) != tt.wantOngoing {
				t.Errorf("ongoing incident = %v, want %v", ongoing != nil, tt.wantOngoing)
			}
			list, _ := database.ListIncidents("svc", "", 0, 0)
			if len(list) != tt.wantCount {
				t.Errorf("incident count = %d, want %d", len(list), tt.wantCount)
			}
		})
	}
}

// InitialStatus answers one question — is an incident open? — so it must stay a
// two-value seed even when the last stored check was a warning.
func TestInitialStatusNeverWarning(t *testing.T) {
	database := openDB(t)
	if err := database.InsertCheck("svc", 100, db.StatusWarning, nil, nil, "", 2); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := InitialStatus(database, "svc"); got != db.StatusOnline {
		t.Errorf("InitialStatus = %q, want online", got)
	}
}
