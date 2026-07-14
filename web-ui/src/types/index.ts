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
