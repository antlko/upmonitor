// Package config defines the on-disk configuration (config.yaml) and its
// load/save/validation logic. config.yaml is the source of truth for services
// and app settings; users, sessions and metrics live in SQLite instead.
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Widget display modes.
const (
	ModeIcon      = "icon"
	ModeName      = "name"
	ModeDashboard = "dashboard"
)

const (
	// CurrentVersion is the schema version written to new config files.
	CurrentVersion = 1

	// DefaultRetentionDays is how long metrics history is kept when the config
	// does not say otherwise. Exported so the api layer's fallbacks cannot drift
	// from the value written into new configs.
	DefaultRetentionDays = 30

	defaultInterval = 30
	defaultTimeout  = 10
	minInterval     = 5
	minTimeout      = 1
)

// Config is the root of config.yaml.
type Config struct {
	Version  int       `yaml:"version"`
	Settings Settings  `yaml:"settings"`
	Services []Service `yaml:"services"`
}

// Settings holds app-wide options.
type Settings struct {
	PublicDashboard   bool          `yaml:"public_dashboard"`
	DefaultWidgetMode string        `yaml:"default_widget_mode"`
	Theme             string        `yaml:"theme"`
	Check             CheckDefaults `yaml:"check"`
}

// CheckDefaults are fallbacks applied to services and the retention window.
type CheckDefaults struct {
	DefaultInterval int `yaml:"default_interval"`
	Timeout         int `yaml:"timeout"`
	RetentionDays   int `yaml:"retention_days"`
}

// Service is a single monitored endpoint.
type Service struct {
	ID     string       `yaml:"id"`
	Name   string       `yaml:"name"`
	URL    string       `yaml:"url"`
	Icon   string       `yaml:"icon,omitempty"`
	Check  ServiceCheck `yaml:"check"`
	Widget Widget       `yaml:"widget"`
	Layout Layout       `yaml:"layout"`
}

// ServiceCheck configures a service's HTTP health check.
type ServiceCheck struct {
	Interval       int    `yaml:"interval"`
	Method         string `yaml:"method,omitempty"`
	Timeout        int    `yaml:"timeout,omitempty"`
	ExpectedStatus []int  `yaml:"expected_status,omitempty"`
}

// Widget controls how a service is rendered on the dashboard.
type Widget struct {
	Mode string `yaml:"mode"`
}

// Layout is a service card's position/size in the dashboard grid.
type Layout struct {
	X int `yaml:"x"`
	Y int `yaml:"y"`
	W int `yaml:"w"`
	H int `yaml:"h"`
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Default returns a fresh config with sensible defaults and no services.
func Default() *Config {
	return &Config{
		Version: CurrentVersion,
		Settings: Settings{
			PublicDashboard:   false,
			DefaultWidgetMode: ModeName,
			Theme:             "dark",
			Check: CheckDefaults{
				DefaultInterval: defaultInterval,
				Timeout:         defaultTimeout,
				RetentionDays:   DefaultRetentionDays,
			},
		},
		Services: []Service{},
	}
}

// Find returns a pointer to the service with the given id, or nil.
func (c *Config) Find(id string) *Service {
	for i := range c.Services {
		if c.Services[i].ID == id {
			return &c.Services[i]
		}
	}
	return nil
}

// normalize fills in defaults for any zero-valued fields.
func (c *Config) normalize() {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	s := &c.Settings
	if s.DefaultWidgetMode == "" {
		s.DefaultWidgetMode = ModeName
	}
	if s.Theme == "" {
		s.Theme = "dark"
	}
	if s.Check.DefaultInterval == 0 {
		s.Check.DefaultInterval = defaultInterval
	}
	if s.Check.Timeout == 0 {
		s.Check.Timeout = defaultTimeout
	}
	if s.Check.RetentionDays == 0 {
		s.Check.RetentionDays = DefaultRetentionDays
	}
	for i := range c.Services {
		svc := &c.Services[i]
		if svc.Check.Interval == 0 {
			svc.Check.Interval = s.Check.DefaultInterval
		}
		if svc.Check.Method == "" {
			svc.Check.Method = "GET"
		}
		if svc.Check.Timeout == 0 {
			svc.Check.Timeout = s.Check.Timeout
		}
		if svc.Widget.Mode == "" {
			svc.Widget.Mode = s.DefaultWidgetMode
		}
	}
}

// Validate normalizes then checks the config for structural errors.
func (c *Config) Validate() error {
	c.normalize()

	if !validMode(c.Settings.DefaultWidgetMode) {
		return fmt.Errorf("invalid default_widget_mode %q", c.Settings.DefaultWidgetMode)
	}
	if c.Settings.Check.RetentionDays < 1 {
		return fmt.Errorf("retention_days must be >= 1")
	}

	seen := make(map[string]bool, len(c.Services))
	for i := range c.Services {
		if err := c.Services[i].Validate(); err != nil {
			return fmt.Errorf("service %d: %w", i, err)
		}
		id := c.Services[i].ID
		if seen[id] {
			return fmt.Errorf("duplicate service id %q", id)
		}
		seen[id] = true
	}
	return nil
}

// Validate checks a single service and clamps out-of-range values.
func (s *Service) Validate() error {
	if !slugRe.MatchString(s.ID) {
		return fmt.Errorf("invalid id %q (use lowercase letters, digits and hyphens)", s.ID)
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required")
	}
	u, err := url.Parse(s.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("url must be a valid http(s) URL")
	}
	if !validMode(s.Widget.Mode) {
		return fmt.Errorf("invalid widget mode %q", s.Widget.Mode)
	}
	if s.Check.Interval < minInterval {
		s.Check.Interval = minInterval
	}
	if s.Check.Timeout < minTimeout {
		s.Check.Timeout = minTimeout
	}
	return nil
}

func validMode(m string) bool {
	return m == ModeIcon || m == ModeName || m == ModeDashboard
}

// Slugify turns an arbitrary name into a valid service id.
func Slugify(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
