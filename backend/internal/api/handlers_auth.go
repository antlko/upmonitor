package api

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/auth"
)

// GET /api/setup/status → whether the first-run admin still needs creating.
func (s *Server) handleSetupStatus(c fiber.Ctx) error {
	n, err := s.conn().CountUsers()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "database error")
	}
	return c.JSON(fiber.Map{"needsSetup": n == 0})
}

// POST /api/setup → create the first admin account (only when none exist).
func (s *Server) handleSetup(c fiber.Ctx) error {
	n, err := s.conn().CountUsers()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "database error")
	}
	if n > 0 {
		return fiber.NewError(fiber.StatusConflict, "setup already completed")
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return fiber.NewError(fiber.StatusBadRequest, "username is required")
	}
	if len(req.Password) < auth.MinPasswordLen || len(req.Password) > auth.MaxPasswordLen {
		return fiber.NewError(fiber.StatusBadRequest, "password must be 8–72 characters")
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not hash password")
	}
	u, err := s.conn().CreateUser(username, hash, "admin")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not create user")
	}
	if err := s.startSession(c, u.ID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not start session")
	}
	return c.JSON(toUserDTO(u))
}

// POST /api/auth/login → verify credentials and set the session cookie.
func (s *Server) handleLogin(c fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	username := strings.TrimSpace(req.Username)
	if !s.logins.allow(username) {
		return fiber.NewError(fiber.StatusTooManyRequests, "too many attempts, please wait a minute")
	}
	u, err := s.conn().GetUserByUsername(username)
	if err != nil || !auth.VerifyPassword(u.PasswordHash, req.Password) {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid username or password")
	}
	if err := s.startSession(c, u.ID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not start session")
	}
	return c.JSON(toUserDTO(u))
}

// POST /api/auth/logout → delete the session and clear the cookie.
func (s *Server) handleLogout(c fiber.Ctx) error {
	if token := c.Cookies(auth.CookieName); token != "" {
		_ = s.conn().DeleteSession(token)
	}
	clearSessionCookie(c)
	return c.JSON(fiber.Map{"ok": true})
}

// GET /api/auth/me → the current user.
func (s *Server) handleMe(c fiber.Ctx) error {
	return c.JSON(toUserDTO(userLocal(c)))
}

// startSession creates a session row and sets the cookie.
func (s *Server) startSession(c fiber.Ctx, userID int64) error {
	token, err := auth.GenerateToken()
	if err != nil {
		return err
	}
	now := time.Now()
	expires := now.Add(auth.SessionTTL)
	if err := s.conn().CreateSession(token, userID, now.Unix(), expires.Unix()); err != nil {
		return err
	}
	setSessionCookie(c, token, expires)
	return nil
}
