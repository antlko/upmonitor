// Package image validates and stores service icons as WebP files. Optimization
// (resize + WebP encoding) happens client-side; the backend only validates the
// magic bytes and size, then stores the file atomically.
package image

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxSize is the largest accepted image payload.
const MaxSize = 2 << 20 // 2 MiB

var idRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// SanitizeID validates a service id used as an image filename (blocks traversal).
func SanitizeID(id string) error {
	if !idRe.MatchString(id) {
		return fmt.Errorf("invalid id")
	}
	return nil
}

// IsWebP reports whether data is a RIFF/WEBP image.
func IsWebP(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WEBP"
}

// FileName returns the on-disk image filename for a service id.
func FileName(id string) string { return id + ".webp" }

// Save validates data as WebP and writes it atomically to imagesDir/<id>.webp,
// returning the stored filename.
func Save(imagesDir, id string, data []byte) (string, error) {
	if err := SanitizeID(id); err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty image")
	}
	if len(data) > MaxSize {
		return "", fmt.Errorf("image too large (max %d KiB)", MaxSize/1024)
	}
	if !IsWebP(data) {
		return "", fmt.Errorf("image must be WebP")
	}
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return "", err
	}
	name := FileName(id)
	dst := filepath.Join(imagesDir, name)
	tmp, err := os.CreateTemp(imagesDir, ".img-*.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return "", err
	}
	return name, nil
}

// Delete removes a service's image if present.
func Delete(imagesDir, id string) error {
	if err := SanitizeID(id); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(imagesDir, FileName(id)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ResolvePath returns a safe absolute path for serving filename from imagesDir,
// rejecting anything that isn't a simple <slug>.webp within the directory.
func ResolvePath(imagesDir, filename string) (string, error) {
	if filename != filepath.Base(filename) || strings.Contains(filename, "..") {
		return "", fmt.Errorf("invalid filename")
	}
	if !strings.HasSuffix(filename, ".webp") {
		return "", fmt.Errorf("invalid filename")
	}
	if err := SanitizeID(strings.TrimSuffix(filename, ".webp")); err != nil {
		return "", err
	}
	return filepath.Join(imagesDir, filename), nil
}
