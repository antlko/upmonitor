<script setup lang="ts">
import { computed, type HTMLAttributes } from 'vue'
import type { ServiceStatus } from '@/types'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{ status: ServiceStatus; pulse?: boolean; size?: 'sm' | 'md'; class?: HTMLAttributes['class'] }>(),
  { pulse: true, size: 'sm' },
)

const color = computed(() =>
  props.status === 'online' ? 'bg-online' : props.status === 'offline' ? 'bg-offline' : 'bg-unknown',
)
const dim = computed(() => (props.size === 'md' ? 'size-2.5' : 'size-2'))
</script>

<template>
  <span :class="cn('relative flex', dim, props.class)">
    <span
      v-if="pulse && status === 'online'"
      :class="cn('absolute inline-flex h-full w-full animate-ping rounded-full opacity-75', color)"
    />
    <span :class="cn('relative inline-flex rounded-full', dim, color)" />
  </span>
</template>
