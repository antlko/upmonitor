package monitor

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
	"upmonitor/internal/incident"
	"upmonitor/internal/notify"
)

// Scheduler runs one goroutine per service, each checking on its own interval.
// Sync reconciles the running workers with a new set of services (add / remove
// / restart on change), so config edits take effect without a restart.
type Scheduler struct {
	db         *db.DB
	dispatcher *notify.Dispatcher
	client     *http.Client

	mu      sync.Mutex
	workers map[string]*worker
	wg      sync.WaitGroup
}

// worker holds a service's check state. svc and lastStatus are guarded by mu,
// since both the worker goroutine and a manual CheckNow (and Sync) can touch them.
type worker struct {
	mu         sync.Mutex
	svc        config.Service
	lastStatus string
	cancel     context.CancelFunc

	// runMu serialises whole cycles. Without it a manual CheckNow and the
	// ticker can overlap for the length of a retry ladder, and the slower one
	// then acts on a stale `prev` — opening an incident for a service the other
	// one just found healthy.
	runMu sync.Mutex
}

// cycleResult is the verdict of one scheduled cycle, which may have taken
// several attempts.
type cycleResult struct {
	Result
	Attempts int
}

// New creates a scheduler backed by database for storing results and dispatcher
// for incident notifications (dispatcher may be nil).
func New(database *db.DB, dispatcher *notify.Dispatcher) *Scheduler {
	return &Scheduler{
		db:         database,
		dispatcher: dispatcher,
		workers:    make(map[string]*worker),
		client: &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				MaxIdleConns:          50,
				MaxIdleConnsPerHost:   2,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}
}

// Sync reconciles running workers with services: unchanged workers keep running,
// changed ones restart (picking up new interval/url), removed ones stop.
func (s *Scheduler) Sync(services []config.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wanted := make(map[string]bool, len(services))
	for _, svc := range services {
		wanted[svc.ID] = true
		if existing, ok := s.workers[svc.ID]; ok {
			if sameCheck(existing.svc, svc) {
				existing.mu.Lock()
				existing.svc = svc
				existing.mu.Unlock()
				continue
			}
			existing.cancel()
		}
		s.startWorker(svc)
	}
	for id, w := range s.workers {
		if !wanted[id] {
			w.cancel()
			delete(s.workers, id)
		}
	}
}

// startWorker launches (and registers) a worker for svc. Caller holds the lock.
func (s *Scheduler) startWorker(svc config.Service) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &worker{svc: svc, cancel: cancel, lastStatus: incident.InitialStatus(s.db, svc.ID)}
	warnIfLadderExceedsInterval(svc)
	s.workers[svc.ID] = w
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx, w)
	}()
}

// run performs an immediate check, then repeats on the service's interval.
func (s *Scheduler) run(ctx context.Context, w *worker) {
	s.check(ctx, w)
	w.mu.Lock()
	interval := time.Duration(w.svc.Check.Interval) * time.Second
	w.mu.Unlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.check(ctx, w)
		}
	}
}

// check runs one health check and stores the result, unless the worker was
// cancelled mid-flight (which would otherwise record a spurious offline). It
// also updates the current cert snapshot and feeds the status transition into
// the incident detector.
func (s *Scheduler) check(parent context.Context, w *worker) {
	w.runMu.Lock()
	defer w.runMu.Unlock()

	w.mu.Lock()
	svc := w.svc
	prev := w.lastStatus
	w.mu.Unlock()

	// The row is stamped with the cycle's start, not its end: a retry ladder can
	// run for tens of seconds, and an outage began when the first attempt failed,
	// not when the last one gave up. It also keeps rows interval-aligned, which
	// is what the chart's bucket sizing assumes.
	start := time.Now()
	cyc := s.runCycle(parent, svc, prev)
	if parent.Err() != nil {
		return
	}
	now := start.Unix()
	if err := s.db.InsertCheck(svc.ID, now, cyc.Status, cyc.LatencyMs, cyc.StatusCode, cyc.Error, cyc.Attempts); err != nil {
		slog.Error("monitor: store check failed", "service", svc.ID, "error", err)
	}
	s.storeTLS(svc, cyc.Result, now)

	w.mu.Lock()
	w.lastStatus = cyc.Status
	w.mu.Unlock()
	incident.OnTransition(parent, s.db, s.dispatcher, svc, prev,
		incident.Outcome{Status: cyc.Status, Attempts: cyc.Attempts, Error: cyc.Error}, now)
}

// runCycle performs up to RetryAttempts checks and classifies the cycle:
//
//	attempt 1 succeeded       → online
//	a later attempt succeeded → warning (the service is alive, it just blipped)
//	every attempt failed      → offline
//
// A warning reports the winning attempt's latency and status code, but the
// FIRST attempt's error — that error is the only record of why it blipped.
//
// Retries are skipped entirely when the service is already offline: they exist
// to decide whether to open an incident, and one is already open. That keeps a
// week-long outage from tripling the traffic we aim at a dead endpoint.
func (s *Scheduler) runCycle(parent context.Context, svc config.Service, prev string) cycleResult {
	attempts := svc.Check.RetryAttempts
	if attempts < 1 || prev == db.StatusOffline {
		attempts = 1
	}
	timeout := time.Duration(svc.Check.Timeout) * time.Second
	interval := time.Duration(svc.Check.Interval) * time.Second
	start := time.Now()

	var first, last Result
	for i := 1; i <= attempts; i++ {
		if i > 1 {
			d := delayAt(svc.Check.RetryDelays, i-2)
			// Never let a ladder outrun its own interval: cycles would then run
			// back to back, thinning out history and leaving the chart's
			// interval-sized buckets half empty.
			if time.Since(start)+d+timeout > interval {
				return cycleResult{Result: last, Attempts: i - 1}
			}
			if !sleepCtx(parent, d) {
				return cycleResult{Result: last, Attempts: i - 1}
			}
		}
		ctx, cancel := context.WithTimeout(parent, timeout)
		res := Check(ctx, s.client, svc)
		cancel()
		if parent.Err() != nil {
			return cycleResult{Result: res, Attempts: i} // caller drops it
		}
		if i == 1 {
			first = res
		}
		last = res
		if res.Status == db.StatusOnline {
			if i == 1 {
				return cycleResult{Result: res, Attempts: 1}
			}
			res.Status = db.StatusWarning
			res.Error = first.Error
			return cycleResult{Result: res, Attempts: i}
		}
	}
	return cycleResult{Result: last, Attempts: attempts}
}

// delayAt returns the wait before retry i (0-based gap index). N attempts use
// N-1 gaps; the last configured delay repeats when there are more gaps than
// entries, and an empty list means retry immediately.
func delayAt(delays []int, i int) time.Duration {
	if len(delays) == 0 {
		return 0
	}
	if i >= len(delays) {
		i = len(delays) - 1
	}
	d := delays[i]
	if d < 0 {
		d = 0
	}
	return time.Duration(d) * time.Second
}

// sleepCtx waits d, reporting false if ctx was cancelled first. A bare
// time.Sleep here would make Stop() and Sync() block for the whole ladder.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// warnIfLadderExceedsInterval logs when a service's retry ladder cannot fit in
// its check interval. Validate deliberately does not clamp this (we never
// rewrite a user's config), so the log is the operator's only signal.
func warnIfLadderExceedsInterval(svc config.Service) {
	attempts := svc.Check.RetryAttempts
	if attempts < 2 {
		return
	}
	budget := time.Duration(attempts) * time.Duration(svc.Check.Timeout) * time.Second
	for i := 0; i < attempts-1; i++ {
		budget += delayAt(svc.Check.RetryDelays, i)
	}
	if interval := time.Duration(svc.Check.Interval) * time.Second; budget > interval {
		slog.Warn("monitor: retry ladder may exceed check interval; cycles will be cut short",
			"service", svc.ID, "worst_case", budget.String(), "interval", interval.String())
	}
}

// storeTLS records the current certificate snapshot for HTTPS services. A
// successful handshake stores the cert; an HTTPS check that failed before a
// cert could be read stores the error so the UI can surface it. Plain-HTTP
// services are skipped entirely.
func (s *Scheduler) storeTLS(svc config.Service, res Result, now int64) {
	if res.TLS != nil {
		if err := s.db.UpsertServiceTLS(svc.ID, now, res.TLS.NotBefore.Unix(), res.TLS.NotAfter.Unix(),
			res.TLS.Issuer, res.TLS.Subject, ""); err != nil {
			slog.Error("monitor: store tls failed", "service", svc.ID, "error", err)
		}
		return
	}
	// Only a fully failed cycle may replace the stored cert with an error: a
	// warning means the service answered, and blanking its cert would be a lie.
	if res.Status == db.StatusOffline && strings.HasPrefix(svc.URL, "https://") {
		if err := s.db.UpsertServiceTLS(svc.ID, now, 0, 0, "", "", res.Error); err != nil {
			slog.Error("monitor: store tls failed", "service", svc.ID, "error", err)
		}
	}
}

// CheckNow runs a check for svc immediately and synchronously (manual trigger),
// reusing the running worker so it participates in transition detection.
func (s *Scheduler) CheckNow(svc config.Service) {
	s.mu.Lock()
	w := s.workers[svc.ID]
	s.mu.Unlock()
	if w == nil {
		// No running worker (e.g. service just added): synthesize an ephemeral one.
		w = &worker{svc: svc, lastStatus: incident.InitialStatus(s.db, svc.ID)}
	}
	s.check(context.Background(), w)
}

// Stop cancels all workers and waits for them to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	for _, w := range s.workers {
		w.cancel()
	}
	s.workers = make(map[string]*worker)
	s.mu.Unlock()
	s.wg.Wait()
}

// sameCheck reports whether two services have identical check parameters.
func sameCheck(a, b config.Service) bool {
	if a.URL != b.URL || a.Check.Interval != b.Check.Interval ||
		a.Check.Method != b.Check.Method || a.Check.Timeout != b.Check.Timeout ||
		a.Check.RetryAttempts != b.Check.RetryAttempts ||
		!slices.Equal(a.Check.ExpectedStatus, b.Check.ExpectedStatus) ||
		!slices.Equal(a.Check.RetryDelays, b.Check.RetryDelays) {
		return false
	}
	return true
}
