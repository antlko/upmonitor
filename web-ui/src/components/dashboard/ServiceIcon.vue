<script setup lang="ts">
import { computed, type HTMLAttributes } from 'vue'
import type { Service } from '@/types'
import { generateIconDataUrl } from '@/lib/icon-generator'
import { cn } from '@/lib/utils'

const props = defineProps<{ service: Service; class?: HTMLAttributes['class'] }>()

// Use the uploaded icon if present, otherwise a deterministic procedural icon.
const src = computed(
  () => props.service.icon ?? generateIconDataUrl(props.service.id, 'gradient', props.service.name),
)
</script>

<template>
  <img
    :src="src"
    :alt="`${service.name} icon`"
    draggable="false"
    :class="cn('rounded-xl object-cover select-none', props.class)"
  />
</template>
