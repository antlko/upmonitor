package monitor

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
)

// Scheduler runs one goroutine per service, each checking on its own interval.
// Sync reconciles the running workers with a new set of services (add / remove
// / restart on change), so config edits take effect without a restart.
type Scheduler struct {
	db     *db.DB
	client *http.Client

	mu      sync.Mutex
	workers map[string]*worker
	wg      sync.WaitGroup
}

type worker struct {
	svc    config.Service
	cancel context.CancelFunc
}

// New creates a scheduler backed by database for storing results.
func New(database *db.DB) *Scheduler {
	return &Scheduler{
		db:      database,
		workers: make(map[string]*worker),
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
				existing.svc = svc
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
	s.workers[svc.ID] = &worker{svc: svc, cancel: cancel}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx, svc)
	}()
}

// run performs an immediate check, then repeats on the service's interval.
func (s *Scheduler) run(ctx context.Context, svc config.Service) {
	s.check(ctx, svc)
	ticker := time.NewTicker(time.Duration(svc.Check.Interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.check(ctx, svc)
		}
	}
}

// check runs one health check and stores the result, unless the worker was
// cancelled mid-flight (which would otherwise record a spurious offline).
func (s *Scheduler) check(parent context.Context, svc config.Service) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(svc.Check.Timeout)*time.Second)
	defer cancel()
	res := Check(ctx, s.client, svc)
	if parent.Err() != nil {
		return
	}
	if err := s.db.InsertCheck(svc.ID, time.Now().Unix(), res.Status, res.LatencyMs, res.StatusCode, res.Error); err != nil {
		slog.Error("monitor: store check failed", "service", svc.ID, "error", err)
	}
}

// CheckNow runs a check for svc immediately and synchronously (manual trigger).
func (s *Scheduler) CheckNow(svc config.Service) {
	s.check(context.Background(), svc)
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
