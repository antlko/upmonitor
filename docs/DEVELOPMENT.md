# Development

upmonitor is a Go backend (`backend/`) that serves a Vue 3 SPA (`web-ui/`). In
production the built SPA is embedded into the Go binary; in development you run
the two side by side and let Vite proxy API calls to the backend.

## Prerequisites

- **Go** 1.25+
- **Node** 22+ (or the version in `web-ui/package.json` `engines`)

## Run both dev servers

**Terminal 1 — backend** (serves the API on `:8080`, writes to `./config`):

```bash
cd backend
go run ./cmd/upmonitor --config-dir ../config --addr :8080
```

**Terminal 2 — frontend** (Vite dev server on `:5173`):

```bash
cd web-ui
npm install
npm run dev
```

Open **http://localhost:5173**. Vite proxies `/api` and `/images` to the backend
(see `web-ui/vite.config.ts`), so cookies and requests work end to end. Edits to
either side hot-reload.

## Build the single binary (production layout)

The Go binary embeds `backend/internal/web/dist`. Build the SPA, copy it in, then
build the binary:

```bash
# 1. Build the SPA
cd web-ui && npm ci && npm run build && cd ..

# 2. Embed it into the backend
rm -rf backend/internal/web/dist
cp -r web-ui/dist backend/internal/web/dist

# 3. Build the static binary (pure Go, no CGO)
cd backend
CGO_ENABLED=0 go build -o ../upmonitor ./cmd/upmonitor

# 4. Run it — serves UI + API on :8080
cd .. && ./upmonitor --config-dir ./config
```

The `Dockerfile` automates exactly these steps across build stages.

## Quality gates

**Frontend** (from `web-ui/`):

```bash
npm run type-check   # vue-tsc, no emit
npm run lint         # oxlint + eslint (auto-fix)
npm run build        # type-check + production build
```

**Backend** (from `backend/`):

```bash
go vet ./...
go build ./...
go test ./...        # add tests under internal/*
```

## Project layout

```
backend/
  cmd/upmonitor/         # main: flags, wiring, graceful shutdown
  internal/
    config/              # config.yaml types, load/save/validate (atomic)
    state/               # bootstrap state.json (active config dir)
    db/                  # SQLite: users, sessions, checks, tls, incidents, integrations
      migrations/        #   goose *.sql migrations (embedded, run on Open)
    auth/                # bcrypt + session tokens
    monitor/             # HTTP checker (+TLS cert) + per-service ticker scheduler
    incident/            # opens/resolves incidents from status transitions
    notify/              # Dispatcher + telegram/slack/email/webhook senders
    image/               # WebP validate/store/serve
    archive/             # export/import zip (validate → backup → apply)
    logger/              # slog JSON logger setup
    api/                 # Fiber server, middleware, all handlers
    web/                 # //go:embed dist (exposes web.FS())
web-ui/
  src/
    api/                 # typed REST client
    components/          # ui (shadcn-vue), layout, dashboard, services,
                         #   incidents, integrations, common
    composables/         # useServicesPolling
    stores/              # pinia: auth, services, incidents, integrations, settings, ui
    views/               # Dashboard, ServiceDetail, Incidents(+Detail), Integrations,
                         #   Resources, Settings, Setup, Login, NotFound
    lib/                 # icon-generator, image (canvas→WebP), prng, format
```

Import direction worth remembering: `monitor → incident → {db, notify}`.
`incident` must never import `monitor` (cycle) — see
[ARCHITECTURE.md §4](ARCHITECTURE.md#4-incident-lifecycle).

## Notes

- **SQLite** uses the pure-Go `modernc.org/sqlite` driver so the binary is fully
  static (`CGO_ENABLED=0`) and cross-compiles to `arm64` with no C toolchain.
- **Image optimization** happens client-side (Canvas → WebP) so the backend has
  no image dependencies; it only validates and stores the WebP bytes.
- **Config vs DB**: `config.yaml` is the source of truth for services/settings;
  `upmonitor.db` holds users, sessions and metrics history.
- **HTTP** uses [Fiber v3](https://gofiber.io); handlers return `fiber.NewError` for
  errors (rendered centrally as `{ "error": ... }`) and `c.JSON` for success.
- **Migrations** are goose `*.sql` files in `internal/db/migrations`, embedded and
  applied on startup (`sqlite3` dialect). Add a new numbered file for schema
  changes; never edit an applied one.
- **Logs** are structured `slog` JSON on stdout; set `UPMONITOR_LOG_LEVEL`
  (`debug|info|warn|error`) to change verbosity.
