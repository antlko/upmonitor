package db

import "testing"

func TestUpsertServiceTLSOverwrites(t *testing.T) {
	database := openTestDB(t)

	if err := database.UpsertServiceTLS("svc", 100, 100, 200, "Old CA", "old.example.com", ""); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := database.UpsertServiceTLS("svc", 300, 300, 400, "New CA", "new.example.com", ""); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := database.GetServiceTLS("svc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected a row, got nil")
	}
	if got.ValidUntil != 400 || got.Issuer != "New CA" || got.Subject != "new.example.com" {
		t.Errorf("row not overwritten: %+v", got)
	}

	// Exactly one row must exist for the service (upsert, not insert).
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM service_tls WHERE service_id = ?`, "svc").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

func TestGetServiceTLSMissing(t *testing.T) {
	database := openTestDB(t)
	got, err := database.GetServiceTLS("nope")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing service, got %+v", got)
	}
}
