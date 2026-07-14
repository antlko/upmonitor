<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="
      cn(
        'flex flex-col rounded-xl border border-border bg-card text-card-foreground shadow-elevation-low',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
