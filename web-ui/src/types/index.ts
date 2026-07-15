/**
 * Shared domain types. These mirror the backend's JSON shapes (to be built) and
 * the `config.yaml` schema, so the same types are reused once the API is wired.
 */

export type ServiceStatus = 'online' | 'offline' | 'unknown'

export type WidgetMode = 'icon' | 'name' | 'dashboard'

export type UserRole = 'admin' | 'readonly'

/** Grid position of a service card (maps to grid-layout-plus item + config.yaml `layout`). */
export interface WidgetLayout {
  x: number
  y: number
  w: number
  h: number
}

/** Health-check configuration for a single service. */
export interface ServiceCheck {
  interval: number // seconds
  method: string
  timeout: number // seconds
  expectedStatus: number[] // empty ⇒ any 2xx is "online"
}

/** A monitored service — the core domain entity. */
export interface Service {
  id: string
  name: string
  url: string
  icon: string | null
  check: ServiceCheck
  widget: { mode: WidgetMode }
  layout: WidgetLayout
  // Runtime state (from the latest check + aggregates; mocked for now).
  status: ServiceStatus
  latencyMs: number | null
  uptime: number // 0..100 over the retention window
  errorCount: number
  lastCheck: string | null // ISO timestamp
  lastSuccess: string | null // ISO timestamp
  latencyHistory: number[] // recent latencies for the sparkline
}

/** One bucketed point of the response-time / error time series. */
export interface SeriesPoint {
  ts: number
  avgLatency: number | null
  errors: number
}

/** Current TLS certificate snapshot for a service (null for HTTP services). */
export interface ServiceTls {
  checkedAt: string
  validFrom: string | null
  validUntil: string | null
  issuer: string
  subject: string
  daysLeft: number | null
  error: string
}

export interface UptimeWindows {
  days7: number
  days30: number
  days365: number
}

/** Response of GET /api/services/:id/metrics — a Service plus detail extras. */
export interface ServiceMetrics extends Service {
  series: SeriesPoint[]
  uptimeWindows: UptimeWindows
  tls: ServiceTls | null
}

export type IncidentStatus = 'ongoing' | 'resolved'
export type IncidentSource = 'auto' | 'manual'

export interface Incident {
  id: number
  serviceId: string
  serviceName: string
  status: IncidentStatus
  source: IncidentSource
  title: string
  startedAt: string
  resolvedAt: string | null
  createdBy: number | null
  createdAt: string
  updatedAt: string
}

export interface IncidentComment {
  id: number
  incidentId: number
  username: string
  body: string
  createdAt: string
}

export interface IncidentDetail extends Incident {
  comments: IncidentComment[]
}

export type IntegrationType = 'telegram' | 'slack' | 'email' | 'webhook'

export interface Integration {
  id: number
  type: IntegrationType
  name: string
  enabled: boolean
  config: Record<string, unknown>
  secrets: Record<string, boolean>
  createdAt: string
  updatedAt: string
}

export interface User {
  id: number
  username: string
  role: UserRole
  createdAt: string
}

export interface AppSettings {
  publicDashboard: boolean
  defaultWidgetMode: WidgetMode
  theme: 'dark' | 'light' | 'system'
  check: {
    defaultInterval: number
    timeout: number
    retentionDays: number
  }
  configDir: string
}
