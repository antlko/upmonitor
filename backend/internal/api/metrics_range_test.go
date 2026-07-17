package api

import (
	"testing"
	"time"
)

func TestRangeSince(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	for _, tt := range []struct {
		r    string
		want time.Duration
	}{
		{"1h", time.Hour},
		{"6h", 6 * time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"365d", 365 * 24 * time.Hour}, // dropped from the tabs, still a valid API value
		{"", 24 * time.Hour},           // unset falls back to 24h
		{"bogus", 24 * time.Hour},
	} {
		if got := now.Sub(rangeSince(now, tt.r)); got != tt.want {
			t.Errorf("rangeSince(%q) looks back %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestChooseBucket(t *testing.T) {
	const (
		hour = int64(3600)
		day  = 24 * hour
	)

	for _, tt := range []struct {
		name     string
		span     int64
		interval int
		want     int64
		wantPts  int64 // buckets across the span; the chart's point count
	}{
		// Default 30s interval: the span drives the choice. Every result must
		// divide its span exactly, or a bucket would straddle the window edge.
		{"1h default", hour, 30, 60, 60},
		{"6h default", 6 * hour, 30, 300, 72},
		{"24h default", day, 30, 900, 96},
		{"7d default", 7 * day, 30, 7200, 84},
		{"30d default", 30 * day, 30, 28800, 90},

		// A check interval wider than span/96 drives the choice instead —
		// otherwise most buckets are empty and the chart renders as a comb.
		{"1h at 5m interval", hour, 300, 300, 12},
		{"1h at 1m interval", hour, 60, 60, 60},
		{"6h at 30m interval", 6 * hour, 1800, 1800, 12},

		// Below the ladder floor we clamp up, not down to a 5s bucket.
		{"1h at min interval", hour, 5, 60, 60},

		// An interval past the top of the ladder can't push us off the end.
		{"24h at absurd interval", day, 999_999, 28800, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := chooseBucket(tt.span, tt.interval)
			if got != tt.want {
				t.Fatalf("chooseBucket(%d, %d) = %d, want %d", tt.span, tt.interval, got, tt.want)
			}
			if got > tt.span {
				return // a bucket wider than the span is fine (one partial bucket)
			}
			if tt.span%got != 0 {
				t.Errorf("bucket %d does not divide span %d exactly (%d buckets + %d remainder)",
					got, tt.span, tt.span/got, tt.span%got)
			}
			if pts := tt.span / got; pts != tt.wantPts {
				t.Errorf("span/bucket = %d points, want %d", pts, tt.wantPts)
			}
			if pts := tt.span / got; pts > targetPoints {
				t.Errorf("span/bucket = %d points, exceeds targetPoints %d", pts, targetPoints)
			}
		})
	}
}
