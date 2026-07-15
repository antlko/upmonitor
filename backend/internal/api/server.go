// Package api wires the HTTP server (Fiber): routing, middleware and handlers.
// The Server owns the live config snapshot, database and scheduler, and applies
// config changes copy-on-write under a lock.
package api

import (
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	recovermw "github.com/gofiber/fiber/v3/middleware/recover"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
	"upmonitor/internal/monitor"
	"upmonitor/internal/notify"
	"upmonitor/internal/state"
	"upmonitor/internal/web"
)

// maxBodyBytes bounds request bodies (config imports can be up to ~20 MiB).
const maxBodyBytes = 32 << 20

// Server holds all runtime state and serves the HTTP API + embedded SPA.
type Server struct {
	mu         sync.RWMutex
	configDir  string
	cfg        *config.Config
	database   *db.DB
	sched      *monitor.Scheduler
	dispatcher *notify.Dispatcher

	app    *fiber.App
	webFS  fs.FS
	logins *loginLimiter
	stop   chan struct{}
}

// New opens the database and config for configDir, builds the Fiber app, starts
// monitoring and retention, and returns a ready Server.
func New(configDir string) (*Server, error) {
	configDir = filepath.Clean(configDir)
	if err := config.EnsureDir(configDir); err != nil {
		return nil, err
	}
	database, err := db.Open(config.DBPath(configDir))
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(configDir)
	if err != nil {
		database.Close()
		return nil, err
	}

	dispatcher := notify.NewDispatcher(database)
	s := &Server{
		configDir:  configDir,
		cfg:        cfg,
		database:   database,
		sched:      monitor.New(database, dispatcher),
		dispatcher: dispatcher,
		webFS:      web.FS(),
		logins:     newLoginLimiter(),
		stop:       make(chan struct{}),
	}
	s.app = fiber.New(fiber.Config{
		AppName:      "upmonitor",
		BodyLimit:    maxBodyBytes,
		ErrorHandler: errorHandler,
	})
	s.app.Use(recovermw.New())
	s.app.Use(s.requestLogger)
	s.routes()

	s.sched.Sync(cfg.Services)
	go s.retentionLoop()
	return s, nil
}

// App returns the Fiber app (for Listen / graceful shutdown in main).
func (s *Server) App() *fiber.App { return s.app }

// Close stops background work and closes the database.
func (s *Server) Close() {
	close(s.stop)
	s.scheduler().Stop()
	s.mu.Lock()
	s.database.Close()
	s.mu.Unlock()
}

// routes registers every endpoint. Auth/role are applied as per-route middleware
// so each route's requirements are explicit.
func (s *Server) routes() {
	app := s.app
	auth, admin := s.authMW, s.adminMW

	// Public (no auth).
	app.Get("/api/setup/status", s.handleSetupStatus)
	app.Post("/api/setup", s.handleSetup)
	app.Post("/api/auth/login", s.handleLogin)
	app.Post("/api/auth/logout", s.handleLogout)
	app.Get("/api/public/services", s.handlePublicServices)

	// Authenticated.
	app.Get("/api/auth/me", auth, s.handleMe)
	app.Get("/api/services", auth, s.handleListServices)
	app.Get("/api/services/:id/metrics", auth, s.handleServiceMetrics)
	app.Get("/api/settings", auth, s.handleGetSettings)
	app.Get("/api/incidents", auth, s.handleListIncidents)
	app.Get("/api/incidents/:id", auth, s.handleGetIncident)
	app.Post("/api/incidents/:id/comments", auth, s.handleAddIncidentComment)

	// Admin only.
	app.Post("/api/services", auth, admin, s.handleCreateService)
	app.Patch("/api/services/layout", auth, admin, s.handleUpdateLayout)
	app.Put("/api/services/:id", auth, admin, s.handleUpdateService)
	app.Delete("/api/services/:id", auth, admin, s.handleDeleteService)
	app.Post("/api/services/:id/check", auth, admin, s.handleCheckNow)
	app.Post("/api/incidents", auth, admin, s.handleCreateIncident)
	app.Put("/api/incidents/:id", auth, admin, s.handleUpdateIncident)
	app.Delete("/api/incidents/:id", auth, admin, s.handleDeleteIncident)
	app.Post("/api/services/:id/image", auth, admin, s.handleUploadImage)
	app.Delete("/api/services/:id/image", auth, admin, s.handleDeleteImage)
	app.Put("/api/settings", auth, admin, s.handleUpdateSettings)
	app.Put("/api/settings/config-path", auth, admin, s.handleConfigPath)
	app.Get("/api/config", auth, admin, s.handleGetRawConfig)
	app.Put("/api/config", auth, admin, s.handlePutRawConfig)
	app.Get("/api/users", auth, admin, s.handleListUsers)
	app.Post("/api/users", auth, admin, s.handleCreateUser)
	app.Put("/api/users/:id", auth, admin, s.handleUpdateUser)
	app.Delete("/api/users/:id", auth, admin, s.handleDeleteUser)
	app.Get("/api/config/export", auth, admin, s.handleExport)
	app.Post("/api/config/import", auth, admin, s.handleImport)
	app.Get("/api/integrations", auth, admin, s.handleListIntegrations)
	app.Post("/api/integrations", auth, admin, s.handleCreateIntegration)
	app.Put("/api/integrations/:id", auth, admin, s.handleUpdateIntegration)
	app.Delete("/api/integrations/:id", auth, admin, s.handleDeleteIntegration)
	app.Post("/api/integrations/:id/test", auth, admin, s.handleTestIntegration)

	// Images (public when public_dashboard is enabled, else authenticated).
	app.Get("/images/:file", s.handleServeImage)

	// SPA fallback — must be registered last.
	app.Use(s.serveSPA)
}

// serveSPA serves the embedded SPA, falling back to index.html for unknown
// client-side routes. Unmatched /api paths return a JSON 404 instead.
func (s *Server) serveSPA(c fiber.Ctx) error {
	if strings.HasPrefix(c.Path(), "/api/") {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	p := strings.TrimPrefix(path.Clean(c.Path()), "/")
	if p == "" {
		return s.sendIndex(c)
	}
	data, err := fs.ReadFile(s.webFS, p)
	if err != nil {
		return s.sendIndex(c)
	}
	if strings.HasPrefix(p, "assets/") {
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if ext := strings.TrimPrefix(path.Ext(p), "."); ext != "" {
		c.Type(ext)
	}
	return c.Send(data)
}

func (s *Server) sendIndex(c fiber.Ctx) error {
	data, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "web UI not built")
	}
	c.Set("Cache-Control", "no-cache")
	c.Type("html")
	return c.Send(data)
}

// --- Concurrency-safe accessors ------------------------------------------------

func (s *Server) config() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Server) conn() *db.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.database
}

func (s *Server) scheduler() *monitor.Scheduler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sched
}

func (s *Server) dispatch() *notify.Dispatcher {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dispatcher
}

func (s *Server) dir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configDir
}

// updateConfig applies a mutation to a config clone, persists it atomically,
// swaps it in and re-syncs the scheduler — all under the write lock.
func (s *Server) updateConfig(fn func(*config.Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := s.cfg.Clone()
	if err := fn(clone); err != nil {
		return err
	}
	if err := config.Save(s.configDir, clone); err != nil {
		return err
	}
	s.cfg = clone
	s.sched.Sync(clone.Services)
	return nil
}

// switchConfigDir points the server at a new config directory (reopening the
// database and reloading config).
func (s *Server) switchConfigDir(dir string) error {
	dir = filepath.Clean(dir)
	if err := config.EnsureDir(dir); err != nil {
		return err
	}
	newDB, err := db.Open(config.DBPath(dir))
	if err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		newDB.Close()
		return err
	}

	s.scheduler().Stop()
	s.mu.Lock()
	oldDB := s.database
	dispatcher := notify.NewDispatcher(newDB)
	s.configDir = dir
	s.database = newDB
	s.cfg = cfg
	s.sched = monitor.New(newDB, dispatcher)
	s.dispatcher = dispatcher
	sched := s.sched
	s.mu.Unlock()

	sched.Sync(cfg.Services)
	oldDB.Close()
	_ = state.Save(state.State{ConfigDir: dir})
	return nil
}

// retentionLoop periodically deletes old checks and expired sessions.
func (s *Server) retentionLoop() {
	s.runRetention()
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.runRetention()
		}
	}
}

func (s *Server) runRetention() {
	s.mu.RLock()
	days := s.cfg.Settings.Check.RetentionDays
	database := s.database
	s.mu.RUnlock()
	if days < 1 {
		days = config.DefaultRetentionDays
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	if n, err := database.DeleteOlderThan(cutoff); err != nil {
		slog.Error("retention: delete checks", "error", err)
	} else if n > 0 {
		slog.Info("retention: removed old checks", "count", n)
	}
	if err := database.DeleteExpiredSessions(time.Now().Unix()); err != nil {
		slog.Error("retention: delete sessions", "error", err)
	}
}
