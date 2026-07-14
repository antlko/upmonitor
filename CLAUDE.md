# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**upmonitor** — "Simple self-hosted service monitoring." A full-stack app: a **Go backend**
(`backend/`) serves a **Vue 3 SPA** (`web-ui/`) plus a JSON API and runs the monitoring worker. In
production the built SPA is embedded into a single static Go binary (one Docker container). Users add
web services, watch live status/response/uptime on a drag-and-drop dashboard, and manage everything via
`config.yaml`. See `README.md` (install), `docs/` (configuration, API, development).

## Working directories

Two toolchains: run `npm` from `web-ui/`, run `go` from `backend/`. In dev they run side by side
(Vite proxies `/api` + `/images` to the Go server on `:8080`, per `web-ui/vite.config.ts`).

## Commands

**Frontend** (from `web-ui/`): `npm run dev` · `npm run type-check` · `npm run lint` (oxlint→eslint,
both `--fix`) · `npm run build` (type-check + build). **Backend** (from `backend/`): `go run
./cmd/upmonitor --config-dir ../config` · `go build ./...` · `go vet ./...`. **Single binary**: build
SPA → `cp -r web-ui/dist backend/internal/web/dist` → `CGO_ENABLED=0 go build ./cmd/upmonitor`
(the `Dockerfile` automates this). No test framework is wired on the frontend; backend has none yet —
add Go tests under `internal/*` rather than inventing commands.

## Architecture

- **Source of truth split:** `config.yaml` holds services + settings (the app rewrites it atomically);
  **SQLite** (`upmonitor.db`) holds users, sessions and metrics history. Both live in the config dir
  (`/config` in Docker; `UPMONITOR_CONFIG_DIR` / `--config-dir`, default `./config`).
- **Backend packages** (`backend/internal/`): `config` (yaml load/save/validate), `db` (modernc pure-Go
  SQLite — keep `CGO_ENABLED=0`; schema via **goose** migrations in `db/migrations/*.sql`, embedded and
  run on `Open`), `auth` (bcrypt + session tokens; cookies are set by the api layer), `monitor` (HTTP
  checker + goroutine-per-service ticker scheduler; `Sync` reconciles on config change), `image` (WebP
  validate/store), `archive` (export/import zip), `logger` (slog JSON setup), `api` (the `Server` owns
  config+db+scheduler and applies edits copy-on-write under a lock; **Fiber v3** app with per-route
  auth/admin middleware, a central `ErrorHandler` rendering `{ "error": ... }`, and a native SPA
  catch-all serving the embedded FS), `web` (`//go:embed dist`, exposes `web.FS()`). Entry:
  `cmd/upmonitor/main.go` (inits slog, `app.Listen`, graceful `app.ShutdownWithContext`).
- **Frontend:** Vue 3 `<script setup>`, Pinia stores (`auth`, `services`, `settings`, `ui`), typed API
  client in `src/api/`, `useServicesPolling` (~10s) drives live updates. Router guards gate
  setup/login/role.

## Conventions that bite

- **Icons: `@lucide/vue`**, NOT `lucide-vue-next` (deprecated). Some names changed in v1 (e.g.
  `Home`→`House`); verify via type-check.
- **UI = shadcn-vue** (Reka UI v2 + Tailwind v4). Primitives in `src/components/ui` are intentionally
  single-word; `eslint.config.ts` disables `vue/multi-word-component-names` there and allows the
  `const { class: _, ...rest }` omit pattern (`varsIgnorePattern: ^_`).
- **Image optimization is client-side** (Canvas → WebP in `src/lib/image.ts`); the backend only
  validates WebP magic bytes and stores. Do not add server-side image codecs (keeps the binary CGO-free).
- **Icon generation is procedural** (`src/lib/icon-generator.ts`, seeded SVG) — deliberately no AI model.
- Config `layout` YAML serializes the `y` key quoted (`"y"`) because yaml.v3 treats bare `y` as a
  boolean; this is valid and round-trips — don't "fix" it.
- Frontend types in `src/types` intentionally mirror the API DTOs in `backend/internal/api/dto.go`
  (camelCase); keep them in sync when changing shapes.
- **Fiber handlers** return `fiber.NewError(code, msg)` for errors (the central `errorHandler` renders
  them as `{ "error": msg }` — the shape the frontend expects); success via `c.JSON`/`c.Status().JSON`.
  Read the authed user with `userLocal(c)`, params with `c.Params`, body with `decode(c, &dst)`.
- **Migrations**: add a new `backend/internal/db/migrations/NNNNN_name.sql` with `-- +goose Up`/`Down`;
  never edit an already-applied migration. Dialect is `sqlite3`.
- **Logging** is slog JSON to stdout; prefer `slog.InfoContext(c.Context(), …)` with key/value attrs.
  Level via `UPMONITOR_LOG_LEVEL` (debug|info|warn|error).
