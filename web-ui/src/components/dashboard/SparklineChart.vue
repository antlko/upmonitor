<script setup lang="ts">
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{ values: number[]; width?: number; height?: number; color?: string }>(),
  { width: 140, height: 40, color: 'var(--color-online)' },
)

const gradId = `spark-${useId()}`

const points = computed(() => {
  const vals = props.values
  if (vals.length < 2) return [] as [number, number][]
  const min = Math.min(...vals)
  const max = Math.max(...vals)
  const range = max - min || 1
  const stepX = props.width / (vals.length - 1)
  const pad = 3
  // A perfectly steady service has no spread; centre it instead of pinning the
  // line to the floor of the chart.
  const flat = max === min
  return vals.map(
    (v, i) =>
      [
        i * stepX,
        flat ? props.height / 2 : props.height - pad - ((v - min) / range) * (props.height - pad * 2),
      ] as [number, number],
  )
})

const line = computed(() =>
  points.value.map((p, i) => `${i ? 'L' : 'M'}${p[0].toFixed(1)} ${p[1].toFixed(1)}`).join(' '),
)
const area = computed(() =>
  points.value.length
    ? `${line.value} L${props.width} ${props.height} L0 ${props.height} Z`
    : '',
)
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
    <path v-if="area" :d="area" :fill="`url(#${gradId})`" />
    <path
      v-if="line"
      :d="line"
      fill="none"
      stroke="currentColor"
      stroke-width="1.75"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
</template>
