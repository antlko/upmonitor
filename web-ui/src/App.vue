<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Toaster } from '@/components/ui/sonner'
import AppShell from '@/components/layout/AppShell.vue'

const route = useRoute()
// Auth / setup / 404 render without the sidebar + topbar chrome.
const bare = computed(() => route.meta.bare === true)
</script>

<template>
  <TooltipProvider :delay-duration="200">
    <RouterView v-if="bare" v-slot="{ Component }">
      <Transition name="fade" mode="out-in">
        <component :is="Component" />
      </Transition>
    </RouterView>
    <AppShell v-else />
    <Toaster position="bottom-right" rich-colors />
  </TooltipProvider>
</template>

<style>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.16s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Route transition inside the shell — subtle rise + fade. */
.page-enter-active {
  transition:
    opacity 0.22s ease,
    transform 0.22s ease;
}
.page-leave-active {
  transition:
    opacity 0.12s ease,
    transform 0.12s ease;
}
.page-enter-from {
  opacity: 0;
  transform: translateY(6px);
}
.page-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
