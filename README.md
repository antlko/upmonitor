<div align="center">

# upmonitor

### Simple, self‑hosted service monitoring — beautiful by default.

A modern, minimal dashboard for keeping an eye on your web services. Add a URL,
watch its status, response time and uptime in real time, and rearrange
everything with drag‑and‑drop. One tiny container, one config file, zero fuss.

[![Docker image](https://img.shields.io/badge/ghcr.io-antlko%2Fupmonitor-2496ED?logo=docker&logoColor=white)](https://github.com/antlko/upmonitor/pkgs/container/upmonitor)
[![Made with Go + Vue](https://img.shields.io/badge/stack-Go%20%2B%20Vue%203-00ADD8?logo=go&logoColor=white)](#how-it-works)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

<!-- Add a screenshot at docs/screenshot.png to show it off here. -->
<!-- ![upmonitor dashboard](docs/screenshot.png) -->

</div>

---

## Why upmonitor?

Most self‑hosted monitors are powerful but look like enterprise control panels.
upmonitor is deliberately small and good‑looking — think Linear/Vercel, not a
grafana clone — while still doing the real work:

- 🟢 **Live status** — HTTP health checks with online / offline / unknown states,
  colored borders and a pulsing indicator.
- 🧩 **Drag‑and‑drop dashboard** — resize widgets and rearrange them; positions
  are saved to `config.yaml`.
- 🎛️ **Three widget modes** — icon only, icon + name, or a mini dashboard with
  response time, uptime and a sparkline. Switch mode in one click from the card.
- 🔔 **Incidents that log themselves** — when a service goes down an incident
  opens automatically and closes when it recovers, with start/end times and a
  duration. Add your own for planned work, and comment to keep the team in sync.
- 📣 **Get told about it** — send incidents to **Telegram, Slack, email or any
  webhook**. Add a channel, hit *Send test*, done.
- 🔍 **Per‑service detail** — uptime over 7/30/365 days, a response‑time chart,
  recent incidents, and **SSL certificate issuer and expiry** with a warning as
  the date approaches.
- 📈 **Metrics that matter** — uptime %, response time, error count and last
  success, kept for 30 days (configurable) in SQLite and trimmed automatically.
- 🎨 **Instant icons** — generate a crisp, unique icon for any service on‑device
  (no external AI), or upload your own — auto‑optimized to WebP in your browser.
- 👥 **Share it** — invite friends as **admin** or **read‑only**, or flip on a
  public read‑only status page.
- 💾 **Portable** — one YAML file plus images, incidents and integrations;
  export/import as a `.zip` with an automatic backup on restore.
- 🌗 **Dark mode first**, fully responsive, and one small binary that serves both
  the API and the UI.

---

## Quick start

upmonitor ships as a **single container**. Pick whichever route you like — each
block is copy‑paste ready.

### Option A — `docker run` (one line)

```bash
docker run -d \
  --name upmonitor \
  -p 8080:8080 \
  -v upmonitor-config:/config \
  --restart unless-stopped \
  ghcr.io/antlko/upmonitor:latest
```

Open **http://localhost:8080** and create your admin account.

### Option B — Docker Compose

Save this as `docker-compose.yml` and run `docker compose up -d`:

```yaml
services:
  upmonitor:
    image: ghcr.io/antlko/upmonitor:latest
    container_name: upmonitor
    ports:
      - "8080:8080"
    volumes:
      - upmonitor-config:/config
    restart: unless-stopped

volumes:
  upmonitor-config:
```

### Option C — Portainer (stack)

1. **Stacks → Add stack**, give it a name (e.g. `upmonitor`).
2. Paste the Compose file from Option B into the **Web editor**.
3. Click **Deploy the stack**.
4. Browse to `http://<your-server>:8080` and finish setup.

That's it — nothing to build.

### Option D — Build from source

No published image needed; build it yourself:

```bash
git clone https://github.com/antlko/upmonitor.git
cd upmonitor
docker compose up -d --build   # uncomment `build: .` in docker-compose.yml first
# ...or build the image directly:
docker build -t upmonitor .
docker run -d -p 8080:8080 -v upmonitor-config:/config upmonitor
```

---

## First run

On first launch you'll be asked to **create an administrator password** — it's
hashed with bcrypt and stored locally in SQLite; it never leaves your server.
After that:

1. **Add a service** — name + URL (e.g. `https://grafana.home.lab`).
2. Watch it turn green as the first health check runs.
3. Switch its widget mode, drag it around, generate an icon.
4. Invite people or enable the public dashboard from **Settings**.

---

## Configuration

Everything lives in one **config directory** (mounted at `/config` in Docker):

```
/config
├── config.yaml       # services + settings (managed by the UI, editable by hand)
├── images/           # service icons (<id>.webp)
├── backups/          # automatic snapshots created before an import
└── upmonitor.db      # users, history, incidents, integrations (SQLite)
```

The database schema is created and upgraded **automatically on start** — there is
no migration step to run when you update the image.

### Ports & volumes

| What      | Value                     | Notes                                  |
| --------- | ------------------------- | -------------------------------------- |
| HTTP port | `8080`                    | change the left side of `-p 8080:8080` |
| Config    | volume mounted at `/config` | holds config, images, database, backups |

### Environment variables

| Variable                | Default   | Description                                  |
| ----------------------- | --------- | -------------------------------------------- |
| `UPMONITOR_CONFIG_DIR`  | `/config` | Where config, images and the database live.  |
| `UPMONITOR_ADDR`        | `:8080`   | Listen address (`host:port`).                |
| `UPMONITOR_LOG_LEVEL`   | `info`    | Log verbosity: `debug`, `info`, `warn`, `error`. |

Command‑line flags mirror these: `--config-dir` and `--addr`.

A commented reference config lives at
[`config/config.example.yaml`](config/config.example.yaml); full field docs in
[`docs/CONFIGURATION.md`](docs/CONFIGURATION.md).

### Backup & restore

**Settings → Configuration → Export** downloads a `.zip` containing
`config.yaml` and your images. **Import** validates the archive, snapshots your
current config into `backups/`, then applies the new one — so a restore is
always safe.

### Roles & public access

- **Admin** — full control (services, users, settings).
- **Read‑only** — can view the dashboard and metrics, nothing else.
- **Public dashboard** — when enabled in Settings, anyone can view a read‑only
  board at **`/public`** without signing in.

---

## How it works

One static Go binary embeds the built Vue app and serves both the API and the UI.

- **Backend** — Go with [Fiber](https://gofiber.io), pure‑Go SQLite
  ([modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), no CGO) with
  [goose](https://github.com/pressly/goose) migrations, a goroutine‑per‑service
  scheduler for checks, bcrypt + cookie sessions, and structured `slog` logging.
- **Frontend** — Vue 3 + TypeScript, Pinia, Tailwind CSS v4 and shadcn‑vue
  components, `grid-layout-plus` for the draggable grid.
- **Config** — `config.yaml` is the source of truth for services and settings;
  users, sessions and metrics live in SQLite.

The result is tiny, fast and easy to run on anything from a NAS to a Raspberry Pi
(images are built for both `amd64` and `arm64`).

## Local development

See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) to run the Vue dev server and Go
backend together, [`docs/API.md`](docs/API.md) for the REST API, and
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for how it behaves under the hood
(incidents, notifications, the dashboard grid, roles).

## License

[MIT](LICENSE)
