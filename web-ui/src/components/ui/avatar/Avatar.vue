<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { AvatarRoot } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <AvatarRoot
    :class="
      cn(
        'relative flex size-9 shrink-0 overflow-hidden rounded-md select-none',
        props.class,
      )
    "
  >
    <slot />
  </AvatarRoot>
</template>
