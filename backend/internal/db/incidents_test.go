package db

import (
	"errors"
	"testing"
)

func TestIncidentCRUDAndOngoingConstraint(t *testing.T) {
	database := openTestDB(t)

	inc, err := database.CreateIncident("svc", "auto", 1000, nil, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if inc.Status != "ongoing" || inc.ResolvedAt != nil {
		t.Errorf("new incident should be ongoing/unresolved, got %+v", inc)
	}

	// A second ongoing incident for the same service is rejected.
	if _, err := database.CreateIncident("svc", "auto", 1001, nil, nil); !errors.Is(err, ErrOngoingExists) {
		t.Errorf("second ongoing create err = %v, want ErrOngoingExists", err)
	}

	// GetOngoingIncident finds it; a different service has none.
	got, err := database.GetOngoingIncident("svc")
	if err != nil || got == nil || got.ID != inc.ID {
		t.Errorf("GetOngoingIncident = %+v, %v", got, err)
	}
	if none, _ := database.GetOngoingIncident("other"); none != nil {
		t.Errorf("expected no ongoing incident for 'other', got %+v", none)
	}

	// Resolve, then a new ongoing incident becomes allowed again.
	resolved, err := database.ResolveOngoingIncident("svc", 2000)
	if err != nil || resolved == nil || resolved.ResolvedAt == nil || *resolved.ResolvedAt != 2000 {
		t.Fatalf("resolve = %+v, %v", resolved, err)
	}
	if _, err := database.CreateIncident("svc", "auto", 3000, nil, nil); err != nil {
		t.Errorf("create after resolve should succeed, got %v", err)
	}
}

func TestIncidentCommentsAndList(t *testing.T) {
	database := openTestDB(t)
	user, err := database.CreateUser("alice", "hash", "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	inc, err := database.CreateIncident("svc", "manual", 1000, nil, &user.ID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	cm, err := database.AddIncidentComment(inc.ID, &user.ID, "looking into it", 1500)
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if cm.Username != "alice" {
		t.Errorf("comment username = %q, want alice", cm.Username)
	}

	comments, err := database.ListIncidentComments(inc.ID)
	if err != nil || len(comments) != 1 {
		t.Fatalf("list comments = %d, %v", len(comments), err)
	}

	// Filter list by status.
	ongoing, err := database.ListIncidents("", "ongoing", "", 0, 0)
	if err != nil || len(ongoing) != 1 {
		t.Fatalf("list ongoing = %d, %v", len(ongoing), err)
	}
	resolvedList, _ := database.ListIncidents("", "resolved", "", 0, 0)
	if len(resolvedList) != 0 {
		t.Errorf("expected 0 resolved, got %d", len(resolvedList))
	}

	// Deleting the incident cascades its comments.
	if err := database.DeleteIncident(inc.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if left, _ := database.ListIncidentComments(inc.ID); len(left) != 0 {
		t.Errorf("comments should cascade on delete, got %d", len(left))
	}
}

// CreateWarningEvent must never collide with the one-ongoing-per-service index
// (it is always 'resolved'), even when a real outage is ongoing for the same
// service.
func TestCreateWarningEvent(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateIncident("svc", "auto", 1000, nil, nil); err != nil {
		t.Fatalf("seed ongoing outage: %v", err)
	}

	ev, err := database.CreateWarningEvent("svc", 2000)
	if err != nil {
		t.Fatalf("create warning event: %v", err)
	}
	if ev.Severity != SeverityWarning {
		t.Errorf("severity = %q, want %q", ev.Severity, SeverityWarning)
	}
	if ev.Status != "resolved" || ev.ResolvedAt == nil || *ev.ResolvedAt != 2000 {
		t.Errorf("warning event should be immediately resolved, got %+v", ev)
	}
	if ev.Source != "auto" {
		t.Errorf("source = %q, want auto", ev.Source)
	}

	// The ongoing outage is untouched.
	ongoing, err := database.GetOngoingIncident("svc")
	if err != nil || ongoing == nil {
		t.Fatalf("ongoing outage should still be there: %+v, %v", ongoing, err)
	}
	if ongoing.Severity != SeverityOutage {
		t.Errorf("ongoing incident severity = %q, want %q", ongoing.Severity, SeverityOutage)
	}
}

func TestCountIncidentsMatchesListFilter(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateIncident("a", "auto", 1000, nil, nil); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if _, err := database.CreateIncident("b", "auto", 1000, nil, nil); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	if _, err := database.ResolveOngoingIncident("b", 1500); err != nil {
		t.Fatalf("resolve b: %v", err)
	}

	if n, err := database.CountIncidents("", "", ""); err != nil || n != 2 {
		t.Errorf("count all = %d, %v, want 2", n, err)
	}
	if n, err := database.CountIncidents("", "ongoing", ""); err != nil || n != 1 {
		t.Errorf("count ongoing = %d, %v, want 1", n, err)
	}
	if n, err := database.CountIncidents("a", "", ""); err != nil || n != 1 {
		t.Errorf("count for service a = %d, %v, want 1", n, err)
	}

	// A page (limit=1, offset=1) plus the total should describe the whole set.
	page, err := database.ListIncidents("", "", "", 1, 1)
	if err != nil || len(page) != 1 {
		t.Fatalf("page = %+v, %v", page, err)
	}
}

// A caller that only wants outages (e.g. a chart's outage bands) must not be
// crowded out by warning events sharing the same table.
func TestListIncidentsFiltersBySeverity(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateIncident("svc", "auto", 1000, nil, nil); err != nil {
		t.Fatalf("seed outage: %v", err)
	}
	if _, err := database.CreateWarningEvent("svc", 2000); err != nil {
		t.Fatalf("seed warning: %v", err)
	}

	if n, err := database.CountIncidents("", "", SeverityOutage); err != nil || n != 1 {
		t.Errorf("count outage = %d, %v, want 1", n, err)
	}
	if n, err := database.CountIncidents("", "", SeverityWarning); err != nil || n != 1 {
		t.Errorf("count warning = %d, %v, want 1", n, err)
	}

	outages, err := database.ListIncidents("", "", SeverityOutage, 0, 0)
	if err != nil || len(outages) != 1 || outages[0].Severity != SeverityOutage {
		t.Fatalf("list outages = %+v, %v", outages, err)
	}
}
