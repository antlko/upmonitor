<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useElementSize } from '@vueuse/core'
import type { ChartType, SeriesPoint } from '@/types'
import { formatLatency } from '@/lib/format'

/** A resolved outage span in unix seconds. An ongoing outage ends at the chart's `to`. */
export interface OutageWindow {
  start: number
  end: number
}

const props = withDefaults(
  defineProps<{
    series: SeriesPoint[]
    /** The requested window. This is the x-domain — not the extent of the data. */
    from: number
    to: number
    bucketSeconds: number
    outages?: OutageWindow[]
    type?: ChartType
    height?: number
  }>(),
  { height: 220, type: 'line', outages: () => [] },
)

const wrap = ref<HTMLElement>()
const { width } = useElementSize(wrap)
const w = computed(() => width.value || 640)
const h = computed(() => props.height)

const gradId = `rtchart-${useId()}`
const pad = { top: 16, right: 16, bottom: 26, left: 48 }
const innerW = computed(() => Math.max(1, w.value - pad.left - pad.right))
const baseY = computed(() => h.value - pad.top - pad.bottom + pad.top)

const span = computed(() => Math.max(1, props.to - props.from))
const values = computed(() =>
  props.series.map((p) => p.avgLatency).filter((v): v is number => v != null),
)

const stats = computed(() => {
  const v = values.value
  if (v.length === 0) return null
  const min = Math.min(...v)
  const max = Math.max(...v)
  return { min, max, avg: v.reduce((s, x) => s + x, 0) / v.length, flat: max === min }
})

/**
 * Bars encode magnitude by length, so they must baseline at zero — a zoomed
 * domain would make one bar look twice another for a 5% difference. The line
 * encodes change instead, so it keeps a min..max domain to show variation on an
 * otherwise flat service (padded when every sample is identical, which would
 * otherwise pin the line to the chart floor).
 */
const domain = computed(() => {
  const s = stats.value
  if (!s) return { lo: 0, hi: 1 }
  if (props.type === 'bars') return { lo: 0, hi: s.max || 1 }
  if (!s.flat) return { lo: s.min, hi: s.max }
  const margin = Math.max(1, Math.abs(s.max) * 0.5)
  return { lo: s.min - margin, hi: s.max + margin }
})

/** Clamped so out-of-window data and ongoing outages stop at the plot edge. */
function x(ts: number) {
  const px = pad.left + ((ts - props.from) / span.value) * innerW.value
  return Math.min(w.value - pad.right, Math.max(pad.left, px))
}
function y(v: number) {
  const { lo, hi } = domain.value
  const range = hi - lo || 1
  return pad.top + (1 - (v - lo) / range) * (h.value - pad.top - pad.bottom)
}
function invert(px: number) {
  return props.from + ((px - pad.left) / innerW.value) * span.value
}

/** A bucket's average describes its whole width, so plot it at the midpoint. */
function midOf(p: SeriesPoint) {
  return p.ts + props.bucketSeconds / 2
}

/**
 * The line is split wherever it would otherwise imply data it doesn't have:
 * a null average (checks ran, none succeeded — the service was down), or a jump
 * larger than one bucket (no checks ran at all). Joining across either would
 * draw a straight, healthy-looking line through an outage.
 */
const segments = computed(() => {
  const out: { ts: number; v: number }[][] = []
  let cur: { ts: number; v: number }[] = []
  let prevTs: number | null = null
  for (const p of props.series) {
    const gapped = prevTs !== null && p.ts - prevTs > props.bucketSeconds * 1.5
    if (p.avgLatency == null || gapped) {
      if (cur.length) out.push(cur)
      cur = []
    }
    if (p.avgLatency != null) cur.push({ ts: midOf(p), v: p.avgLatency })
    prevTs = p.ts
  }
  if (cur.length) out.push(cur)
  return out
})

function path(seg: { ts: number; v: number }[]) {
  return seg.map((p, i) => `${i ? 'L' : 'M'}${x(p.ts).toFixed(1)} ${y(p.v).toFixed(1)}`).join(' ')
}
const lines = computed(() => segments.value.map(path))
const areas = computed(() =>
  segments.value
    .filter((seg) => seg.length > 1)
    .map((seg) => {
      const b = baseY.value
      return `${path(seg)} L${x(seg[seg.length - 1]!.ts).toFixed(1)} ${b} L${x(seg[0]!.ts).toFixed(1)} ${b} Z`
    }),
)
/** A lone sample has no segment to stroke — mark it so the chart isn't blank. */
const dots = computed(() =>
  segments.value
    .filter((seg) => seg.length === 1)
    .map((seg) => ({ cx: x(seg[0]!.ts), cy: y(seg[0]!.v) })),
)

const bars = computed(() =>
  props.series.map((p, i) => {
    const x0 = x(p.ts)
    const x1 = x(p.ts + props.bucketSeconds)
    const gap = Math.min(2, (x1 - x0) * 0.2)
    const down = p.avgLatency == null
    return {
      i,
      x: x0 + gap / 2,
      w: Math.max(1, x1 - x0 - gap),
      y: down ? pad.top : y(p.avgLatency!),
      h: down ? baseY.value - pad.top : Math.max(1, baseY.value - y(p.avgLatency!)),
      down,
    }
  }),
)

/** Outage bands, clamped to the window and dropped when they fall outside it. */
const bands = computed(() =>
  props.outages
    .filter((o) => o.end >= props.from && o.start <= props.to)
    .map((o) => {
      const x0 = x(o.start)
      // A brief outage would otherwise round away to nothing at a wide range.
      return { x: x0, w: Math.max(2, x(o.end) - x0) }
    }),
)

// Reference lines at max / avg / min — collapsed to one when every sample is
// identical, which would otherwise stack three labels on the same row.
const yTicks = computed(() => {
  const s = stats.value
  if (!s) return []
  if (s.flat) return [{ label: formatLatency(s.avg), y: y(s.avg) }]
  return [
    { label: formatLatency(s.max), y: y(s.max) },
    { label: formatLatency(s.avg), y: y(s.avg) },
    { label: formatLatency(s.min), y: y(s.min) },
  ]
})

function fmtTick(ts: number) {
  const d = new Date(ts * 1000)
  return span.value <= 24 * 3600
    ? d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}
function fmtFull(ts: number) {
  return new Date(ts * 1000).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// Ticks span the requested window, not the data — that is what makes the range
// tabs mean something when a service has less history than the range covers.
const xTicks = computed(() => {
  const n = w.value < 480 ? 2 : 4
  return Array.from({ length: n + 1 }, (_, i) => {
    const ts = props.from + (span.value * i) / n
    return {
      ts,
      x: x(ts),
      label: fmtTick(ts),
      anchor: i === 0 ? ('start' as const) : i === n ? ('end' as const) : ('middle' as const),
    }
  })
})

// --- hover ---
const hoverIdx = ref<number | null>(null)

function onMove(e: PointerEvent) {
  const el = wrap.value
  if (!el || props.series.length === 0) return
  const ts = invert(e.clientX - el.getBoundingClientRect().left)
  let best = -1
  let bestDist = Infinity
  props.series.forEach((p, i) => {
    const d = Math.abs(midOf(p) - ts)
    if (d < bestDist) {
      bestDist = d
      best = i
    }
  })
  // Snapping across a gap would report a reading from a different hour, so
  // require the cursor to be within the nearest bucket.
  hoverIdx.value = bestDist <= props.bucketSeconds ? best : null
}

const hover = computed(() => {
  const i = hoverIdx.value
  if (i == null) return null
  const p = props.series[i]
  if (!p) return null
  return {
    x: x(midOf(p)),
    y: p.avgLatency == null ? null : y(p.avgLatency),
    time: fmtFull(p.ts),
    latency: p.avgLatency,
    errors: p.errors,
  }
})

// Pinned to the top of the plot rather than to the point: it can never cover
// the line, and it flips at the edges instead of overflowing the card.
const tipStyle = computed(() => {
  const hv = hover.value
  if (!hv) return {}
  const flip = hv.x > w.value * 0.6
  return {
    left: `${hv.x}px`,
    top: `${pad.top}px`,
    transform: `translate(${flip ? 'calc(-100% - 10px)' : '10px'}, 0)`,
  }
})
</script>

<template>
  <div ref="wrap" class="relative w-full">
    <div
      v-if="series.length === 0"
      class="flex h-40 items-center justify-center text-sm text-muted-foreground"
    >
      No response-time data in this range yet.
    </div>

    <template v-else>
      <svg :viewBox="`0 0 ${w} ${h}`" :width="w" :height="h" class="text-primary">
        <defs>
          <linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="currentColor" stop-opacity="0.18" />
            <stop offset="1" stop-color="currentColor" stop-opacity="0" />
          </linearGradient>
        </defs>

        <!-- Outage bands sit behind everything: they are context, not data. -->
        <rect
          v-for="(b, i) in bands"
          :key="`band-${i}`"
          :x="b.x"
          :y="pad.top"
          :width="b.w"
          :height="baseY - pad.top"
          fill="var(--color-offline)"
          opacity="0.14"
        />

        <!-- horizontal gridlines + y labels -->
        <g v-for="t in yTicks" :key="t.label">
          <line
            :x1="pad.left"
            :x2="w - pad.right"
            :y1="t.y"
            :y2="t.y"
            stroke="var(--color-border)"
            stroke-dasharray="3 3"
          />
          <text
            :x="pad.left - 8"
            :y="t.y + 3"
            text-anchor="end"
            class="fill-muted-foreground text-[10px] tabular-nums"
          >
            {{ t.label }}
          </text>
        </g>

        <template v-if="type === 'bars'">
          <rect
            v-for="b in bars"
            :key="`bar-${b.i}`"
            :x="b.x"
            :y="b.y"
            :width="b.w"
            :height="b.h"
            :fill="b.down ? 'var(--color-offline)' : 'currentColor'"
            :opacity="b.down ? 0.55 : hoverIdx === b.i ? 1 : 0.85"
            rx="1"
          />
        </template>
        <template v-else>
          <path v-for="(d, i) in areas" :key="`area-${i}`" :d="d" :fill="`url(#${gradId})`" />
          <path
            v-for="(d, i) in lines"
            :key="`line-${i}`"
            :d="d"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <circle v-for="(d, i) in dots" :key="`dot-${i}`" :cx="d.cx" :cy="d.cy" r="3" fill="currentColor" />
        </template>

        <!-- x labels -->
        <text
          v-for="t in xTicks"
          :key="`xt-${t.ts}`"
          :x="t.x"
          :y="h - 6"
          :text-anchor="t.anchor"
          class="fill-muted-foreground text-[10px] tabular-nums"
        >
          {{ t.label }}
        </text>

        <!-- crosshair -->
        <g v-if="hover">
          <line
            :x1="hover.x"
            :x2="hover.x"
            :y1="pad.top"
            :y2="baseY"
            stroke="var(--color-muted-foreground)"
            stroke-width="1"
            stroke-dasharray="3 3"
          />
          <circle
            v-if="hover.y != null && type === 'line'"
            :cx="hover.x"
            :cy="hover.y"
            r="3.5"
            fill="currentColor"
            stroke="var(--color-card)"
            stroke-width="1.5"
          />
        </g>

        <rect
          :x="pad.left"
          :y="pad.top"
          :width="innerW"
          :height="baseY - pad.top"
          fill="transparent"
          @pointermove="onMove"
          @pointerleave="hoverIdx = null"
        />
      </svg>

      <div
        v-if="hover"
        :style="tipStyle"
        class="pointer-events-none absolute z-10 whitespace-nowrap rounded-md border border-border bg-popover px-2.5 py-1.5 text-xs shadow-elevation-medium"
      >
        <p class="tabular-nums text-muted-foreground">{{ hover.time }}</p>
        <p v-if="hover.latency != null" class="mt-0.5 font-medium tabular-nums">
          {{ formatLatency(hover.latency) }}
        </p>
        <p v-else class="mt-0.5 font-medium text-offline">No successful check</p>
        <p v-if="hover.errors > 0" class="mt-0.5 tabular-nums text-offline">
          {{ hover.errors }} failed {{ hover.errors === 1 ? 'check' : 'checks' }}
        </p>
      </div>
    </template>
  </div>
</template>
