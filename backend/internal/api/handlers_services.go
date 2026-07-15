package api

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/config"
	"upmonitor/internal/db"
	"upmonitor/internal/image"
)

var errServiceNotFound = errors.New("service not found")

// retentionWindow returns the unix cutoff for the current retention setting.
func (s *Server) retentionWindow(cfg *config.Config) int64 {
	days := cfg.Settings.Check.RetentionDays
	if days < 1 {
		days = config.DefaultRetentionDays
	}
	return time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
}

// GET /api/services → all services with current status + metrics.
func (s *Server) handleListServices(c fiber.Ctx) error {
	cfg := s.config()
	metrics, err := s.conn().MetricsForAll(s.retentionWindow(cfg), 24)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not load metrics")
	}
	out := make([]serviceDTO, 0, len(cfg.Services))
	for _, svc := range cfg.Services {
		out = append(out, toServiceDTO(svc, metrics[svc.ID]))
	}
	return c.JSON(out)
}

type serviceInput struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Interval int    `json:"interval"`
	Mode     string `json:"mode"`
}

// POST /api/services → add a service (persisted to config.yaml).
func (s *Server) handleCreateService(c fiber.Ctx) error {
	var in serviceInput
	if err := decode(c, &in); err != nil {
		return err
	}
	id := config.Slugify(in.Name)
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "a valid name is required")
	}
	err := s.updateConfig(func(cfg *config.Config) error {
		if cfg.Find(id) != nil {
			return fmt.Errorf("a service named %q already exists", in.Name)
		}
		mode := in.Mode
		if mode == "" {
			mode = cfg.Settings.DefaultWidgetMode
		}
		w, h := defaultSize(mode)
		interval := in.Interval
		if interval == 0 {
			interval = cfg.Settings.Check.DefaultInterval
		}
		cfg.Services = append(cfg.Services, config.Service{
			ID:     id,
			Name:   in.Name,
			URL:    in.URL,
			Check:  config.ServiceCheck{Interval: interval, Method: "GET", Timeout: cfg.Settings.Check.Timeout},
			Widget: config.Widget{Mode: mode},
			Layout: config.Layout{X: 0, Y: maxBottom(cfg.Services), W: w, H: h},
		})
		return nil
	})
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(toServiceDTO(*s.config().Find(id), nil))
}

// PUT /api/services/:id → edit a service.
func (s *Server) handleUpdateService(c fiber.Ctx) error {
	id := c.Params("id")
	var in serviceInput
	if err := decode(c, &in); err != nil {
		return err
	}
	err := s.updateConfig(func(cfg *config.Config) error {
		svc := cfg.Find(id)
		if svc == nil {
			return errServiceNotFound
		}
		if in.Name != "" {
			svc.Name = in.Name
		}
		if in.URL != "" {
			svc.URL = in.URL
		}
		if in.Interval != 0 {
			svc.Check.Interval = in.Interval
		}
		if in.Mode != "" {
			svc.Widget.Mode = in.Mode
		}
		return nil
	})
	if err != nil {
		return configErr(err)
	}
	cfg := s.config()
	metrics, _ := s.conn().MetricsForAll(s.retentionWindow(cfg), 24)
	return c.JSON(toServiceDTO(*cfg.Find(id), metrics[id]))
}

// DELETE /api/services/:id → remove a service, its image and its history.
func (s *Server) handleDeleteService(c fiber.Ctx) error {
	id := c.Params("id")
	err := s.updateConfig(func(cfg *config.Config) error {
		idx := -1
		for i := range cfg.Services {
			if cfg.Services[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return errServiceNotFound
		}
		cfg.Services = append(cfg.Services[:idx], cfg.Services[idx+1:]...)
		return nil
	})
	if err != nil {
		return configErr(err)
	}
	_ = image.Delete(config.ImagesPath(s.dir()), id)
	_ = s.conn().DeleteServiceHistory(id)
	_ = s.conn().DeleteServiceTLS(id)
	_ = s.conn().DeleteServiceIncidents(id)
	return c.SendStatus(fiber.StatusNoContent)
}

type layoutItem struct {
	ID   string `json:"id"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Mode string `json:"mode"`
}

// PATCH /api/services/layout → bulk-save grid positions and widget modes.
func (s *Server) handleUpdateLayout(c fiber.Ctx) error {
	var items []layoutItem
	if err := decode(c, &items); err != nil {
		return err
	}
	err := s.updateConfig(func(cfg *config.Config) error {
		for _, it := range items {
			svc := cfg.Find(it.ID)
			if svc == nil {
				continue
			}
			svc.Layout = config.Layout{X: it.X, Y: it.Y, W: it.W, H: it.H}
			if it.Mode != "" {
				svc.Widget.Mode = it.Mode
			}
		}
		return nil
	})
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// POST /api/services/:id/check → run a check immediately.
func (s *Server) handleCheckNow(c fiber.Ctx) error {
	id := c.Params("id")
	svc := s.config().Find(id)
	if svc == nil {
		return fiber.NewError(fiber.StatusNotFound, "service not found")
	}
	s.scheduler().CheckNow(*svc)
	cfg := s.config()
	metrics, _ := s.conn().MetricsForAll(s.retentionWindow(cfg), 24)
	return c.JSON(toServiceDTO(*svc, metrics[id]))
}

type metricsResponse struct {
	serviceDTO
	Series        []db.SeriesPoint `json:"series"`
	UptimeWindows uptimeWindowsDTO `json:"uptimeWindows"`
	TLS           *tlsDTO          `json:"tls"`
}

// rangeSince maps a ?range= value to a lookback window (default 24h).
func rangeSince(now time.Time, r string) time.Time {
	switch r {
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	case "30d":
		return now.Add(-30 * 24 * time.Hour)
	case "365d":
		return now.Add(-365 * 24 * time.Hour)
	default:
		return now.Add(-24 * time.Hour)
	}
}

// GET /api/services/:id/metrics?range=24h|7d|30d|365d → aggregates, time series,
// multi-window uptime and current TLS certificate info.
func (s *Server) handleServiceMetrics(c fiber.Ctx) error {
	id := c.Params("id")
	svc := s.config().Find(id)
	if svc == nil {
		return fiber.NewError(fiber.StatusNotFound, "service not found")
	}
	now := time.Now()
	since := rangeSince(now, c.Query("range"))
	metrics, _ := s.conn().MetricsForAll(since.Unix(), 60)
	series, err := s.conn().SeriesFor(id, since.Unix(), now.Unix(), 96)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not load metrics")
	}
	if series == nil {
		series = []db.SeriesPoint{}
	}

	day := 24 * time.Hour
	up7, _, _ := s.conn().UptimeSince(id, now.Add(-7*day).Unix())
	up30, _, _ := s.conn().UptimeSince(id, now.Add(-30*day).Unix())
	up365, _, _ := s.conn().UptimeSince(id, now.Add(-365*day).Unix())
	tlsInfo, _ := s.conn().GetServiceTLS(id)

	return c.JSON(metricsResponse{
		serviceDTO:    toServiceDTO(*svc, metrics[id]),
		Series:        series,
		UptimeWindows: uptimeWindowsDTO{Days7: up7, Days30: up30, Days365: up365},
		TLS:           toTLSDTO(tlsInfo),
	})
}

// configErr maps a config-update error to a 404 or 400 Fiber error.
func configErr(err error) error {
	if errors.Is(err, errServiceNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "service not found")
	}
	return fiber.NewError(fiber.StatusBadRequest, err.Error())
}

func defaultSize(mode string) (int, int) {
	if mode == config.ModeDashboard {
		return 3, 4
	}
	return 2, 2
}

func maxBottom(services []config.Service) int {
	y := 0
	for _, svc := range services {
		if b := svc.Layout.Y + svc.Layout.H; b > y {
			y = b
		}
	}
	return y
}
