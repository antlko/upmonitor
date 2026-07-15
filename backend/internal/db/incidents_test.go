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
	ongoing, err := database.ListIncidents("", "ongoing", 0, 0)
	if err != nil || len(ongoing) != 1 {
		t.Fatalf("list ongoing = %d, %v", len(ongoing), err)
	}
	resolvedList, _ := database.ListIncidents("", "resolved", 0, 0)
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
