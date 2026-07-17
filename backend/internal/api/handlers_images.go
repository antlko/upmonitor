package api

import (
	"os"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/config"
	"upmonitor/internal/image"
)

// POST /api/services/:id/image → store a WebP icon (raw body, already optimized
// client-side). Updates the service's icon in config.yaml.
func (s *Server) handleUploadImage(c fiber.Ctx) error {
	id := c.Params("id")
	if s.config().Find(id) == nil {
		return fiber.NewError(fiber.StatusNotFound, "service not found")
	}
	name, err := image.Save(config.ImagesPath(s.dir()), id, c.Body())
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := s.updateConfig(func(cfg *config.Config) error {
		if svc := cfg.Find(id); svc != nil {
			svc.Icon = name
		}
		return nil
	}); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"icon": "/images/" + name})
}

// DELETE /api/services/:id/image → remove a service's icon.
func (s *Server) handleDeleteImage(c fiber.Ctx) error {
	id := c.Params("id")
	if err := image.Delete(config.ImagesPath(s.dir()), id); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := s.updateConfig(func(cfg *config.Config) error {
		if svc := cfg.Find(id); svc != nil {
			svc.Icon = ""
		}
		return nil
	}); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GET /images/:file → serve a stored icon (authenticated).
func (s *Server) handleServeImage(c fiber.Ctx) error {
	if s.authenticate(c) == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	path, err := image.ResolvePath(config.ImagesPath(s.dir()), c.Params("file"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid image")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "image not found")
	}
	// Icons keep a stable filename across replacement, so revalidate each time.
	c.Set("Cache-Control", "no-cache")
	c.Type("webp")
	return c.Send(data)
}
