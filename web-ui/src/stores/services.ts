import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Service, WidgetMode } from '@/types'
import { api, type ServiceInput, type LayoutItem } from '@/api'

export type { ServiceInput }

/** Grid item shape emitted by grid-layout-plus (`i` is the service id). */
interface GridItem {
  i: string
  x: number
  y: number
  w: number
  h: number
}

/** Services + live metrics, backed by the REST API. */
export const useServicesStore = defineStore('services', () => {
  const services = ref<Service[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  const hasServices = computed(() => services.value.length > 0)
  const onlineCount = computed(() => services.value.filter((s) => s.status === 'online').length)
  const offlineCount = computed(() => services.value.filter((s) => s.status === 'offline').length)
  const unknownCount = computed(() => services.value.filter((s) => s.status === 'unknown').length)
  const avgUptime = computed(() => {
    const tracked = services.value.filter((s) => s.status !== 'unknown')
    if (tracked.length === 0) return 0
    return tracked.reduce((sum, s) => sum + s.uptime, 0) / tracked.length
  })

  function getById(id: string): Service | undefined {
    return services.value.find((s) => s.id === id)
  }

  function replace(service: Service) {
    const i = services.value.findIndex((s) => s.id === service.id)
    if (i >= 0) services.value[i] = service
    else services.value.push(service)
  }

  /** Initial load (shows a loading state). */
  async function fetchServices() {
    loading.value = true
    try {
      services.value = await api.listServices()
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /** Quiet refresh used by polling (no loading flag, no layout reshuffle). */
  async function refresh() {
    services.value = await api.listServices()
  }

  async function addService(input: ServiceInput): Promise<Service> {
    const service = await api.createService(input)
    services.value.push(service)
    return service
  }

  async function updateService(id: string, patch: Partial<ServiceInput>) {
    replace(await api.updateService(id, patch))
  }

  async function removeService(id: string) {
    await api.deleteService(id)
    services.value = services.value.filter((s) => s.id !== id)
  }

  async function setWidgetMode(id: string, mode: WidgetMode) {
    const s = getById(id)
    if (!s) return
    await api.updateLayout([{ id, x: s.layout.x, y: s.layout.y, w: s.layout.w, h: s.layout.h, mode }])
    s.widget.mode = mode
  }

  /** Persist grid positions (from grid-layout-plus) to config.yaml. */
  async function saveLayout(items: GridItem[]) {
    const payload: LayoutItem[] = items.map((it) => ({ id: it.i, x: it.x, y: it.y, w: it.w, h: it.h }))
    for (const it of items) {
      const s = getById(it.i)
      if (s) s.layout = { x: it.x, y: it.y, w: it.w, h: it.h }
    }
    await api.updateLayout(payload)
  }

  async function checkNow(id: string) {
    replace(await api.checkNow(id))
  }

  async function uploadIcon(id: string, blob: Blob) {
    const { icon } = await api.uploadImage(id, blob)
    const s = getById(id)
    if (s) s.icon = `${icon}?t=${Date.now()}`
  }

  async function removeIcon(id: string) {
    await api.deleteImage(id)
    const s = getById(id)
    if (s) s.icon = null
  }

  return {
    services,
    loading,
    loaded,
    hasServices,
    onlineCount,
    offlineCount,
    unknownCount,
    avgUptime,
    getById,
    fetchServices,
    refresh,
    addService,
    updateService,
    removeService,
    setWidgetMode,
    saveLayout,
    checkNow,
    uploadIcon,
    removeIcon,
  }
})
