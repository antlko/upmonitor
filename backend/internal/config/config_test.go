package config

import "testing"

// A config written before per-service chart styles existed has no `chart:` key.
// It must load and pick up the default rather than failing validation — normalize
// runs before the enum check, and this pins that ordering.
func TestParseUpgradesConfigWithoutChart(t *testing.T) {
	cfg, err := Parse([]byte(`
version: 1
settings:
  default_widget_mode: name
services:
  - id: grafana
    name: Grafana
    url: https://grafana.home.lab
    check: { interval: 30 }
    widget: { mode: dashboard }
    layout: { x: 0, y: 0, w: 3, h: 4 }
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := cfg.Services[0].Chart.Type; got != DefaultChartType {
		t.Errorf("chart type = %q, want %q", got, DefaultChartType)
	}
}

func TestValidateChartType(t *testing.T) {
	for _, tt := range []struct {
		chart   string
		wantErr bool
	}{
		{ChartLine, false},
		{ChartBars, false},
		{"", false}, // normalize fills it in before the check
		{"pie", true},
	} {
		cfg := Default()
		cfg.Services = []Service{{
			ID:     "svc",
			Name:   "Svc",
			URL:    "https://example.com",
			Widget: Widget{Mode: ModeName},
			Chart:  Chart{Type: tt.chart},
		}}
		err := cfg.Validate()
		if tt.wantErr && err == nil {
			t.Errorf("chart type %q: want an error, got nil", tt.chart)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("chart type %q: unexpected error %v", tt.chart, err)
		}
	}
}

// Clone must copy Chart by value. It is not in clone.go's explicit deep-copy
// list — only reference-typed fields are — so this guards the day someone gives
// Chart a slice field and the shallow copy silently starts aliasing.
func TestCloneIsolatesChartType(t *testing.T) {
	cfg := Default()
	cfg.Services = []Service{{ID: "svc", Chart: Chart{Type: ChartLine}}}

	clone := cfg.Clone()
	clone.Services[0].Chart.Type = ChartBars

	if got := cfg.Services[0].Chart.Type; got != ChartLine {
		t.Errorf("original chart type = %q after mutating the clone, want %q", got, ChartLine)
	}
}

// Retry settings follow the same zero-means-inherit rule as interval/timeout.
// The []int{0} case matters: an explicit "retry immediately" must survive
// normalize rather than being mistaken for "unset".
func TestNormalizeInheritsRetryConfig(t *testing.T) {
	for _, tt := range []struct {
		name            string
		settingAttempts int
		settingDelays   []int
		svcAttempts     int
		svcDelays       []int
		wantAttempts    int
		wantDelays      []int
	}{
		{"both unset", 0, nil, 0, nil, defaultRetryAttempts, DefaultRetryDelays},
		{"inherits settings", 5, []int{2, 4}, 0, nil, 5, []int{2, 4}},
		{"service wins", 5, []int{2, 4}, 2, []int{7}, 2, []int{7}},
		{"retries disabled stays disabled", 5, []int{2}, 1, nil, 1, []int{2}},
		{"explicit immediate retry is not inherit", 5, []int{9}, 3, []int{0}, 3, []int{0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Settings.Check.RetryAttempts = tt.settingAttempts
			cfg.Settings.Check.RetryDelays = tt.settingDelays
			cfg.Services = []Service{{
				ID: "svc", Name: "Svc", URL: "https://example.com",
				Check: ServiceCheck{RetryAttempts: tt.svcAttempts, RetryDelays: tt.svcDelays},
			}}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}
			got := cfg.Services[0].Check
			if got.RetryAttempts != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", got.RetryAttempts, tt.wantAttempts)
			}
			if !equalInts(got.RetryDelays, tt.wantDelays) {
				t.Errorf("delays = %v, want %v", got.RetryDelays, tt.wantDelays)
			}
		})
	}
}

// Inheriting the settings slice by assignment would alias one array across
// every service; clamping or editing one would then edit them all.
func TestNormalizeDoesNotAliasRetryDelays(t *testing.T) {
	cfg := Default()
	cfg.Settings.Check.RetryDelays = []int{1, 2, 3}
	cfg.Services = []Service{
		{ID: "a", Name: "A", URL: "https://a.example.com"},
		{ID: "b", Name: "B", URL: "https://b.example.com"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	cfg.Services[0].Check.RetryDelays[0] = 99

	if got := cfg.Services[1].Check.RetryDelays[0]; got != 1 {
		t.Errorf("service b delay[0] = %d after mutating service a, want 1", got)
	}
	if got := cfg.Settings.Check.RetryDelays[0]; got != 1 {
		t.Errorf("settings delay[0] = %d after mutating a service, want 1", got)
	}
}

// updateConfig mutates the clone before swapping it in, so a shared backing
// array would let readers observe a half-applied edit. Both levels must copy.
func TestCloneIsolatesRetryDelays(t *testing.T) {
	cfg := Default()
	cfg.Settings.Check.RetryDelays = []int{1, 5, 10}
	cfg.Services = []Service{{ID: "svc", Check: ServiceCheck{RetryDelays: []int{2, 4}}}}

	clone := cfg.Clone()
	clone.Settings.Check.RetryDelays[0] = 99
	clone.Services[0].Check.RetryDelays[0] = 99

	if got := cfg.Settings.Check.RetryDelays[0]; got != 1 {
		t.Errorf("original settings delay[0] = %d after mutating the clone, want 1", got)
	}
	if got := cfg.Services[0].Check.RetryDelays[0]; got != 2 {
		t.Errorf("original service delay[0] = %d after mutating the clone, want 2", got)
	}
}

func TestValidateClampsRetryConfig(t *testing.T) {
	for _, tt := range []struct {
		name         string
		attempts     int
		delays       []int
		wantAttempts int
		wantDelays   []int
	}{
		{"negative attempts", -1, []int{1}, 1, []int{1}},
		{"too many attempts", 99, []int{1}, maxRetryAttempts, []int{1}},
		{"negative delay", 2, []int{-5, 3}, 2, []int{0, 3}},
		{"delay over cap", 2, []int{9999}, 2, []int{maxRetryDelay}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Services = []Service{{
				ID: "svc", Name: "Svc", URL: "https://example.com",
				Check: ServiceCheck{RetryAttempts: tt.attempts, RetryDelays: tt.delays},
			}}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}
			got := cfg.Services[0].Check
			if got.RetryAttempts != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", got.RetryAttempts, tt.wantAttempts)
			}
			if !equalInts(got.RetryDelays, tt.wantDelays) {
				t.Errorf("delays = %v, want %v", got.RetryDelays, tt.wantDelays)
			}
		})
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
