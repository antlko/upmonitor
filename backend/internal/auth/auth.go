// Package auth provides password hashing (bcrypt) and opaque session tokens.
// Session cookies themselves are set/cleared by the API layer (Fiber).
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// CookieName is the session cookie name.
	CookieName = "upmonitor_session"
	// SessionTTL is how long a session stays valid (sliding).
	SessionTTL = 30 * 24 * time.Hour
	// MaxPasswordLen is bcrypt's hard input limit.
	MaxPasswordLen = 72
	// MinPasswordLen is the minimum acceptable password length.
	MinPasswordLen = 8
)

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

// VerifyPassword reports whether password matches the stored hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken returns a cryptographically random 256-bit hex token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
