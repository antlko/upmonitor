package api

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/db"
	"upmonitor/internal/notify"
)

// secretFields lists the sensitive keys per integration type. These are never
// echoed back in responses and are preserved on update when omitted.
var secretFields = map[string][]string{
	"telegram": {"botToken"},
	"slack":    {"webhookUrl"},
	"email":    {"password"},
	"webhook":  {},
}

// requiredFields lists keys that must be present and non-empty per type.
var requiredFields = map[string][]string{
	"telegram": {"botToken", "chatId"},
	"slack":    {"webhookUrl"},
	"email":    {"host", "from", "to"},
	"webhook":  {"url"},
}

type integrationInput struct {
	Type    string         `json:"type"`
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
}

// GET /api/integrations → all channels (secrets redacted).
func (s *Server) handleListIntegrations(c fiber.Ctx) error {
	list, err := s.conn().ListIntegrations()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not load integrations")
	}
	out := make([]integrationDTO, 0, len(list))
	for _, in := range list {
		out = append(out, toIntegrationDTO(in))
	}
	return c.JSON(out)
}

// POST /api/integrations → create a channel.
func (s *Server) handleCreateIntegration(c fiber.Ctx) error {
	var in integrationInput
	if err := decode(c, &in); err != nil {
		return err
	}
	if _, ok := requiredFields[in.Type]; !ok {
		return fiber.NewError(fiber.StatusBadRequest, "unknown integration type")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name is required")
	}
	cfg := in.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	if err := validateIntegrationConfig(in.Type, cfg); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	raw, _ := json.Marshal(cfg)
	created, err := s.conn().CreateIntegration(in.Type, name, in.Enabled, raw)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not create integration")
	}
	return c.Status(fiber.StatusCreated).JSON(toIntegrationDTO(*created))
}

// PUT /api/integrations/:id → update a channel. Omitted/blank secret fields keep
// their stored value.
func (s *Server) handleUpdateIntegration(c fiber.Ctx) error {
	existing, err := s.integrationParam(c)
	if err != nil {
		return err
	}
	var in integrationInput
	if err := decode(c, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name is required")
	}
	merged := map[string]any{}
	_ = json.Unmarshal(existing.Config, &merged)
	secret := secretSet(existing.Type)
	for k, v := range in.Config {
		if secret[k] {
			if str, ok := v.(string); ok && str == "" {
				continue // blank secret → keep the stored value
			}
		}
		merged[k] = v
	}
	if err := validateIntegrationConfig(existing.Type, merged); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	raw, _ := json.Marshal(merged)
	updated, err := s.conn().UpdateIntegration(existing.ID, name, in.Enabled, raw)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not update integration")
	}
	return c.JSON(toIntegrationDTO(*updated))
}

// DELETE /api/integrations/:id → remove a channel.
func (s *Server) handleDeleteIntegration(c fiber.Ctx) error {
	existing, err := s.integrationParam(c)
	if err != nil {
		return err
	}
	if err := s.conn().DeleteIntegration(existing.ID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not delete integration")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/integrations/:id/test → send one real test notification.
func (s *Server) handleTestIntegration(c fiber.Ctx) error {
	existing, err := s.integrationParam(c)
	if err != nil {
		return err
	}
	msg := notify.Message{
		Event:       notify.EventIncidentStart,
		ServiceName: "Test notification",
		ServiceURL:  "https://example.com",
		StartedAt:   time.Now(),
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	if err := s.dispatch().Test(ctx, *existing, msg); err != nil {
		return c.JSON(fiber.Map{"ok": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// integrationParam parses :id and loads the integration (404 if absent).
func (s *Server) integrationParam(c fiber.Ctx) (*db.Integration, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid integration id")
	}
	in, err := s.conn().GetIntegration(id)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "integration not found")
	}
	return in, nil
}

func secretSet(kind string) map[string]bool {
	set := map[string]bool{}
	for _, f := range secretFields[kind] {
		set[f] = true
	}
	return set
}

// validateIntegrationConfig checks required string fields are present.
func validateIntegrationConfig(kind string, cfg map[string]any) error {
	for _, field := range requiredFields[kind] {
		if v, _ := cfg[field].(string); strings.TrimSpace(v) == "" {
			return fiber.NewError(fiber.StatusBadRequest, field+" is required")
		}
	}
	return nil
}
