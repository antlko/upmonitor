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
| `PATCH /api/services/layout`     | admin | Bulk-save grid layout `[{ id, x, y, w, h, mode? }]`.  |
| `POST /api/services/{id}/check`  | admin | Run a check immediately; returns updated metrics.     |
| `GET /api/services/{id}/metrics` | user  | Aggregates + time series (`?range=24h\|7d`).          |
| `POST /api/services/{id}/image`  | admin | Upload a WebP icon (raw `image/webp` body).           |
| `DELETE /api/services/{id}/image`| admin | Remove the icon.                                      |

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
| `GET /api/config/export`   | admin | Download `backup.zip` (config.yaml + images/).   |
| `POST /api/config/import`  | admin | Apply a `.zip` (raw `application/zip` body).     |

## Public & static

| Method & path              | Auth   | Description                                          |
| -------------------------- | ------ | --------------------------------------------------- |
| `GET /api/public/services` | none¹  | Read-only service list for the public dashboard.    |
| `GET /images/{file}`       | user²  | Serve a stored icon.                                |

¹ Returns `403` unless `public_dashboard` is enabled.
² Public when `public_dashboard` is enabled.

## Service object

```jsonc
{
  "id": "grafana",
  "name": "Grafana",
  "url": "https://grafana.home.lab",
  "icon": "/images/grafana.webp",      // or null
  "check": { "interval": 30, "method": "GET", "timeout": 10, "expectedStatus": [200] },
  "widget": { "mode": "dashboard" },
  "layout": { "x": 0, "y": 0, "w": 3, "h": 4 },
  "status": "online",                   // online | offline | unknown
  "latencyMs": 118,                     // or null
  "uptime": 99.98,                      // percent over the retention window
  "errorCount": 0,
  "lastCheck": "2026-07-14T12:00:00Z",  // or null
  "lastSuccess": "2026-07-14T12:00:00Z",// or null
  "latencyHistory": [120, 118, 121]     // recent latencies for the sparkline
}
```
