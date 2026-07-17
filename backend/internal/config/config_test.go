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
