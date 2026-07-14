<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// The upmonitor logo: a rounded gradient tile with a heartbeat/uptime pulse.
defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <svg
    viewBox="0 0 32 32"
    :class="cn('size-8', $props.class)"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
  >
    <defs>
      <linearGradient id="brand-grad" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0" stop-color="oklch(0.72 0.18 285)" />
        <stop offset="1" stop-color="oklch(0.6 0.2 265)" />
      </linearGradient>
    </defs>
    <rect x="1" y="1" width="30" height="30" rx="9" fill="url(#brand-grad)" />
    <path
      d="M6 17.5h4l2.5-6 3.5 11 2.6-8 1.9 3H26"
      stroke="#fff"
      stroke-width="2.1"
      stroke-linecap="round"
      stroke-linejoin="round"
      stroke-opacity="0.95"
    />
  </svg>
</template>
