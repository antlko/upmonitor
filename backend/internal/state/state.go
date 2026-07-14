// Package state persists a tiny bootstrap file that records which config
// directory is active. It lives outside the config directory (since it points
// at it) so the "custom config folder" setting can survive restarts.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State is the bootstrap record.
type State struct {
	ConfigDir string `json:"configDir"`
}

// Path returns the location of state.json (OS config dir, e.g.
// ~/.config/upmonitor/state.json), falling back to ./.upmonitor.
func Path() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = ".upmonitor"
		return filepath.Join(base, "state.json")
	}
	return filepath.Join(base, "upmonitor", "state.json")
}

// Load reads the bootstrap state; a missing file yields a zero State.
func Load() State {
	var s State
	data, err := os.ReadFile(Path())
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

// Save writes the bootstrap state, creating parent directories as needed.
func Save(s State) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
