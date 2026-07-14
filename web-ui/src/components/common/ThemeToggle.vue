<script setup lang="ts">
import { Sun, Moon } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
</script>

<template>
  <Tooltip>
    <TooltipTrigger as-child>
      <Button variant="ghost" size="icon" aria-label="Toggle theme" @click="ui.toggleTheme()">
        <Transition name="theme-swap" mode="out-in">
          <Moon v-if="ui.isDark" key="moon" />
          <Sun v-else key="sun" />
        </Transition>
      </Button>
    </TooltipTrigger>
    <TooltipContent>{{ ui.isDark ? 'Switch to light' : 'Switch to dark' }}</TooltipContent>
  </Tooltip>
</template>

<style scoped>
.theme-swap-enter-active,
.theme-swap-leave-active {
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
}
.theme-swap-enter-from {
  opacity: 0;
  transform: rotate(-45deg) scale(0.7);
}
.theme-swap-leave-to {
  opacity: 0;
  transform: rotate(45deg) scale(0.7);
}
</style>
