package db

import (
	"path/filepath"
	"testing"
)

// openTestDB opens a fresh migrated database in a temp dir for tests.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestMigrationsCreateTables(t *testing.T) {
	database := openTestDB(t)
	want := []string{
		"users", "sessions", "checks",
		"service_tls", "incidents", "incident_comments",
		"integrations", "notification_log",
	}
	for _, table := range want {
		var name string
		err := database.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not created: %v", table, err)
		}
	}
}
