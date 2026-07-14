import { defineStore } from 'pinia'
import { computed, watch } from 'vue'
import { useStorage } from '@vueuse/core'

/** Global UI state: theme (dark-first) and sidebar collapse, both persisted. */
export const useUiStore = defineStore('ui', () => {
  const theme = useStorage<'dark' | 'light'>('upmonitor-theme', 'dark')
  const sidebarCollapsed = useStorage('upmonitor-sidebar-collapsed', false)
  const commandPaletteOpen = useStorage('upmonitor-cmdk', false)

  function applyTheme() {
    document.documentElement.classList.toggle('dark', theme.value === 'dark')
  }
  watch(theme, applyTheme, { immediate: true })

  const isDark = computed(() => theme.value === 'dark')

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }
  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return { theme, isDark, sidebarCollapsed, commandPaletteOpen, toggleTheme, toggleSidebar }
})
