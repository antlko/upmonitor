package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
)

// codeServer replies with the given status codes in order, repeating the last
// one, and counts the requests it served.
func codeServer(t *testing.T, codes ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := int(hits.Add(1)) - 1
		if i >= len(codes) {
			i = len(codes) - 1
		}
		w.WriteHeader(codes[i])
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestRunCycleClassification(t *testing.T) {
	for _, tt := range []struct {
		name         string
		codes        []int
		attempts     int
		prev         string
		wantStatus   string
		wantAttempts int
		wantHits     int32
	}{
		{"first try succeeds", []int{200}, 3, db.StatusOnline, db.StatusOnline, 1, 1},
		{"recovers on retry", []int{500, 200}, 3, db.StatusOnline, db.StatusWarning, 2, 2},
		{"recovers on last retry", []int{500, 500, 200}, 3, db.StatusOnline, db.StatusWarning, 3, 3},
		{"all attempts fail", []int{500}, 3, db.StatusOnline, db.StatusOffline, 3, 3},
		{"retries disabled", []int{500, 200}, 1, db.StatusOnline, db.StatusOffline, 1, 1},
		{"no retries while already down", []int{500, 200}, 3, db.StatusOffline, db.StatusOffline, 1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv, hits := codeServer(t, tt.codes...)
			s := &Scheduler{client: srv.Client()}
			svc := config.Service{
				ID: "svc", URL: srv.URL,
				Check: config.ServiceCheck{
					Interval: 300, Timeout: 5, Method: "GET",
					RetryAttempts: tt.attempts, RetryDelays: []int{0},
				},
			}

			got := s.runCycle(context.Background(), svc, tt.prev)

			if got.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Attempts != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", got.Attempts, tt.wantAttempts)
			}
			if n := hits.Load(); n != tt.wantHits {
				t.Errorf("served %d requests, want %d", n, tt.wantHits)
			}
		})
	}
}

// A warning reports the winning attempt's latency and code, but the FIRST
// attempt's error — that error is the only record of why it blipped.
func TestRunCycleWarningKeepsFirstError(t *testing.T) {
	srv, _ := codeServer(t, http.StatusBadGateway, http.StatusOK)
	s := &Scheduler{client: srv.Client()}
	svc := config.Service{
		ID: "svc", URL: srv.URL,
		Check: config.ServiceCheck{Interval: 300, Timeout: 5, RetryAttempts: 2, RetryDelays: []int{0}},
	}

	got := s.runCycle(context.Background(), svc, db.StatusOnline)

	if got.Status != db.StatusWarning {
		t.Fatalf("status = %q, want warning", got.Status)
	}
	if got.Error == "" {
		t.Error("warning lost the first attempt's error")
	}
	if got.StatusCode == nil || *got.StatusCode != http.StatusOK {
		t.Errorf("status code = %v, want 200 (from the winning attempt)", got.StatusCode)
	}
	if got.LatencyMs == nil {
		t.Error("warning has no latency, want the winning attempt's")
	}
}

// Stop() and Sync() must not block for the length of a retry ladder.
func TestRunCycleRespectsCancellation(t *testing.T) {
	srv, hits := codeServer(t, http.StatusInternalServerError)
	s := &Scheduler{client: srv.Client()}
	svc := config.Service{
		ID: "svc", URL: srv.URL,
		Check: config.ServiceCheck{Interval: 300, Timeout: 5, RetryAttempts: 3, RetryDelays: []int{30}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	s.runCycle(ctx, svc, db.StatusOnline)

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("cycle took %v after cancellation, want an early return", elapsed)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("served %d requests after cancellation, want 1", n)
	}
}

// A ladder that cannot fit in the interval is cut short rather than letting
// cycles run back to back.
func TestRunCycleBudget(t *testing.T) {
	srv, hits := codeServer(t, http.StatusInternalServerError)
	s := &Scheduler{client: srv.Client()}
	svc := config.Service{
		ID: "svc", URL: srv.URL,
		Check: config.ServiceCheck{Interval: 5, Timeout: 2, RetryAttempts: 5, RetryDelays: []int{3}},
	}

	got := s.runCycle(context.Background(), svc, db.StatusOnline)

	if got.Attempts >= 5 {
		t.Errorf("attempts = %d, want fewer than 5 (budget guard)", got.Attempts)
	}
	if n := hits.Load(); n >= 5 {
		t.Errorf("served %d requests, want fewer than 5", n)
	}
	if got.Status != db.StatusOffline {
		t.Errorf("status = %q, want offline", got.Status)
	}
}

func TestDelayAt(t *testing.T) {
	for _, tt := range []struct {
		name   string
		delays []int
		index  int
		want   time.Duration
	}{
		{"empty means immediate", nil, 0, 0},
		{"first gap", []int{1, 5, 10}, 0, time.Second},
		{"second gap", []int{1, 5, 10}, 1, 5 * time.Second},
		{"past the end repeats the last", []int{1, 5}, 7, 5 * time.Second},
		{"negative is clamped", []int{-3}, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := delayAt(tt.delays, tt.index); got != tt.want {
				t.Errorf("delayAt(%v, %d) = %v, want %v", tt.delays, tt.index, got, tt.want)
			}
		})
	}
}

// Without this, editing retry settings would not restart the worker and the
// change would silently not take effect.
func TestSameCheckDetectsRetryChange(t *testing.T) {
	base := config.Service{
		URL: "https://a.example",
		Check: config.ServiceCheck{
			Interval: 30, Method: "GET", Timeout: 10,
			ExpectedStatus: []int{200}, RetryAttempts: 3, RetryDelays: []int{1, 5},
		},
	}
	same := base
	same.Check.ExpectedStatus = []int{200}
	same.Check.RetryDelays = []int{1, 5}
	if !sameCheck(base, same) {
		t.Error("identical checks reported as different")
	}

	for _, tt := range []struct {
		name   string
		mutate func(*config.Service)
	}{
		{"attempts", func(s *config.Service) { s.Check.RetryAttempts = 5 }},
		{"delay values", func(s *config.Service) { s.Check.RetryDelays = []int{1, 3} }},
		{"delay length", func(s *config.Service) { s.Check.RetryDelays = []int{1} }},
		{"interval", func(s *config.Service) { s.Check.Interval = 60 }},
		{"expected status", func(s *config.Service) { s.Check.ExpectedStatus = []int{204} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			other := base
			other.Check.RetryDelays = append([]int(nil), base.Check.RetryDelays...)
			other.Check.ExpectedStatus = append([]int(nil), base.Check.ExpectedStatus...)
			tt.mutate(&other)
			if sameCheck(base, other) {
				t.Errorf("a changed %s was reported as unchanged", tt.name)
			}
		})
	}
}
