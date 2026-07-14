// Command upmonitor is the single-binary server: it serves the embedded web UI
// and the JSON API (Fiber), and runs the monitoring scheduler.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"upmonitor/internal/api"
	"upmonitor/internal/logger"
	"upmonitor/internal/state"
)

func main() {
	logger.Init()

	var (
		addr      string
		configDir string
	)
	flag.StringVar(&addr, "addr", envOr("UPMONITOR_ADDR", ":8080"), "HTTP listen address")
	flag.StringVar(&configDir, "config-dir", "", "config directory (default: $UPMONITOR_CONFIG_DIR or ./config)")
	flag.Parse()

	dir := resolveConfigDir(configDir)
	slog.Info("starting upmonitor", "config_dir", dir, "addr", addr)

	srv, err := api.New(dir)
	if err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
	defer srv.Close()

	app := srv.App()
	go func() {
		if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("listening", "addr", addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

// resolveConfigDir picks the config directory (flag > env > saved state > default)
// and persists the choice so restarts without a flag stay consistent.
func resolveConfigDir(flagDir string) string {
	if flagDir != "" {
		_ = state.Save(state.State{ConfigDir: flagDir})
		return flagDir
	}
	if env := os.Getenv("UPMONITOR_CONFIG_DIR"); env != "" {
		_ = state.Save(state.State{ConfigDir: env})
		return env
	}
	if st := state.Load(); st.ConfigDir != "" {
		return st.ConfigDir
	}
	return "./config"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
