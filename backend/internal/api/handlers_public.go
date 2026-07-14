package api

import "github.com/gofiber/fiber/v3"

// GET /api/public/services → read-only service list for the anonymous public
// dashboard (only when public_dashboard is enabled).
func (s *Server) handlePublicServices(c fiber.Ctx) error {
	cfg := s.config()
	if !cfg.Settings.PublicDashboard {
		return fiber.NewError(fiber.StatusForbidden, "public dashboard is disabled")
	}
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
