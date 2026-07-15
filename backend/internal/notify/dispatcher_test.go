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
	if _, err := database.CreateIntegration("webhook", "on", true, enabledCfg); err != nil {
		t.Fatalf("create enabled: %v", err)
	}
	if _, err := database.CreateIntegration("webhook", "off", false, enabledCfg); err != nil {
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
	if _, err := database.CreateIntegration("webhook", "bad", true, cfg); err != nil {
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
