<script setup lang="ts">
import { TooltipRoot, type TooltipRootProps, type TooltipRootEmits, useForwardPropsEmits } from 'reka-ui'

const props = withDefaults(defineProps<TooltipRootProps>(), { delayDuration: 200 })
const emits = defineEmits<TooltipRootEmits>()
const forwarded = useForwardPropsEmits(props, emits)
</script>

<template>
  <TooltipRoot v-bind="forwarded">
    <slot />
  </TooltipRoot>
</template>
