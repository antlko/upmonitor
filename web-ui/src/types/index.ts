/**
 * Shared domain types. These mirror the backend's JSON shapes (to be built) and
 * the `config.yaml` schema, so the same types are reused once the API is wired.
 */

/**
 * `warning` is a check that failed and then recovered on a retry: the service
 * answered, so it counts as uptime and opens no incident. It is sticky — the
 * service stays in warning until a cycle succeeds on its first attempt.
 */
export type ServiceStatus = 'online' | 'offline' | 'warning' | 'unknown'

export type WidgetMode = 'icon' | 'name' | 'dashboard'

/** How a service's response-time history is drawn, on both the card and the detail page. */
export type ChartType = 'line' | 'bars'

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
  /** Attempts per cycle before the service is called offline. 1 disables retries. */
  retryAttempts: number
  /** Waits (seconds) between attempts; the last value repeats. */
  retryDelays: number[]
}

/** A monitored service — the core domain entity. */
export interface Service {
  id: string
  name: string
  url: string
  icon: string | null
  check: ServiceCheck
  widget: { mode: WidgetMode }
  chart: { type: ChartType }
  layout: WidgetLayout
  // Runtime state (from the latest check + aggregates; mocked for now).
  status: ServiceStatus
  latencyMs: number | null
  uptime: number // 0..100 over the retention window
  errorCount: number
  /** Cycles in the window that only succeeded on a retry. */
  warningCount: number
  lastCheck: string | null // ISO timestamp
  lastSuccess: string | null // ISO timestamp
  /**
   * Recent check latencies for the sparkline, chronological. `null` means that
   * check was **offline** — not that the reading is missing.
   */
  latencyHistory: (number | null)[]
  /**
   * Parallel to `latencyHistory`. A warning check carries a latency just like an
   * online one, so the status is the only way to tell them apart.
   */
  statusHistory: ServiceStatus[]
}

/**
 * One bucketed point of the response-time / error time series.
 * `avgLatency: null` means no check succeeded in this bucket — the service was
 * down for all of it. Buckets with no checks at all are omitted entirely, so
 * consecutive points are not necessarily adjacent in time.
 */
export interface SeriesPoint {
  ts: number
  avgLatency: number | null
  errors: number
  /** Checks in this bucket that recovered on a retry. */
  warnings: number
}

/** One stored check cycle, as listed in a service's ping console. */
export interface CheckRow {
  id: number
  ts: string
  status: ServiceStatus
  latencyMs: number | null
  statusCode: number | null
  /** For a warning, why the FIRST attempt failed. */
  error: string
  attempts: number
}

/** A page of check rows; `nextBefore` is null once there is nothing older. */
export interface ChecksPage {
  checks: CheckRow[]
  nextBefore: number | null
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
  /** The requested window (unix seconds). The chart's x-domain, not the data's extent. */
  from: number
  to: number
  /** Width of one series bucket; also the gap threshold for breaking the line. */
  bucketSeconds: number
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
  /** Opt-in to warning events (a check that recovered on a retry). Off by default. */
  notifyWarnings: boolean
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
  defaultWidgetMode: WidgetMode
  theme: 'dark' | 'light' | 'system'
  check: {
    defaultInterval: number
    timeout: number
    retentionDays: number
    retryAttempts: number
    retryDelays: number[]
  }
  configDir: string
}
