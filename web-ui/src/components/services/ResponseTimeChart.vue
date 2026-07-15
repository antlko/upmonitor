<script setup lang="ts">
import { computed, ref } from 'vue'
import { useElementSize } from '@vueuse/core'
import type { SeriesPoint } from '@/types'
import { formatLatency } from '@/lib/format'

const props = withDefaults(defineProps<{ series: SeriesPoint[]; height?: number }>(), {
  height: 220,
})

const wrap = ref<HTMLElement>()
const { width } = useElementSize(wrap)
const w = computed(() => width.value || 640)
const h = computed(() => props.height)

const gradId = 'rtchart-grad'
const pad = { top: 16, right: 16, bottom: 26, left: 48 }

// Points that actually have a latency reading, kept in chronological order.
const pts = computed(() => props.series.filter((p) => p.avgLatency != null))
const values = computed(() => pts.value.map((p) => p.avgLatency as number))

const stats = computed(() => {
  const v = values.value
  if (v.length === 0) return null
  const min = Math.min(...v)
  const max = Math.max(...v)
  const avg = v.reduce((s, x) => s + x, 0) / v.length
  return { min, max, avg, flat: max === min }
})

// The y-scale domain is padded for a flat series so a stable service renders as
// a centred line rather than one pinned to the chart floor.
const domain = computed(() => {
  const s = stats.value
  if (!s) return { lo: 0, hi: 1 }
  if (!s.flat) return { lo: s.min, hi: s.max }
  const margin = Math.max(1, Math.abs(s.max) * 0.5)
  return { lo: s.min - margin, hi: s.max + margin }
})

function x(i: number) {
  const n = pts.value.length
  const innerW = w.value - pad.left - pad.right
  if (n <= 1) return pad.left + innerW / 2
  return pad.left + (i / (n - 1)) * innerW
}
function y(v: number) {
  const { lo, hi } = domain.value
  const range = hi - lo || 1
  const innerH = h.value - pad.top - pad.bottom
  return pad.top + (1 - (v - lo) / range) * innerH
}

const line = computed(() =>
  values.value.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)} ${y(v).toFixed(1)}`).join(' '),
)

// A lone sample produces a path with no segment to stroke — mark it with a dot.
const dot = computed(() => {
  const v = values.value
  return v.length === 1 ? { cx: x(0), cy: y(v[0]!) } : null
})
const area = computed(() => {
  if (values.value.length === 0) return ''
  const base = h.value - pad.bottom
  return `${line.value} L${x(values.value.length - 1).toFixed(1)} ${base} L${x(0).toFixed(1)} ${base} Z`
})

// Reference lines at max / avg / min — collapsed to a single line when every
// sample is identical, which would otherwise stack three labels on one row.
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

function fmtTime(ts: number) {
  const d = new Date(ts * 1000)
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}
const xLabels = computed(() => {
  const p = pts.value
  if (p.length === 0) return []
  return [
    { label: fmtTime(p[0]!.ts), anchor: 'start' as const, x: pad.left },
    { label: fmtTime(p[p.length - 1]!.ts), anchor: 'end' as const, x: w.value - pad.right },
  ]
})
</script>

<template>
  <div ref="wrap" class="w-full">
    <div v-if="!stats" class="flex h-40 items-center justify-center text-sm text-muted-foreground">
      No response-time data in this range yet.
    </div>
    <svg v-else :viewBox="`0 0 ${w} ${h}`" :width="w" :height="h" class="text-primary">
      <defs>
        <linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="currentColor" stop-opacity="0.18" />
          <stop offset="1" stop-color="currentColor" stop-opacity="0" />
        </linearGradient>
      </defs>

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

      <path :d="area" :fill="`url(#${gradId})`" />
      <path
        :d="line"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <circle v-if="dot" :cx="dot.cx" :cy="dot.cy" r="3" fill="currentColor" />

      <!-- x labels -->
      <text
        v-for="l in xLabels"
        :key="l.label"
        :x="l.x"
        :y="h - 6"
        :text-anchor="l.anchor"
        class="fill-muted-foreground text-[10px] tabular-nums"
      >
        {{ l.label }}
      </text>
    </svg>
  </div>
</template>
