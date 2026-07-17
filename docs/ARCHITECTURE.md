# Architecture & behaviour

How upmonitor actually behaves at runtime: what owns which data, how a check
becomes an incident, how the dashboard grid is stored, how navigation and roles
work, and which forms exist. Written for someone (or some agent) about to change
this code.

Companion docs: [CONFIGURATION.md](CONFIGURATION.md) (every `config.yaml` field),
[API.md](API.md) (every endpoint), [DEVELOPMENT.md](DEVELOPMENT.md) (how to run it).

---

## 1. Where data lives

Four stores, each with a strict job. Putting data in the wrong one is the most
common design mistake here.

| Store | Path | Holds | Written by |
| --- | --- | --- | --- |
| `config.yaml` | `<config-dir>/config.yaml` | services, app settings, **widget mode + grid layout** | `config.Save` (atomic temp-file + rename) |
| SQLite | `<config-dir>/upmonitor.db` | users, sessions, checks history, `service_tls`, **incidents + comments**, **integrations + notification log** | `internal/db/*.go` |
| images | `<config-dir>/images/<id>.webp` | service icons | `internal/image` |
| `state.json` | outside the config dir | which config dir is active | `internal/state` |

**Rule of thumb:** user-authored configuration a human might hand-edit → YAML.
Machine-generated history, anything high-volume, and anything secret → SQLite.

Why the split matters:

- A service is deleted from `config.yaml`, but its incidents stay in SQLite.
  DTO conversion therefore resolves names defensively (`toIncidentDTO` falls back
  to `"(deleted service)"`). Deleting a service explicitly cascades:
  `handleDeleteService` calls `DeleteServiceHistory`, `DeleteServiceTLS` and
  `DeleteServiceIncidents`.
- Integration secrets are deliberately **not** in `config.yaml` — that file is
  exposed verbatim by `GET /api/config` (the raw YAML editor) and is what people
  screenshot and commit.

## 2. Changing config safely (copy-on-write)

Every mutation of `config.yaml` goes through `Server.updateConfig` in
`internal/api/server.go`. Under one write lock it: clones the config → applies
your mutation → validates → saves atomically → swaps the pointer in → re-syncs
the scheduler.

```go
err := s.updateConfig(func(cfg *config.Config) error {
    svc := cfg.Find(id)
    if svc == nil { return errServiceNotFound }
    svc.Name = in.Name
    return nil
})
```

Consequences to respect:

- Never mutate `s.cfg` directly — readers hold it without a lock via `s.config()`.
- If you add a field to `config.Service`, also handle it in `config/clone.go`,
  or the clone will alias the original slice/map and mutations will leak.
- A failed validation leaves the old config in place; nothing is half-applied.
- DB-backed features (incidents, integrations) do **not** use `updateConfig` —
  they query `s.conn()` directly.

## 3. The monitoring loop

`internal/monitor/scheduler.go`. One goroutine per service, each on its own
`interval` ticker. `Sync(services)` reconciles after any config change: unchanged
workers keep running, changed ones restart, removed ones stop (`sameCheck`
decides).

One `check()` does, in order:

1. Take a snapshot of `svc` and `prev := w.lastStatus` under the worker mutex.
2. Run the HTTP request with the service timeout.
3. **Bail if the worker was cancelled mid-flight** (`parent.Err() != nil`) —
   otherwise a config edit or shutdown would record a bogus `offline`.
4. `InsertCheck(...)` — append to history.
5. `storeTLS(...)` — upsert the cert snapshot (HTTPS only).
6. Update `w.lastStatus`.
7. `incident.OnTransition(prev, res.Status, ...)`.

`Check()` returns **only `online` or `offline`** — never `unknown`. `unknown` is
purely a "no data yet" state in the DB/DTO layer. So transitions are always
`online ↔ offline`.

Concurrency: `worker.svc` and `worker.lastStatus` are guarded by `worker.mu`,
because the ticker goroutine, `Sync` and a manual `CheckNow` can all touch them.
The workers *map* is guarded by `Scheduler.mu`.

## 4. Incident lifecycle

**The principle: an incident is derived from a status _transition_, never from
the status itself.** Nothing scans the DB looking for down services. The only
trigger is `prev != current` inside a worker, at the moment a check is recorded.

`internal/incident/incident.go` — `OnTransition` has exactly three branches:

| Transition | Effect |
| --- | --- |
| `prev == current` | no-op — the overwhelmingly common path, and why a service that stays down doesn't spawn an incident every tick |
| `online → offline` | `CreateIncident(source="auto", started_at=ts)` → fire `incident_start` |
| `offline → online` | `ResolveOngoingIncident(resolved_at=ts)` → fire `incident_resolve` (silent if nothing was open) |

### The invariant: one open incident per service

Enforced in the schema, not in Go — `migrations/00003_incidents.sql`:

```sql
CREATE UNIQUE INDEX idx_incidents_one_ongoing ON incidents (service_id) WHERE status = 'ongoing';
```

Any number of `resolved` rows, at most one `ongoing` per service. This is what
makes the race safe: a ticker check and a manual `CheckNow` can run at once, both
read the same `prev`, and both try to open an incident. The second `INSERT` hits
the index, `CreateIncident` returns `db.ErrOngoingExists`, and `OnTransition`
treats it as "someone already opened it" (Debug log, no error). Manual creation
hits the same wall and surfaces as a `400`.

### Surviving a restart

`lastStatus` lives in memory, and memory is empty after a restart. `InitialStatus`
seeds it from the DB when a worker starts:

- an open incident exists → start `offline` (we were mid-outage) → the next
  successful check closes it correctly;
- none → start `online` → a first failing check opens one, a first successful
  check doesn't "resolve" anything spurious.

Without this, a restart during an outage would strand the incident open forever.

### Manual incidents

Same table, `source = 'manual'` — for what the monitor can't see (planned
maintenance). Subject to the same one-open invariant. Admins can also edit
timestamps/title, resolve, and delete. Comments live in `incident_comments`
(`ON DELETE CASCADE`), author resolved by joining `users`; **any signed-in user
may comment**, all other mutations are admin-only.

### Retention asymmetry

The hourly retention loop trims **`checks` only** (plus expired sessions).
Incidents are never aged out — outage history outlives the metrics window. They
die only with their service, or by explicit delete.

### Known limitation: no flap protection

A single failed check opens an incident and fires notifications immediately.
There is no retry/confirmation threshold, so one network blip = noise. If you add
one, the natural place is a consecutive-failure counter on `worker` gating the
call to `OnTransition` — `internal/incident` itself would not need to change,
since it is already decoupled from *how* we decided the service is down.

### Why `incident` is its own package

`monitor` imports `incident`, so `incident` must never import `monitor` (import
cycle). Hence everything arrives as arguments:
`OnTransition(ctx, db, dispatcher, svc, prev, current, ts)`. It depends only on
`db` + `notify`, which also makes it trivially testable — tests pass `nil` as the
dispatcher.

## 5. Notifications

Fired from exactly two events: incident start and incident resolve. Nothing else
notifies.

`incident.fire()` builds a `notify.Message` and calls
`go dispatcher.Notify(...)` — **deliberately in a goroutine**, so a slow SMTP
server can't stall the check loop.

`internal/notify/dispatcher.go` then: loads enabled integrations → one goroutine
per integration → calls its `Sender` → writes a `notification_log` row for
**every** attempt (sent or failed). No retries.

Senders live one-per-file (`telegram.go`, `slack.go`, `email.go`, `webhook.go`)
and self-register via `init()` into the `senders` map. **To add a channel type:**
add the file with an `init()` registration, add the type to the CHECK constraint
in a *new* migration, add its secret keys to `secretFields` and required keys to
`requiredFields` in `handlers_integrations.go`, and add the form fields in
`IntegrationFormDialog.vue`.

### Secret handling

`secretFields` (in `handlers_integrations.go`) lists sensitive keys per type
(`botToken`, `webhookUrl`, `password`). Three rules:

1. **Never echoed.** `toIntegrationDTO` strips them and exposes only
   `secrets: { botToken: true }` so the UI can show a "configured" placeholder.
2. **Merge-on-omit.** On `PUT`, a blank/absent secret keeps the stored value —
   the client can't resend what it can't read.
3. **Included in exports.** A backup without secrets can't restore working
   integrations; the export endpoint is already admin-only.

## 6. Dashboard grid & layout persistence

The grid is `grid-layout-plus` in `ServiceGrid.vue`: 12 columns, row height 68px,
responsive breakpoints `lg/md/sm/xs/xxs` at `1200/900/640/480/0`.

**There is exactly one layout, stored per service in `config.yaml`** as
`layout: {x, y, w, h}` — *not* one layout per breakpoint:

```yaml
layout: { x: 0, y: 0, w: 3, h: 4 }
```

(The `y` key serialises quoted — `"y"` — because yaml.v3 reads bare `y` as a
boolean. That's correct and round-trips; don't "fix" it.)

**The trap:** persist only on a real user drag/resize — `GridItem`'s `@moved` /
`@resized`. Never persist `GridLayout`'s `@layout-updated`, because it *also*
fires when the grid reflows itself for a narrower breakpoint. Wiring saves to it
means opening the app on a laptop silently rewrites the stored desktop layout.
(This was a real bug; the whole arrangement collapsed to `w:2` after one window
resize.)

Corollary that still stands: dragging cards *while on a narrow screen* saves
those narrow coordinates as the one true layout. Fixing that properly needs
per-breakpoint layouts — a schema change.

Saving flows `ServiceGrid` → `services.saveLayout()` → `PATCH /api/services/layout`
(bulk, `[{id,x,y,w,h,mode?,chart?}]`). The same endpoint carries `mode` and
`chart`, which is why `services.setWidgetMode()` / `setChartType()` can reuse it.
Both are only applied server-side when non-empty, so a drag-save doesn't reset
them — but `x/y/w/h` are assigned unconditionally, so those two actions must
re-send the service's current position or they'd zero the layout.

**Dragging must not select text.** interact.js (under `grid-layout-plus`)
deliberately skips `preventDefault` on pointerdown and expects CSS to suppress
selection; the library's own `user-select: none` only lands on
`.vgl-item--dragging`, by which point the browser has anchored a selection — and
never on the neighbours the pointer sweeps across. `ServiceGrid` therefore sets
`select-none` on the whole grid whenever it's interactive (not in `readonly`,
where nothing is draggable and text stays selectable).

### Widget modes

`widget.mode` per service: `icon` (icon + status badge), `name` (icon + name +
URL), `dashboard` (mini dashboard with status pill, response, uptime, sparkline).
Rendered by the three branches of `ServiceCard.vue`.

Card interaction model — three distinct targets, don't conflate them:

- **card body click** → in-app detail page (`/services/:id`), gated on the
  `linkable` prop, with a 4px drag threshold so ending a drag doesn't navigate;
- **↗ hover button** → the external monitored site (available to read-only users
  too);
- **⋯ hover menu** → open / widget mode / chart style / edit / replace image /
  generate icon / delete (admin only).

Because the actions overlay the top-right corner, `dashboard` mode reserves
horizontal room (`headerPad`) so a long name truncates *before* reaching them.
`name` mode needs no reserve — its content is vertically centred, clear of the
buttons — and lets the name wrap to two lines (`line-clamp-2`) instead of
truncating.

## 7. Navigation, roles & guards

Two roles only: `admin` and `readonly` (a CHECK constraint on `users.role`).

| Path | Name | Access |
| --- | --- | --- |
| `/` | `dashboard` | any signed-in user |
| `/services/:id` | `service-detail` | any signed-in user |
| `/incidents` | `incidents` | any signed-in user (**anyone may comment**) |
| `/incidents/:id` | `incident-detail` | any signed-in user |
| `/resources` | `resources` | **admin** |
| `/integrations` | `integrations` | **admin** |
| `/settings` | `settings` | **admin** |
| `/setup` | `setup` | first-run only (`meta.bare`) |
| `/login` | `login` | anonymous (`meta.bare`) |
| `/:pathMatch(.*)*` | `not-found` | — (`meta.bare`) |

`meta.bare` renders without the sidebar/topbar shell.

The guard order in `router/index.ts` matters: bootstrap auth → force `/setup` if
`needsSetup` → bounce `/setup` once set up → force `/login` if anonymous →
bounce `/login` if signed in → block `readonly` from `adminOnly`.

**Adding a page needs three edits in sync:** the route, the `adminOnly` set (if
restricted), and the nav entry in `AppSidebar.vue`. Route-level gating is UX
only — **the API must enforce it independently** via `auth, admin` middleware.

`/incidents` being open to `readonly` while its mutations are admin-gated is a
deliberate split: the guard lets everyone in, and the page hides
create/edit/resolve/delete behind `auth.isAdmin`.

## 8. Forms (what exists to fill in)

| Form | File | Submits |
| --- | --- | --- |
| First-run setup | `SetupView.vue` | `POST /api/setup` — first admin |
| Sign in | `LoginView.vue` | `POST /api/auth/login` |
| Add/edit service | `services/ServiceFormDialog.vue` | name, url, interval, widget mode → `POST`/`PUT /api/services` |
| Generate icon | `services/IconGeneratorDialog.vue` | procedural SVG → rasterised → icon upload |
| Create/edit incident | `incidents/IncidentFormDialog.vue` | service, title, startedAt, resolvedAt (`datetime-local` ⇄ RFC3339) |
| Comment composer | `IncidentDetailView.vue` | `POST /api/incidents/:id/comments` |
| Add/edit integration | `integrations/IntegrationFormDialog.vue` | type-conditional fields; secrets blank = keep |
| Settings | `SettingsView.vue` | settings, users, config path, raw `config.yaml` editor |
| Confirm | `common/ConfirmDialog.vue` | generic destructive confirmation |

Dialog convention: `props { open, <entity>? }`, emits `update:open` + `submit`,
reset state on open via `watch(() => props.open, …)`, and let the **parent** call
the store and toast. Presence of `<entity>` means edit mode.

## 9. Images

Optimisation is **client-side on purpose** (`src/lib/image.ts`, Canvas → WebP):
the backend only checks magic bytes and stores, which keeps the binary CGO-free.
Do not add server-side image codecs.

Uploads are a **raw `image/webp` body**, not multipart. Three entry points, all
converging on the same `optimizeToWebP → services.uploadIcon` pipeline: the ⋯
menu's file picker, drag-and-drop onto a card, and Ctrl+V paste (targets the
hovered card via `hoveredServiceId`; ignored while a text field is focused).

With no icon, `ServiceIcon.vue` renders a seeded procedural SVG
(`src/lib/icon-generator.ts`) — deliberately no AI model.

## 10. TLS certificate info

Read from the TLS handshake the HTTPS check already performs
(`resp.TLS.PeerCertificates[0]`) — no extra connection, no WHOIS/RDAP (domain
registration expiry is explicitly out of scope).

Stored as **one upserted row per service** (`service_tls`), not per check: a cert
is stable for weeks, so per-check rows would be pure redundancy.

A failed handshake means there's no `resp` at all, so no cert can be read; the
check error is stored instead (with zero timestamps) so the UI can still show
that something is wrong. Plain-HTTP services are skipped entirely and the detail
page shows "Not applicable".

## 11. Export / import

`GET /api/config/export` zips `config.yaml` + `images/` + `incidents.json` +
`integrations.json`. Import replaces all of it, after snapshotting the current
state to `backups/`.

Two behaviours worth knowing:

- **Missing bundles are skipped, not treated as empty.** An older archive without
  `incidents.json` leaves existing incidents alone rather than wiping them.
- **IDs are preserved** so `incident_comments.incident_id` stays valid, and user
  references that don't exist in the target DB are nulled (users aren't part of
  the archive), so foreign keys hold across instances.

## 12. Where to make a change

| Goal | Touch |
| --- | --- |
| New API endpoint | route in `api/server.go` → handler in `api/handlers_*.go` → DTO in `api/dto.go` → client in `web-ui/src/api/index.ts` → type in `web-ui/src/types/index.ts` → document in [API.md](API.md) |
| New DB table/column | new `db/migrations/NNNNN_*.sql` (never edit an applied one) → queries in `db/<table>.go` |
| New config field | `config/config.go` struct + `Default()` + `normalize()` + `Validate()` → `config/clone.go` **only if the field is a reference type** (slice/map/pointer — value types ride along in `Clone`'s `copy`) → [CONFIGURATION.md](CONFIGURATION.md) |
| New per-service preference | follow `widget.mode` / `chart.type`: config field → `serviceDTO` → `layoutItem` + an `if it.X != ""` guard in `handleUpdateLayout` → a store action that re-sends the current `x/y/w/h` (the handler assigns layout unconditionally) → card ⋯ menu |
| Change the response-time chart | `services/ResponseTimeChart.vue` (detail) / `dashboard/SparklineChart.vue` (card) — both hand-rolled SVG, no chart library. Bucketing is server-side: `api.chooseBucket` + `db.SeriesFor` |
| Change monitoring/incident behaviour | `monitor/scheduler.go` (when) / `incident/incident.go` (what) |
| New notification channel | see §5 |
| New page | route + `adminOnly` + `AppSidebar.vue` nav + API-side `auth, admin` |
| Change card layout/behaviour | `dashboard/ServiceCard.vue` (highest-churn file) |

## 13. Testing

Backend uses stdlib `testing`, table-driven, with `httptest` for HTTP senders and
a temp-file SQLite DB per test (`db.openTestDB` / `db.Open(t.TempDir()+…)`).
Tests live beside the package. There is **no frontend test framework** — verify
UI changes by running the dev servers (see [DEVELOPMENT.md](DEVELOPMENT.md)).

Worth knowing when testing incidents: `OnTransition` accepts a `nil` dispatcher,
so incident logic is testable without any notification plumbing.
