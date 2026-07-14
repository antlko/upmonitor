package api

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/auth"
	"upmonitor/internal/db"
)

const userLocalKey = "user"

// authenticate resolves the session cookie to a user and slides its expiry.
func (s *Server) authenticate(c fiber.Ctx) *db.User {
	token := c.Cookies(auth.CookieName)
	if token == "" {
		return nil
	}
	now := time.Now()
	database := s.conn()
	sess, err := database.GetSession(token, now.Unix())
	if err != nil {
		return nil
	}
	u, err := database.GetUserByID(sess.UserID)
	if err != nil {
		return nil
	}
	_ = database.TouchSession(sess.Token, now.Add(auth.SessionTTL).Unix())
	return u
}

// userLocal returns the authenticated user stashed on the context.
func userLocal(c fiber.Ctx) *db.User {
	u, _ := c.Locals(userLocalKey).(*db.User)
	return u
}

// authMW requires a valid session and stashes the user on the context.
func (s *Server) authMW(c fiber.Ctx) error {
	u := s.authenticate(c)
	if u == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	c.Locals(userLocalKey, u)
	return c.Next()
}

// adminMW requires the admin role (runs after authMW).
func (s *Server) adminMW(c fiber.Ctx) error {
	if u := userLocal(c); u == nil || u.Role != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "administrator access required")
	}
	return c.Next()
}

// requestLogger logs API/image requests with method, status and duration.
func (s *Server) requestLogger(c fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	p := c.Path()
	if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/images/") {
		// The error handler sets the final status after this middleware returns,
		// so derive it from the error to log the real response code.
		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) {
				status = fe.Code
			}
		}
		slog.InfoContext(c.Context(), "http request",
			"method", c.Method(),
			"path", p,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
		)
	}
	return err
}

// errorHandler renders errors (and recovered panics) as JSON `{ "error": ... }`,
// matching the frontend's contract.
func errorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := "internal server error"
	var fe *fiber.Error
	if errors.As(err, &fe) {
		code = fe.Code
		msg = fe.Message
	}
	if code >= 500 {
		slog.ErrorContext(c.Context(), "request error",
			"method", c.Method(), "path", c.Path(), "status", code, "error", err.Error())
	}
	return c.Status(code).JSON(fiber.Map{"error": msg})
}

// setSessionCookie writes the session cookie.
func setSessionCookie(c fiber.Ctx, token string, expires time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HTTPOnly: true,
		Secure:   c.Secure(),
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie (logout).
func clearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		Secure:   c.Secure(),
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}
