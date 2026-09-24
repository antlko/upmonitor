package archive

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
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

// writeLegacyBundle builds a minimal archive with a hand-written
// integrations.json, standing in for an export from an older version.
func writeLegacyBundle(buf *bytes.Buffer, dir, integrationsJSON string) error {
	zw := zip.NewWriter(buf)
	cfgData, err := os.ReadFile(config.YAMLPath(dir))
	if err != nil {
		return err
	}
	f, err := zw.Create(config.FileName)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, string(cfgData)); err != nil {
		return err
	}
	f, err = zw.Create("integrations.json")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, integrationsJSON); err != nil {
		return err
	}
	return zw.Close()
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
	if _, err := srcDB.CreateIntegration("telegram", "ops", true, false, cfgJSON); err != nil {
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

	incidents, _ := dstDB.ListIncidents("", "", "", 0, 0)
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
	if _, err := dstDB.CreateIntegration("slack", "keep", true, false, []byte(`{"webhookUrl":"x"}`)); err != nil {
		t.Fatalf("seed integration: %v", err)
	}

	if _, err := Import(dstDir, buf.Bytes(), dstDB); err != nil {
		t.Fatalf("import: %v", err)
	}

	if incidents, _ := dstDB.ListIncidents("", "", "", 0, 0); len(incidents) != 1 {
		t.Errorf("incidents wiped by bundle-less import: got %d, want 1", len(incidents))
	}
	if integrations, _ := dstDB.ListIntegrations(); len(integrations) != 1 {
		t.Errorf("integrations wiped by bundle-less import: got %d, want 1", len(integrations))
	}
}

// A backup that loses the retry settings or the warning opt-in would restore a
// working-looking install that behaves differently. Both ride along: the retry
// fields inside config.yaml, the flag through ReplaceIntegrations.
func TestRoundTripPreservesRetryAndWarningSettings(t *testing.T) {
	srcDir, srcDB := seed(t)

	cfg := config.Default()
	cfg.Settings.Check.RetryAttempts = 4
	cfg.Settings.Check.RetryDelays = []int{2, 6}
	cfg.Services = []config.Service{{
		ID: "api", Name: "API", URL: "https://api.example.com",
		Check:  config.ServiceCheck{Interval: 30, RetryAttempts: 2, RetryDelays: []int{7}},
		Widget: config.Widget{Mode: config.ModeName},
	}}
	if err := config.Save(srcDir, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if _, err := srcDB.CreateIntegration("slack", "ops", true, true, []byte(`{"webhookUrl":"https://hook"}`)); err != nil {
		t.Fatalf("create integration: %v", err)
	}

	var buf bytes.Buffer
	if err := Export(srcDir, &buf, srcDB); err != nil {
		t.Fatalf("export: %v", err)
	}

	dstDir, dstDB := seed(t)
	restored, err := Import(dstDir, buf.Bytes(), dstDB)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if got := restored.Settings.Check.RetryAttempts; got != 4 {
		t.Errorf("settings retry attempts = %d, want 4", got)
	}
	if got := restored.Settings.Check.RetryDelays; len(got) != 2 || got[0] != 2 || got[1] != 6 {
		t.Errorf("settings retry delays = %v, want [2 6]", got)
	}
	svc := restored.Find("api")
	if svc == nil {
		t.Fatal("service api missing after import")
	}
	if svc.Check.RetryAttempts != 2 {
		t.Errorf("service retry attempts = %d, want 2", svc.Check.RetryAttempts)
	}
	if len(svc.Check.RetryDelays) != 1 || svc.Check.RetryDelays[0] != 7 {
		t.Errorf("service retry delays = %v, want [7]", svc.Check.RetryDelays)
	}

	list, err := dstDB.ListIntegrations()
	if err != nil || len(list) != 1 {
		t.Fatalf("list integrations: %v (%d)", err, len(list))
	}
	if !list[0].NotifyWarnings {
		t.Error("notifyWarnings was lost in the round trip")
	}
}

// An archive written before warnings existed has no notifyWarnings key; the
// zero value must keep the opt-in off rather than silently enabling it.
func TestImportOfPreWarningArchiveKeepsOptOut(t *testing.T) {
	dir, database := seed(t)
	old := `{"integrations":[{"ID":1,"Type":"slack","Name":"ops","Enabled":true,` +
		`"Config":{"webhookUrl":"https://hook"},"CreatedAt":1,"UpdatedAt":1}]}`

	var buf bytes.Buffer
	if err := writeLegacyBundle(&buf, dir, old); err != nil {
		t.Fatalf("build archive: %v", err)
	}
	if _, err := Import(dir, buf.Bytes(), database); err != nil {
		t.Fatalf("import: %v", err)
	}

	list, err := database.ListIntegrations()
	if err != nil || len(list) != 1 {
		t.Fatalf("list integrations: %v (%d)", err, len(list))
	}
	if list[0].NotifyWarnings {
		t.Error("a pre-warning archive enabled warning notifications, want them off")
	}
}
