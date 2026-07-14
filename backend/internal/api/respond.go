package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
)

// decode unmarshals the JSON request body into dst, returning a 400 error on
// failure. Errors are rendered by the central errorHandler.
func decode(c fiber.Ctx, dst any) error {
	if err := json.Unmarshal(c.Body(), dst); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	return nil
}
