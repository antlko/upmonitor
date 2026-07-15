<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <td :class="cn('px-3 py-2.5 align-middle', props.class)">
    <slot />
  </td>
</template>
