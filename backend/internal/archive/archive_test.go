package archive

import (
	"bytes"
	"testing"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
)

// seed builds a config dir + database with one incident (and comment) and one
// integration, returning the dir and open db.
func seed(t *testing.T) (string, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	if err := config.EnsureDir(dir); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	if err := config.Save(dir, config.Default()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	database, err := db.Open(config.DBPath(dir))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return dir, database
}

func TestExportImportRoundTrip(t *testing.T) {
	srcDir, srcDB := seed(t)
	inc, err := srcDB.CreateIncident("api", "auto", 1000, nil, nil)
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if _, err := srcDB.AddIncidentComment(inc.ID, nil, "investigating", 1100); err != nil {
		t.Fatalf("comment: %v", err)
	}
	cfgJSON := []byte(`{"botToken":"secret-token","chatId":"99"}`)
	if _, err := srcDB.CreateIntegration("telegram", "ops", true, cfgJSON); err != nil {
		t.Fatalf("create integration: %v", err)
	}

	var buf bytes.Buffer
	if err := Export(srcDir, &buf, srcDB); err != nil {
		t.Fatalf("export: %v", err)
	}

	// Import into a fresh dir + db.
	dstDir, dstDB := seed(t)
	if _, err := Import(dstDir, buf.Bytes(), dstDB); err != nil {
		t.Fatalf("import: %v", err)
	}

	incidents, _ := dstDB.ListIncidents("", "", 0, 0)
	if len(incidents) != 1 || incidents[0].ServiceID != "api" {
		t.Fatalf("imported incidents = %+v", incidents)
	}
	comments, _ := dstDB.ListIncidentComments(incidents[0].ID)
	if len(comments) != 1 || comments[0].Body != "investigating" {
		t.Fatalf("imported comments = %+v", comments)
	}
	integrations, _ := dstDB.ListIntegrations()
	if len(integrations) != 1 || string(integrations[0].Config) != string(cfgJSON) {
		t.Fatalf("imported integrations = %+v (secret should round-trip)", integrations)
	}
}

func TestImportWithoutBundlesLeavesDataUntouched(t *testing.T) {
	// An archive exported without a DB has no incidents.json / integrations.json.
	srcDir, _ := seed(t)
	var buf bytes.Buffer
	if err := Export(srcDir, &buf, nil); err != nil {
		t.Fatalf("export: %v", err)
	}

	// The destination already has data that must survive the import.
	dstDir, dstDB := seed(t)
	if _, err := dstDB.CreateIncident("keep", "manual", 1, nil, nil); err != nil {
		t.Fatalf("seed incident: %v", err)
	}
	if _, err := dstDB.CreateIntegration("slack", "keep", true, []byte(`{"webhookUrl":"x"}`)); err != nil {
		t.Fatalf("seed integration: %v", err)
	}

	if _, err := Import(dstDir, buf.Bytes(), dstDB); err != nil {
		t.Fatalf("import: %v", err)
	}

	if incidents, _ := dstDB.ListIncidents("", "", 0, 0); len(incidents) != 1 {
		t.Errorf("incidents wiped by bundle-less import: got %d, want 1", len(incidents))
	}
	if integrations, _ := dstDB.ListIntegrations(); len(integrations) != 1 {
		t.Errorf("integrations wiped by bundle-less import: got %d, want 1", len(integrations))
	}
}
