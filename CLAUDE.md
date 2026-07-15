# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**upmonitor** — "Simple self-hosted service monitoring." A full-stack app: a **Go backend**
(`backend/`) serves a **Vue 3 SPA** (`web-ui/`) plus a JSON API and runs the monitoring worker. In
production the built SPA is embedded into a single static Go binary (one Docker container). Users add
web services, watch live status/response/uptime on a drag-and-drop dashboard, get incidents opened
automatically on outages with notifications, and manage everything via `config.yaml`.

## Read this first

This file is the **index**: commands, invariants and gotchas. The deep detail lives in `docs/` —
follow the pointer instead of re-deriving behaviour from the code.

| Doc | Read it when you need |
| --- | --- |
| **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** | **How it behaves**: data ownership, config copy-on-write, monitoring loop, **incident lifecycle**, notifications, **grid/layout persistence**, navigation+roles, forms inventory, images, export/import, "where to make a change" map |
| [docs/API.md](docs/API.md) | Every endpoint + object shape (services, metrics, incidents, integrations) |
| [docs/CONFIGURATION.md](docs/CONFIGURATION.md) | Every `config.yaml` field, `widget.mode`, `layout` grid semantics |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Running both dev servers, building the single binary |
| [README.md](README.md) | User-facing install/usage |

**When you change behaviour, update the matching doc in the same change** — these files are the
contract future sessions rely on.

## Working directories

Two toolchains: run `npm` from `web-ui/`, run `go` from `backend/`. In dev they run side by side
(Vite proxies `/api` + `/images` to the Go server on `:8080`, per `web-ui/vite.config.ts`).

## Commands

**Frontend** (from `web-ui/`): `npm run dev` · `npm run type-check` · `npm run lint` (oxlint→eslint,
both `--fix`) · `npm run build` (type-check + build). **Backend** (from `backend/`): `go run
./cmd/upmonitor --config-dir ../config` · `go build ./...` · `go vet ./...`. **Single binary**: build
SPA → `cp -r web-ui/dist backend/internal/web/dist` → `CGO_ENABLED=0 go build ./cmd/upmonitor`
(the `Dockerfile` automates this). **Tests**: backend uses stdlib `testing` (`go test ./...`) —
table-driven, `httptest` for HTTP senders, temp-file SQLite per test; add new ones beside the package.
**No test framework is wired on the frontend** — verify UI changes by running the dev servers, don't
invent a test command.

## Architecture

- **Source of truth split:** `config.yaml` holds services + settings incl. widget mode and grid layout
  (the app rewrites it atomically); **SQLite** (`upmonitor.db`) holds users, sessions, checks history,
  TLS snapshots, incidents + comments, and integrations + notification log. Both live in the config dir
  (`/config` in Docker; `UPMONITOR_CONFIG_DIR` / `--config-dir`, default `./config`). Hand-editable
  config → YAML; history, high-volume and **secrets** → SQLite.
- **Backend packages** (`backend/internal/`): `state` (tiny `state.json` outside the config dir, recording
  which config dir is active — lets a custom config-dir setting survive restarts), `config` (yaml load/save/validate), `db` (modernc pure-Go
  SQLite — keep `CGO_ENABLED=0`; schema via **goose** migrations in `db/migrations/*.sql`, embedded and
  run on `Open`), `auth` (bcrypt + session tokens; cookies are set by the api layer), `monitor` (HTTP
  checker + goroutine-per-service ticker scheduler; `Sync` reconciles on config change; also reads the
  TLS cert off HTTPS responses and feeds status transitions to `incident`), `incident` (opens/resolves
  outage incidents from online↔offline transitions; imports `db`+`notify`, never `monitor`, to avoid a
  cycle), `notify` (incident-start/resolve notifications; a `Dispatcher` fans out to per-type `Sender`s
  — telegram/slack/email/webhook — registered via `init()`), `image` (WebP
  validate/store), `archive` (export/import zip), `logger` (slog JSON setup), `api` (the `Server` owns
  config+db+scheduler and applies edits copy-on-write under a lock; **Fiber v3** app with per-route
  auth/admin middleware, a central `ErrorHandler` rendering `{ "error": ... }`, and a native SPA
  catch-all serving the embedded FS), `web` (`//go:embed dist`, exposes `web.FS()`). Entry:
  `cmd/upmonitor/main.go` (inits slog, `app.Listen`, graceful `app.ShutdownWithContext`).
- **Frontend:** Vue 3 `<script setup>`, Pinia stores (`auth`, `services`, `incidents`, `integrations`,
  `settings`, `ui`), typed API client in `src/api/`, `useServicesPolling` (~10s) drives live updates.
  Router guards gate setup/login/role. `/incidents` is open to any signed-in user (anyone may comment);
  mutations inside it are admin-gated. `/integrations` is admin-only. Routes, roles and the full form
  inventory: [ARCHITECTURE.md §7–8](docs/ARCHITECTURE.md#7-navigation-roles--guards).

## Invariants (breaking these causes real bugs)

- **An incident is derived from a status _transition_, never from the status itself.** Only
  `prev != current` inside a worker creates/resolves one — nothing scans for down services. At most
  one `ongoing` incident per service, enforced by a **partial unique index**, which is what makes the
  ticker-vs-`CheckNow` race safe. `lastStatus` is in-memory and re-seeded from the DB on start
  (`InitialStatus`) so restarts mid-outage don't strand incidents. Full flow:
  [ARCHITECTURE.md §4](docs/ARCHITECTURE.md#4-incident-lifecycle).
- **`config.yaml` is mutated only via `Server.updateConfig`** (clone → mutate → validate → atomic save
  → swap → `Sync`). Never touch `s.cfg` directly; add new `Service` fields to `config/clone.go` too.
  DB-backed features use `s.conn()` instead.
- **Integration secrets never leave the server**: stripped from DTOs (only a `secrets: {k: true}` flag
  is exposed) and preserved on `PUT` when blank/omitted. They *are* included in export archives on
  purpose — a secret-less backup can't restore working channels.
- **Route guards are UX only** — the API must enforce roles independently (`auth, admin` middleware).
- **Check results are only `online`/`offline`**; `unknown` means "no data yet" and never comes from a
  check.

## Conventions that bite

- **Icons: `@lucide/vue`**, NOT `lucide-vue-next` (deprecated). Some names changed in v1 (e.g.
  `Home`→`House`) and some don't exist (there is no `Slack`); verify via type-check.
- **Tailwind v4 resets buttons to `cursor: default`.** Interactive elements need an explicit
  `cursor-pointer` — it's already in the `buttonVariants` base, `TabsTrigger`, `SelectTrigger` and
  `DropdownMenuItem`, so reuse those rather than hand-rolling a bare `<button>`.
- **UI = shadcn-vue** (Reka UI v2 + Tailwind v4). Primitives in `src/components/ui` are intentionally
  single-word; `eslint.config.ts` disables `vue/multi-word-component-names` there and allows the
  `const { class: _, ...rest }` omit pattern (`varsIgnorePattern: ^_`).
- **Image optimization is client-side** (Canvas → WebP in `src/lib/image.ts`); the backend only
  validates WebP magic bytes and stores. Do not add server-side image codecs (keeps the binary CGO-free).
- **Icon generation is procedural** (`src/lib/icon-generator.ts`, seeded SVG) — deliberately no AI model.
- Config `layout` YAML serializes the `y` key quoted (`"y"`) because yaml.v3 treats bare `y` as a
  boolean; this is valid and round-trips — don't "fix" it.
- **Dashboard layout saves only on a user drag/resize** (`GridItem` `@moved`/`@resized` in
  `ServiceGrid.vue`). Never persist from `GridLayout`'s `@layout-updated`: it also fires when the grid
  reflows for a narrower breakpoint, so merely resizing the window would overwrite the stored desktop
  layout (there is one layout in config.yaml, not one per breakpoint).
- Frontend types in `src/types` intentionally mirror the API DTOs in `backend/internal/api/dto.go`
  (camelCase); keep them in sync when changing shapes.
- **Fiber handlers** return `fiber.NewError(code, msg)` for errors (the central `errorHandler` renders
  them as `{ "error": msg }` — the shape the frontend expects); success via `c.JSON`/`c.Status().JSON`.
  Read the authed user with `userLocal(c)`, params with `c.Params`, body with `decode(c, &dst)`.
- **Migrations**: add a new `backend/internal/db/migrations/NNNNN_name.sql` with `-- +goose Up`/`Down`;
  never edit an already-applied migration. Dialect is `sqlite3`. They are **embedded and run
  automatically inside `db.Open`** on every start (goose skips applied ones) — there is no separate
  migrate command, and the distroless image has no shell to run one. Never add a manual migration step.
- **Defaults live in one place.** e.g. `config.DefaultRetentionDays` is exported precisely so the api
  fallbacks can't drift from what new configs get. Don't re-hardcode a default next to a use site.
- **Existing configs keep their stored values.** Changing a default only affects new installs and
  unset fields (`normalize()` fills zeros only); never silently rewrite a user's `config.yaml`.
- **Logging** is slog JSON to stdout; prefer `slog.InfoContext(c.Context(), …)` with key/value attrs.
  Level via `UPMONITOR_LOG_LEVEL` (debug|info|warn|error).
