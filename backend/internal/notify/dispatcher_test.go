package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"upmonitor/internal/db"
)

func TestDispatcherFansOutToEnabledOnly(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
	}))
	defer srv.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	enabledCfg, _ := json.Marshal(webhookConfig{URL: srv.URL})
	if _, err := database.CreateIntegration("webhook", "on", true, false, enabledCfg); err != nil {
		t.Fatalf("create enabled: %v", err)
	}
	if _, err := database.CreateIntegration("webhook", "off", false, false, enabledCfg); err != nil {
		t.Fatalf("create disabled: %v", err)
	}

	// An incident is needed for the notification_log foreign key.
	inc, err := database.CreateIncident("svc", "auto", 1000, nil, nil)
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	d := NewDispatcher(database)
	d.Notify(context.Background(), Message{Event: EventIncidentStart, IncidentID: inc.ID, ServiceName: "svc"})

	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Errorf("server hits = %d, want 1 (only the enabled integration)", got)
	}

	var logCount, sentCount int
	database.QueryRow(`SELECT COUNT(*) FROM notification_log`).Scan(&logCount)
	database.QueryRow(`SELECT COUNT(*) FROM notification_log WHERE status='sent'`).Scan(&sentCount)
	if logCount != 1 || sentCount != 1 {
		t.Errorf("notification_log rows = %d (sent %d), want 1 sent", logCount, sentCount)
	}
}

func TestDispatcherLogsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	cfg, _ := json.Marshal(webhookConfig{URL: srv.URL})
	if _, err := database.CreateIntegration("webhook", "bad", true, false, cfg); err != nil {
		t.Fatalf("create: %v", err)
	}
	inc, _ := database.CreateIncident("svc", "auto", 1000, nil, nil)

	NewDispatcher(database).Notify(context.Background(), Message{Event: EventIncidentStart, IncidentID: inc.ID})

	var failed int
	database.QueryRow(`SELECT COUNT(*) FROM notification_log WHERE status='failed'`).Scan(&failed)
	if failed != 1 {
		t.Errorf("failed log rows = %d, want 1", failed)
	}
}

// Warnings are opt-in per channel; incident events must not be affected by the
// same filter. The NULL incident_id assertion is what proves migration 00005
// landed — it fails loudly against the old NOT NULL schema.
func TestDispatcherSkipsWarningsUnlessOptedIn(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
	}))
	defer srv.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	cfg, _ := json.Marshal(webhookConfig{URL: srv.URL})
	if _, err := database.CreateIntegration("webhook", "opted in", true, true, cfg); err != nil {
		t.Fatalf("create opted-in: %v", err)
	}
	if _, err := database.CreateIntegration("webhook", "default", true, false, cfg); err != nil {
		t.Fatalf("create default: %v", err)
	}

	d := NewDispatcher(database)
	d.Notify(context.Background(), Message{Event: EventWarning, ServiceName: "svc", Attempts: 2})

	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Errorf("warning reached %d channels, want 1 (only the opted-in one)", got)
	}

	var n int
	err = database.QueryRow(
		`SELECT COUNT(*) FROM notification_log WHERE event = 'warning' AND incident_id IS NULL`).Scan(&n)
	if err != nil {
		t.Fatalf("count log: %v", err)
	}
	if n != 1 {
		t.Errorf("logged %d warning rows with a NULL incident, want 1", n)
	}

	// The opt-in filter must not leak into incident events.
	atomic.StoreInt64(&hits, 0)
	inc, err := database.CreateIncident("svc", "auto", 1000, nil, nil)
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	d.Notify(context.Background(), Message{Event: EventIncidentStart, IncidentID: inc.ID, ServiceName: "svc"})
	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Errorf("incident reached %d channels, want 2", got)
	}
}
