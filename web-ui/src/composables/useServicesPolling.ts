import { onMounted, onUnmounted } from 'vue'
import { useServicesStore } from '@/stores/services'

/**
 * Load services once, then poll for live status/metrics on an interval and when
 * the tab regains focus. Errors are swallowed so a transient failure doesn't
 * break the dashboard.
 */
export function useServicesPolling(intervalMs = 10_000) {
  const services = useServicesStore()
  let timer: ReturnType<typeof setInterval> | undefined

  async function tick() {
    try {
      await services.refresh()
    } catch {
      // transient; keep the last known data
    }
  }

  function onVisibility() {
    if (document.visibilityState === 'visible') tick()
  }

  onMounted(async () => {
    await services.fetchServices()
    timer = setInterval(tick, intervalMs)
    document.addEventListener('visibilitychange', onVisibility)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
    document.removeEventListener('visibilitychange', onVisibility)
  })
}
