import type { ServiceStatus } from '@/types'

/** Human label for a status. */
export function statusLabel(status: ServiceStatus): string {
  return status === 'online' ? 'Online' : status === 'offline' ? 'Offline' : 'Unknown'
}

/** Format a latency in ms, e.g. `120ms` or `1.2s`. */
export function formatLatency(ms: number | null): string {
  if (ms == null) return '—'
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

/** Format an uptime percentage, e.g. `99.98%`. */
export function formatUptime(pct: number): string {
  return `${pct.toFixed(2)}%`
}

/** Compact relative time, e.g. `12s ago`, `4m ago`, `2h ago`. */
export function timeAgo(iso: string | null): string {
  if (!iso) return 'never'
  const diff = Date.now() - new Date(iso).getTime()
  const s = Math.max(0, Math.floor(diff / 1000))
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  const d = Math.floor(h / 24)
  return `${d}d ago`
}

/** Up-to-two-letter initials derived from a service name (fallback icon glyph). */
export function initials(name: string): string {
  const parts = name.trim().split(/[\s._-]+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0]!.slice(0, 2).toUpperCase()
  return (parts[0]![0]! + parts[1]![0]!).toUpperCase()
}

/** Strip protocol / trailing slash for a compact URL display. */
export function prettyUrl(url: string): string {
  return url.replace(/^https?:\/\//, '').replace(/\/$/, '')
}
