# REST API

All endpoints are JSON over HTTP, same-origin, authenticated with an
`HttpOnly` session cookie (set on login/setup). Write endpoints require the
**admin** role; read endpoints require any authenticated user unless noted.

Errors use `{ "error": "message" }` with an appropriate status code.

## Setup & auth

| Method & path            | Auth   | Description                                        |
| ------------------------ | ------ | -------------------------------------------------- |
| `GET /api/setup/status`  | none   | `{ "needsSetup": bool }` — is first-run setup due? |
| `POST /api/setup`        | none¹  | Create the first admin `{ username, password }`.   |
| `POST /api/auth/login`   | none   | Log in `{ username, password }`; sets the cookie.  |
| `POST /api/auth/logout`  | none   | Clear the session.                                 |
| `GET /api/auth/me`       | user   | The current user.                                  |

¹ Only succeeds while no users exist.

## Services & metrics

| Method & path                    | Auth  | Description                                            |
| -------------------------------- | ----- | ----------------------------------------------------- |
| `GET /api/services`              | user  | All services with live status + metrics.              |
| `POST /api/services`             | admin | Add `{ name, url, interval, mode }`.                  |
| `PUT /api/services/{id}`         | admin | Edit `{ name?, url?, interval?, mode? }`.             |
| `DELETE /api/services/{id}`      | admin | Remove the service, its image and history.            |
| `PATCH /api/services/layout`     | admin | Bulk-save grid layout `[{ id, x, y, w, h, mode?, chart? }]`. `mode`/`chart` apply only when set; `x/y/w/h` always do. |
| `POST /api/services/{id}/check`  | admin | Run a check immediately; returns updated metrics.     |
| `GET /api/services/{id}/metrics` | user  | Aggregates, time series, uptime windows + TLS (`?range=1h\|6h\|24h\|7d\|30d\|365d`, default `24h`; unknown values fall back to `24h`). |
| `POST /api/services/{id}/image`  | admin | Upload a WebP icon (raw `image/webp` body).           |
| `DELETE /api/services/{id}/image`| admin | Remove the icon.                                      |

## Incidents

Incidents are opened and resolved automatically from `online↔offline`
transitions; these endpoints cover reading them plus the manual/edit path. See
[ARCHITECTURE.md §4](ARCHITECTURE.md#4-incident-lifecycle) for the lifecycle.

| Method & path                       | Auth  | Description                                                        |
| ----------------------------------- | ----- | ------------------------------------------------------------------ |
| `GET /api/incidents`                | user  | List, newest first (capped at 500). Filters: `?status=ongoing\|resolved`, `?serviceId=`. |
| `GET /api/incidents/{id}`           | user  | One incident **with its comments**.                                |
| `POST /api/incidents`               | admin | Log one manually `{ serviceId, title?, startedAt?, resolvedAt? }`.¹ |
| `PUT /api/incidents/{id}`           | admin | Edit `{ title?, startedAt?, resolvedAt? }`; setting `resolvedAt` resolves it.² |
| `DELETE /api/incidents/{id}`        | admin | Delete the incident (comments cascade).                            |
| `POST /api/incidents/{id}/comments` | user³ | Add a comment `{ body }`.                                          |

¹ `startedAt` defaults to now. Returns `400` if the service already has an
ongoing incident (at most one per service) or the `serviceId` is unknown.
² Omitting a field leaves it unchanged. Timestamps are RFC3339.
³ **Any** signed-in user may comment — this is the one incident write that is not
admin-gated.

## Integrations

Notification channels. Fire on incident start and resolve only.

| Method & path                        | Auth  | Description                                              |
| ------------------------------------ | ----- | -------------------------------------------------------- |
| `GET /api/integrations`              | admin | List channels — **secrets are never returned**.          |
| `POST /api/integrations`             | admin | Create `{ type, name, enabled, config }`.                |
| `PUT /api/integrations/{id}`         | admin | Update; blank/omitted secrets keep their stored value.   |
| `DELETE /api/integrations/{id}`      | admin | Delete (its notification log cascades).                  |
| `POST /api/integrations/{id}/test`   | admin | Send one real test message. Always `200`; see below.     |

`type` is `telegram` | `slack` | `email` | `webhook`. The `config` shape depends
on it:

```jsonc
{ "botToken": "123:ABC", "chatId": "-100123" }                       // telegram
{ "webhookUrl": "https://hooks.slack.com/services/…" }               // slack
{ "host": "smtp.example.com", "port": 587, "username": "u",          // email
  "password": "p", "from": "alerts@x.com", "to": "ops@x.com" }       //   (STARTTLS; port 465 unsupported)
{ "url": "https://…", "method": "POST", "headers": { "X-K": "v" },   // webhook
  "bodyTemplate": "{{.ServiceName}} is {{.Event}}" }                 //   (blank ⇒ default JSON payload)
```

The test endpoint reports delivery in the body rather than via a status code, so
the UI can show the real reason:

```jsonc
{ "ok": true }
{ "ok": false, "error": "unexpected status 401: {\"description\":\"Unauthorized\"}" }
```

## Settings, config & users

| Method & path                    | Auth  | Description                                     |
| -------------------------------- | ----- | ---------------------------------------------- |
| `GET /api/settings`              | user  | Current settings + active config directory.    |
| `PUT /api/settings`              | admin | Update settings.                               |
| `PUT /api/settings/config-path`  | admin | Switch config directory `{ path }` (reloads).  |
| `GET /api/config`                | admin | Raw `config.yaml` text `{ content }`.          |
| `PUT /api/config`                | admin | Replace `config.yaml` from text `{ content }`. |
| `GET /api/users`                 | admin | List accounts.                                 |
| `POST /api/users`                | admin | Create `{ username, password, role }`.         |
| `PUT /api/users/{id}`            | admin | Update `{ role?, password? }`.                 |
| `DELETE /api/users/{id}`         | admin | Delete (blocks self and last admin).           |

## Backup / restore

| Method & path              | Auth  | Description                                       |
| -------------------------- | ----- | ------------------------------------------------ |
| `GET /api/config/export`   | admin | Download `backup.zip` — config.yaml, images/, `incidents.json`, `integrations.json`.¹ |
| `POST /api/config/import`  | admin | Apply a `.zip` (raw `application/zip` body).²    |

¹ Integration **secrets are included in plaintext** — a backup without them
couldn't restore working channels. Store the file accordingly.
² Snapshots the current state to `backups/` first. Archives lacking
`incidents.json` / `integrations.json` (older exports) leave that data untouched
rather than wiping it.

## Static

| Method & path        | Auth | Description          |
| -------------------- | ---- | -------------------- |
| `GET /images/{file}` | user | Serve a stored icon. |

## Service object

```jsonc
{
  "id": "grafana",
  "name": "Grafana",
  "url": "https://grafana.home.lab",
  "icon": "/images/grafana.webp",      // or null
  "check": { "interval": 30, "method": "GET", "timeout": 10, "expectedStatus": [200] },
  "widget": { "mode": "dashboard" },
  "chart": { "type": "line" },          // line | bars
  "layout": { "x": 0, "y": 0, "w": 3, "h": 4 },
  "status": "online",                   // online | offline | unknown
  "latencyMs": 118,                     // or null
  "uptime": 99.98,                      // percent over the retention window
  "errorCount": 0,
  "lastCheck": "2026-07-14T12:00:00Z",  // or null
  "lastSuccess": "2026-07-14T12:00:00Z",// or null
  "latencyHistory": [120, 118, null, 121] // sparkline; null ⇒ that check was offline
}
```

`latencyHistory` is chronological and includes failed checks as `null` — an
offline check may well have a latency (a fast `500`), but charting it would draw
a healthy line through a failure, so only successful checks carry a number.

## Metrics object

`GET /api/services/{id}/metrics` returns the service object above **plus**:

```jsonc
{
  "series": [                            // bucketed over [from, to], ≤96 points
    { "ts": 1784100000, "avgLatency": 118.5, "errors": 0 },
    { "ts": 1784100900, "avgLatency": null, "errors": 30 }  // down for the whole bucket
  ],
  "from": 1784013600,                    // requested window (unix seconds) —
  "to": 1784100000,                      //   the chart's x-domain
  "bucketSeconds": 900,                  // width of one series bucket
  "uptimeWindows": {                     // fixed windows, independent of ?range
    "days7": 100, "days30": 99.98, "days365": 99.98
  },
  "tls": {                               // null for http:// services
    "checkedAt": "2026-07-15T10:46:24Z",
    "validFrom": "2026-05-31T21:39:12Z", // null if no cert was read
    "validUntil": "2026-08-29T21:41:26Z",
    "issuer": "Cloudflare TLS Issuing ECC CA 3",
    "subject": "example.com",
    "daysLeft": 45,                      // null if no cert was read
    "error": ""                          // set when the handshake failed
  }
}
```

`series` describes only what was actually checked, and both of its "no data"
shapes are meaningful:

- **`avgLatency: null`** — checks ran but none succeeded: the service was down
  for that whole bucket. (An offline check may still have a latency, e.g. a fast
  `500`; it is excluded on purpose so the line means *response time when up*.)
- **a missing bucket** — no check ran at all (app restart, service added
  mid-window, history trimmed). Consecutive points are therefore not necessarily
  adjacent: a gap larger than `bucketSeconds` is real.

Clients must break the line on both rather than joining across them.

`bucketSeconds` is chosen server-side from the span **and** the service's check
interval, so it is at least the interval — a bucket narrower than the interval
would leave most buckets empty. It always divides the window exactly. Typical
values at the default 30s interval: `1h`→60s, `6h`→300s, `24h`→900s, `7d`→7200s,
`30d`→28800s.

`uptimeWindows` can only see stored history, so they are bounded by
`check.retention_days` (default 30) — `days365` will read low until enough
history exists.

## Incident object

```jsonc
{
  "id": 1,
  "serviceId": "grafana",
  "serviceName": "Grafana",      // "(deleted service)" if it's gone from config.yaml
  "status": "ongoing",           // ongoing | resolved
  "source": "auto",              // auto (from a transition) | manual
  "title": "",                   // free text; usually empty for auto incidents
  "startedAt": "2026-07-15T10:45:24Z",
  "resolvedAt": null,            // null while ongoing
  "createdBy": null,             // user id for manual incidents
  "createdAt": "2026-07-15T10:45:24Z",
  "updatedAt": "2026-07-15T10:45:24Z",

  // GET /api/incidents/{id} only:
  "comments": [
    { "id": 1, "incidentId": 1, "username": "admin",
      "body": "Looking into it", "createdAt": "2026-07-15T10:50:00Z" }
  ]
}
```

## Integration object

```jsonc
{
  "id": 1,
  "type": "telegram",
  "name": "Ops",
  "enabled": true,
  "config": { "chatId": "-100123" },  // secret keys stripped
  "secrets": { "botToken": true },    // which secrets are set, never their values
  "createdAt": "2026-07-15T10:45:24Z",
  "updatedAt": "2026-07-15T10:45:24Z"
}
```
