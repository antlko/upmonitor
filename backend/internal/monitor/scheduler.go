package monitor

import (
	"context"
	"log/slog"
	"net"
	"net/http"
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
	w.mu.Lock()
	svc := w.svc
	prev := w.lastStatus
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(parent, time.Duration(svc.Check.Timeout)*time.Second)
	defer cancel()
	res := Check(ctx, s.client, svc)
	if parent.Err() != nil {
		return
	}
	now := time.Now().Unix()
	if err := s.db.InsertCheck(svc.ID, now, res.Status, res.LatencyMs, res.StatusCode, res.Error); err != nil {
		slog.Error("monitor: store check failed", "service", svc.ID, "error", err)
	}
	s.storeTLS(svc, res, now)

	w.mu.Lock()
	w.lastStatus = res.Status
	w.mu.Unlock()
	incident.OnTransition(parent, s.db, s.dispatcher, svc, prev, res.Status, now)
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
	if res.Status != db.StatusOnline && strings.HasPrefix(svc.URL, "https://") {
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
		len(a.Check.ExpectedStatus) != len(b.Check.ExpectedStatus) {
		return false
	}
	for i := range a.Check.ExpectedStatus {
		if a.Check.ExpectedStatus[i] != b.Check.ExpectedStatus[i] {
			return false
		}
	}
	return true
}
