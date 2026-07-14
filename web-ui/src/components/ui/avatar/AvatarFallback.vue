<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { AvatarFallback, type AvatarFallbackProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<AvatarFallbackProps & { class?: HTMLAttributes['class'] }>()
</script>

<template>
  <AvatarFallback
    :delay-ms="props.delayMs"
    :as-child="props.asChild"
    :as="props.as"
    :class="
      cn(
        'flex size-full items-center justify-center rounded-md bg-muted text-sm font-medium text-muted-foreground',
        props.class,
      )
    "
  >
    <slot />
  </AvatarFallback>
</template>
