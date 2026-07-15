// Package archive implements configuration export/import as a .zip bundle
// (config.yaml + images/ + incidents.json + integrations.json). Import validates
// everything, snapshots the current config to backups/, then applies the bundle.
package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
	"upmonitor/internal/image"
)

// Zip entry names for the DB-backed data included in a bundle.
const (
	incidentsFile    = "incidents.json"
	integrationsFile = "integrations.json"
)

type incidentsBundle struct {
	Incidents []db.Incident        `json:"incidents"`
	Comments  []db.IncidentComment `json:"comments"`
}

type integrationsBundle struct {
	Integrations []db.Integration `json:"integrations"`
}

// Export writes a zip of dir's config.yaml, images/, and the DB-backed incidents
// and integrations to w. Integration secrets are included in plaintext.
func Export(dir string, w io.Writer, database *db.DB) error {
	zw := zip.NewWriter(w)

	cfgData, err := os.ReadFile(config.YAMLPath(dir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		f, err := zw.Create(config.FileName)
		if err != nil {
			return err
		}
		if _, err := f.Write(cfgData); err != nil {
			return err
		}
	}

	imagesDir := config.ImagesPath(dir)
	entries, _ := os.ReadDir(imagesDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".webp") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(imagesDir, e.Name()))
		if err != nil {
			return err
		}
		f, err := zw.Create(path.Join(config.ImagesDir, e.Name()))
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	}

	if database != nil {
		incidents, comments, err := exportIncidents(database)
		if err != nil {
			return err
		}
		if err := writeJSON(zw, incidentsFile, incidentsBundle{Incidents: incidents, Comments: comments}); err != nil {
			return err
		}
		integrations, err := database.ListIntegrations()
		if err != nil {
			return err
		}
		if err := writeJSON(zw, integrationsFile, integrationsBundle{Integrations: integrations}); err != nil {
			return err
		}
	}
	return zw.Close()
}

func exportIncidents(database *db.DB) ([]db.Incident, []db.IncidentComment, error) {
	incidents, err := database.ListIncidents("", "", 0, 0)
	if err != nil {
		return nil, nil, err
	}
	comments, err := database.AllIncidentComments()
	if err != nil {
		return nil, nil, err
	}
	return incidents, comments, nil
}

func writeJSON(zw *zip.Writer, name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// Backup snapshots dir to dir/backups/backup-<timestamp>.zip and returns the path.
func Backup(dir string, database *db.DB) (string, error) {
	backupsDir := config.BackupsPath(dir)
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("backup-%s.zip", time.Now().Format("20060102-150405"))
	p := filepath.Join(backupsDir, name)
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := Export(dir, f, database); err != nil {
		return "", err
	}
	return p, nil
}

// Import validates a zip bundle, backs up the current config, then applies the
// new config.yaml, images, incidents and integrations. Bundles that omit
// incidents.json / integrations.json (older exports) leave that data untouched.
// It returns the parsed config on success.
func Import(dir string, data []byte, database *db.DB) (*config.Config, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip: %w", err)
	}

	var cfgData, incidentsData, integrationsData []byte
	images := map[string][]byte{}

	for _, f := range zr.File {
		name := path.Clean(f.Name)
		switch {
		case name == config.FileName:
			cfgData, err = readZip(f, 1<<20)
			if err != nil {
				return nil, err
			}
		case name == incidentsFile:
			incidentsData, err = readZip(f, 8<<20)
			if err != nil {
				return nil, err
			}
		case name == integrationsFile:
			integrationsData, err = readZip(f, 1<<20)
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(name, config.ImagesDir+"/") && strings.HasSuffix(name, ".webp"):
			base := path.Base(name)
			if err := image.SanitizeID(strings.TrimSuffix(base, ".webp")); err != nil {
				return nil, fmt.Errorf("invalid image name %q", base)
			}
			b, err := readZip(f, image.MaxSize)
			if err != nil {
				return nil, err
			}
			if !image.IsWebP(b) {
				return nil, fmt.Errorf("image %q is not valid WebP", base)
			}
			images[base] = b
		}
		// Any other entries are ignored.
	}

	if cfgData == nil {
		return nil, fmt.Errorf("archive is missing config.yaml")
	}
	cfg, err := config.Parse(cfgData)
	if err != nil {
		return nil, fmt.Errorf("config.yaml is invalid: %w", err)
	}

	// Parse DB bundles up front so a malformed archive fails before any changes.
	var incidents *incidentsBundle
	if incidentsData != nil {
		var b incidentsBundle
		if err := json.Unmarshal(incidentsData, &b); err != nil {
			return nil, fmt.Errorf("incidents.json is invalid: %w", err)
		}
		incidents = &b
	}
	var integrations *integrationsBundle
	if integrationsData != nil {
		var b integrationsBundle
		if err := json.Unmarshal(integrationsData, &b); err != nil {
			return nil, fmt.Errorf("integrations.json is invalid: %w", err)
		}
		integrations = &b
	}

	// Snapshot current state before making any changes.
	if _, err := Backup(dir, database); err != nil {
		return nil, fmt.Errorf("backup current config: %w", err)
	}

	// Apply: write config atomically, then replace images.
	if err := config.Save(dir, cfg); err != nil {
		return nil, fmt.Errorf("apply config: %w", err)
	}
	imagesDir := config.ImagesPath(dir)
	if err := clearWebP(imagesDir); err != nil {
		return nil, fmt.Errorf("clear images: %w", err)
	}
	for name, b := range images {
		id := strings.TrimSuffix(name, ".webp")
		if _, err := image.Save(imagesDir, id, b); err != nil {
			return nil, fmt.Errorf("write image %q: %w", name, err)
		}
	}

	// Replace DB-backed data only when present in the archive.
	if database != nil && incidents != nil {
		if err := database.ReplaceIncidents(incidents.Incidents, incidents.Comments); err != nil {
			return nil, fmt.Errorf("apply incidents: %w", err)
		}
	}
	if database != nil && integrations != nil {
		if err := database.ReplaceIntegrations(integrations.Integrations); err != nil {
			return nil, fmt.Errorf("apply integrations: %w", err)
		}
	}
	return cfg, nil
}

func readZip(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, limit))
}

func clearWebP(imagesDir string) error {
	entries, err := os.ReadDir(imagesDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".webp") {
			if err := os.Remove(filepath.Join(imagesDir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
