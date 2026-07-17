package api

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/auth"
	"upmonitor/internal/config"
)

// GET /api/settings → current settings + active config directory.
func (s *Server) handleGetSettings(c fiber.Ctx) error {
	return c.JSON(toSettingsDTO(s.config(), s.dir()))
}

// PUT /api/settings → overwrite app settings (not services).
func (s *Server) handleUpdateSettings(c fiber.Ctx) error {
	var in settingsDTO
	if err := decode(c, &in); err != nil {
		return err
	}
	err := s.updateConfig(func(cfg *config.Config) error {
		if in.DefaultWidgetMode != "" {
			cfg.Settings.DefaultWidgetMode = in.DefaultWidgetMode
		}
		if in.Theme != "" {
			cfg.Settings.Theme = in.Theme
		}
		if in.Check.DefaultInterval > 0 {
			cfg.Settings.Check.DefaultInterval = in.Check.DefaultInterval
		}
		if in.Check.Timeout > 0 {
			cfg.Settings.Check.Timeout = in.Check.Timeout
		}
		if in.Check.RetentionDays > 0 {
			cfg.Settings.Check.RetentionDays = in.Check.RetentionDays
		}
		return nil
	})
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(toSettingsDTO(s.config(), s.dir()))
}

// PUT /api/settings/config-path → point the app at a different config folder.
func (s *Server) handleConfigPath(c fiber.Ctx) error {
	var in struct {
		Path string `json:"path"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	dir := strings.TrimSpace(in.Path)
	if dir == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path is required")
	}
	if err := s.switchConfigDir(dir); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not use that folder: "+err.Error())
	}
	return c.JSON(toSettingsDTO(s.config(), s.dir()))
}

// GET /api/config → raw config.yaml text (advanced editor).
func (s *Server) handleGetRawConfig(c fiber.Ctx) error {
	data, err := config.Marshal(s.config())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not render config")
	}
	return c.JSON(fiber.Map{"content": string(data)})
}

// PUT /api/config → replace config.yaml from raw text (validated).
func (s *Server) handlePutRawConfig(c fiber.Ctx) error {
	var in struct {
		Content string `json:"content"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	parsed, err := config.Parse([]byte(in.Content))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := s.updateConfig(func(cfg *config.Config) error {
		*cfg = *parsed
		return nil
	}); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// GET /api/users → all accounts.
func (s *Server) handleListUsers(c fiber.Ctx) error {
	users, err := s.conn().ListUsers()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not load users")
	}
	out := make([]userDTO, 0, len(users))
	for i := range users {
		out = append(out, toUserDTO(&users[i]))
	}
	return c.JSON(out)
}

// POST /api/users → create an account.
func (s *Server) handleCreateUser(c fiber.Ctx) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return fiber.NewError(fiber.StatusBadRequest, "username is required")
	}
	if !validRole(in.Role) {
		return fiber.NewError(fiber.StatusBadRequest, "role must be admin or readonly")
	}
	if len(in.Password) < auth.MinPasswordLen || len(in.Password) > auth.MaxPasswordLen {
		return fiber.NewError(fiber.StatusBadRequest, "password must be 8–72 characters")
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not hash password")
	}
	u, err := s.conn().CreateUser(username, hash, in.Role)
	if err != nil {
		return fiber.NewError(fiber.StatusConflict, "username already exists")
	}
	return c.Status(fiber.StatusCreated).JSON(toUserDTO(u))
}

// PUT /api/users/:id → change role and/or password.
func (s *Server) handleUpdateUser(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}
	var in struct {
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	target, err := s.conn().GetUserByID(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found")
	}
	role := target.Role
	if in.Role != "" {
		if !validRole(in.Role) {
			return fiber.NewError(fiber.StatusBadRequest, "role must be admin or readonly")
		}
		if target.Role == "admin" && in.Role != "admin" {
			if n, _ := s.countAdmins(); n <= 1 {
				return fiber.NewError(fiber.StatusBadRequest, "cannot demote the last administrator")
			}
		}
		role = in.Role
	}
	hash := ""
	if in.Password != "" {
		if len(in.Password) < auth.MinPasswordLen || len(in.Password) > auth.MaxPasswordLen {
			return fiber.NewError(fiber.StatusBadRequest, "password must be 8–72 characters")
		}
		hash, err = auth.HashPassword(in.Password)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "could not hash password")
		}
	}
	if err := s.conn().UpdateUser(id, role, hash); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not update user")
	}
	updated, _ := s.conn().GetUserByID(id)
	return c.JSON(toUserDTO(updated))
}

// DELETE /api/users/:id → remove an account.
func (s *Server) handleDeleteUser(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}
	if me := userLocal(c); me != nil && me.ID == id {
		return fiber.NewError(fiber.StatusBadRequest, "you cannot delete your own account")
	}
	target, err := s.conn().GetUserByID(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found")
	}
	if target.Role == "admin" {
		if n, _ := s.countAdmins(); n <= 1 {
			return fiber.NewError(fiber.StatusBadRequest, "cannot delete the last administrator")
		}
	}
	if err := s.conn().DeleteUser(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not delete user")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) countAdmins() (int, error) {
	users, err := s.conn().ListUsers()
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range users {
		if users[i].Role == "admin" {
			n++
		}
	}
	return n, nil
}

func validRole(role string) bool {
	return role == "admin" || role == "readonly"
}
