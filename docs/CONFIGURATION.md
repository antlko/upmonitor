# Configuration reference

upmonitor is configured by a single `config.yaml` in your config directory
(`/config` in Docker). The UI reads and writes this file for you, but it is
plain YAML you can edit by hand — it is validated and re-loaded whenever the app
saves it.

**Not in this file** (they live in `upmonitor.db`, SQLite, alongside it): users,
sessions, metrics history, TLS snapshots, **incidents + comments**, and
**notification integrations** — integration secrets are kept out of the YAML on
purpose, since this file is served verbatim by the raw-config editor.

A ready-to-copy template is in [`config/config.example.yaml`](../config/config.example.yaml).

## Top level

```yaml
version: 1 # schema version (currently 1)
settings: { ... } # app-wide settings
services: [ ... ] # monitored services
```

## `settings`

| Key                    | Type            | Default | Description                                                     |
| ---------------------- | --------------- | ------- | --------------------------------------------------------------- |
| `public_dashboard`     | bool            | `false` | Expose a read-only dashboard at `/public` with no sign-in.      |
| `default_widget_mode`  | `icon`/`name`/`dashboard` | `name` | Widget mode used for newly added services.            |
| `theme`                | `dark`/`light`  | `dark`  | Default interface theme.                                        |
| `check.default_interval` | int (seconds) | `30`    | Fallback check interval when a service doesn't set one.         |
| `check.timeout`        | int (seconds)   | `10`    | Fallback request timeout.                                       |
| `check.retention_days` | int (days)      | `30`     | Metrics history is trimmed to this window (hourly).             |

## `services`

Each entry defines one monitored endpoint.

```yaml
- id: grafana # slug: lowercase letters, digits, hyphens. Also the image filename.
  name: Grafana # display name
  url: https://grafana.home.lab # http(s) URL to check
  icon: grafana.webp # optional; file under images/
  check:
    interval: 30 # seconds between checks (min 5)
    method: GET # HTTP method
    timeout: 10 # seconds before the check fails (min 1)
    expected_status: [200] # accepted status codes; empty ⇒ any 2xx is "online"
  widget:
    mode: dashboard # icon | name | dashboard
  layout: { x: 0, y: 0, w: 3, h: 4 } # grid position (12 columns) and size
```

### Field notes

- **`id`** must be unique and match `^[a-z0-9]+(-[a-z0-9]+)*$`. It's derived from
  the name when you add a service in the UI, and doubles as the image filename
  (`<id>.webp`) and the metrics key.
- **`expected_status`** — leave empty (or omit) to treat any `2xx` as online.
  Otherwise a check is "online" only when the response code is in the list.
- **`icon`** is optional. With no icon, the UI renders a generated procedural
  icon; uploading or generating one stores a `<id>.webp` and sets this field.

### `widget.mode`

How the card renders on the dashboard:

| Mode | Shows | Typical size |
| --- | --- | --- |
| `icon` | icon with a status badge | `2×2` |
| `name` | icon + name + URL (name wraps to 2 lines) | `2×2` |
| `dashboard` | status pill, response, uptime, sparkline | `3×4` |

Changeable in one click from the card's `⋯` menu (it rides along with the layout
endpoint), or in the add/edit service form.

### `layout` — the dashboard grid

`layout` is the card's place in a **12-column** grid; `h` is in row units of
68px. There is exactly **one layout per service, not one per screen size**:

```yaml
layout: { x: 0, y: 0, w: 3, h: 4 } # x/y = position, w/h = size in grid units
```

Notes that matter if you hand-edit or hack on this:

- The `y` key is written **quoted** (`"y"`) because YAML 1.1 reads a bare `y` as
  a boolean. This is intentional and round-trips — don't "fix" it.
- The dashboard is responsive (breakpoints at 1200/900/640/480px) and reflows to
  fewer columns on small screens, but that reflow is **not** saved. Only a
  deliberate drag or resize writes `layout` back, so opening the app on a phone
  won't rewrite your desktop arrangement. (Dragging *while* on a narrow screen
  does save those coordinates — there's only one layout to save into.)
- Overlapping or out-of-range values are tolerated: the grid compacts cards
  vertically on load.

## Status classification

A check is recorded as:

- **online** — response received within the timeout and the status matches
  `expected_status` (or is any `2xx` when the list is empty).
- **offline** — a network/DNS/TLS error, a timeout, or an unexpected status.
- **unknown** — no check has completed yet (e.g. just added). A check itself
  never *results* in `unknown`; it only ever records `online` or `offline`.

A change from `online` to `offline` opens an incident and notifies your
integrations; the reverse resolves it. There is no retry threshold — a single
failed check is enough. See
[ARCHITECTURE.md §4](ARCHITECTURE.md#4-incident-lifecycle).

## Editing by hand

You can edit `config.yaml` directly. Changes made through the UI are written
atomically (temp file + rename) and re-validated. If you edit the file while the
app is running, use **Settings → Configuration** (or restart) to reload it. An
invalid file is rejected with an error rather than applied.
