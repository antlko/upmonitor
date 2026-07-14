import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AppSettings } from '@/types'
import { api } from '@/api'

const defaults: AppSettings = {
  publicDashboard: false,
  defaultWidgetMode: 'name',
  theme: 'dark',
  check: { defaultInterval: 30, timeout: 10, retentionDays: 7 },
  configDir: '',
}

/** App settings, backed by config.yaml through the API. */
export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<AppSettings>({ ...defaults })
  const loaded = ref(false)

  async function fetch() {
    settings.value = await api.getSettings()
    loaded.value = true
  }

  async function update(patch: Partial<AppSettings>) {
    settings.value = await api.updateSettings({ ...settings.value, ...patch })
  }

  async function setConfigPath(path: string) {
    settings.value = await api.setConfigPath(path)
  }

  return { settings, loaded, fetch, update, setConfigPath }
})
