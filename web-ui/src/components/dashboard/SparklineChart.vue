<script setup lang="ts">
import { computed, useId } from 'vue'
import type { ChartType } from '@/types'

const props = withDefaults(
  defineProps<{
    /** Chronological latencies; `null` means that check was offline. */
    values: (number | null)[]
    width?: number
    height?: number
    color?: string
    type?: ChartType
  }>(),
  { width: 140, height: 40, color: 'var(--color-online)', type: 'line' },
)

const gradId = `spark-${useId()}`
const pad = 3

/** Scale is built from the online samples only — nulls have no y position. */
const scale = computed(() => {
  const up = props.values.filter((v): v is number => v != null)
  if (up.length === 0) return null
  const min = Math.min(...up)
  const max = Math.max(...up)
  return { min, max, range: max - min || 1, flat: max === min }
})

/** Offline samples still occupy a slot, so x stays keyed on the full array. */
function xAt(i: number) {
  const n = props.values.length
  return n < 2 ? props.width / 2 : (i / (n - 1)) * props.width
}
function yAt(v: number) {
  const s = scale.value
  if (!s) return props.height / 2
  // A perfectly steady service has no spread; centre it instead of pinning the
  // line to the floor of the chart.
  if (s.flat) return props.height / 2
  return props.height - pad - ((v - s.min) / s.range) * (props.height - pad * 2)
}

/** Runs of consecutive online samples. An outage ends a run rather than being
 *  interpolated across, which would draw a healthy line through downtime. */
const segments = computed(() => {
  const out: [number, number][][] = []
  let cur: [number, number][] = []
  props.values.forEach((v, i) => {
    if (v == null) {
      if (cur.length) out.push(cur)
      cur = []
      return
    }
    cur.push([xAt(i), yAt(v)])
  })
  if (cur.length) out.push(cur)
  return out
})

function path(seg: [number, number][]) {
  return seg.map((p, i) => `${i ? 'L' : 'M'}${p[0].toFixed(1)} ${p[1].toFixed(1)}`).join(' ')
}
const lines = computed(() => segments.value.filter((s) => s.length > 1).map(path))
const areas = computed(() =>
  segments.value
    .filter((s) => s.length > 1)
    .map((s) => `${path(s)} L${s[s.length - 1]![0].toFixed(1)} ${props.height} L${s[0]![0].toFixed(1)} ${props.height} Z`),
)
/** A single online sample between two outages has no segment to stroke. */
const dots = computed(() =>
  segments.value.filter((s) => s.length === 1).map((s) => ({ cx: s[0]![0], cy: s[0]![1] })),
)

const bars = computed(() => {
  const n = props.values.length
  const slot = n > 0 ? props.width / n : props.width
  const w = Math.max(1, slot - Math.min(1.5, slot * 0.25))
  return props.values.map((v, i) => {
    const down = v == null
    const top = down ? 0 : yAt(v)
    return {
      i,
      x: i * slot + (slot - w) / 2,
      w,
      y: top,
      h: Math.max(1, props.height - top),
      down,
    }
  })
})

/** Vertical marks where the service was down, in both chart types. */
const downMarks = computed(() => {
  if (props.type === 'bars') return [] // the red bars already say it
  const n = props.values.length
  const slot = n > 1 ? props.width / (n - 1) : props.width
  return props.values
    .map((v, i) => ({ v, i }))
    .filter(({ v }) => v == null)
    .map(({ i }) => ({ i, x: Math.max(0, xAt(i) - slot / 2), w: Math.max(1.5, slot) }))
})
</script>

<template>
  <svg
    :viewBox="`0 0 ${width} ${height}`"
    :width="width"
    :height="height"
    preserveAspectRatio="none"
    class="overflow-visible"
    :style="{ color }"
  >
    <defs>
      <linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="currentColor" stop-opacity="0.22" />
        <stop offset="1" stop-color="currentColor" stop-opacity="0" />
      </linearGradient>
    </defs>

    <rect
      v-for="m in downMarks"
      :key="`down-${m.i}`"
      :x="m.x"
      y="0"
      :width="m.w"
      :height="height"
      fill="var(--color-offline)"
      opacity="0.3"
    />

    <template v-if="type === 'bars'">
      <rect
        v-for="b in bars"
        :key="`bar-${b.i}`"
        :x="b.x"
        :y="b.y"
        :width="b.w"
        :height="b.h"
        :fill="b.down ? 'var(--color-offline)' : 'currentColor'"
        :opacity="b.down ? 0.55 : 0.85"
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
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <circle v-for="(d, i) in dots" :key="`dot-${i}`" :cx="d.cx" :cy="d.cy" r="1.5" fill="currentColor" />
    </template>
  </svg>
</template>
