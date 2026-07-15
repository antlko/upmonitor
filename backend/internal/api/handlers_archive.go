package api

import (
	"bytes"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/archive"
)

// maxImportSize caps an uploaded backup archive.
const maxImportSize = 20 << 20 // 20 MiB

// GET /api/config/export → download a backup.zip (config.yaml + images/).
func (s *Server) handleExport(c fiber.Ctx) error {
	var buf bytes.Buffer
	if err := archive.Export(s.dir(), &buf, s.conn()); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "export failed")
	}
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", `attachment; filename="upmonitor-backup.zip"`)
	return c.Send(buf.Bytes())
}

// POST /api/config/import → validate a backup archive, snapshot the current
// config, then apply the archive (raw application/zip body).
func (s *Server) handleImport(c fiber.Ctx) error {
	data := c.Body()
	if len(data) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "empty archive")
	}
	if len(data) > maxImportSize {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "archive too large")
	}
	cfg, err := archive.Import(s.dir(), data, s.conn())
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	// Swap the freshly-applied config into memory and re-sync monitoring.
	s.mu.Lock()
	s.cfg = cfg
	sched := s.sched
	s.mu.Unlock()
	sched.Sync(cfg.Services)

	return c.JSON(fiber.Map{"ok": true, "services": len(cfg.Services)})
}
