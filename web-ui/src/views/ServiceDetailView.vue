<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowUpRight, Pencil, Trash2, LoaderCircle, RefreshCw } from '@lucide/vue'
import ServiceIcon from '@/components/dashboard/ServiceIcon.vue'
import StatusDot from '@/components/dashboard/StatusDot.vue'
import ResponseTimeChart from '@/components/services/ResponseTimeChart.vue'
import SslCertCard from '@/components/services/SslCertCard.vue'
import ServiceFormDialog from '@/components/services/ServiceFormDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toast } from '@/components/ui/sonner'
import { useServicesStore, type ServiceInput } from '@/stores/services'
import { useAuthStore } from '@/stores/auth'
import { api, ApiError, type MetricsRange } from '@/api'
import type { ServiceMetrics, Incident } from '@/types'
import { statusLabel, formatLatency, formatUptime, timeAgo, prettyUrl, averageLatency } from '@/lib/format'

const route = useRoute()
const router = useRouter()
const services = useServicesStore()
const auth = useAuthStore()

const id = computed(() => String(route.params.id))
const range = ref<MetricsRange>('24h')
const metrics = ref<ServiceMetrics | null>(null)
const incidents = ref<Incident[]>([])
const loading = ref(true)
const notFound = ref(false)
let timer: ReturnType<typeof setInterval> | undefined

// Prefer the polled store copy for live header state; fall back to metrics.
const svc = computed(() => services.getById(id.value) ?? metrics.value)

const statusPill = computed(() => {
  const s = svc.value?.status ?? 'unknown'
  return {
    online: 'bg-online/10 text-online',
    offline: 'bg-offline/10 text-offline',
    unknown: 'bg-unknown/15 text-muted-foreground',
  }[s]
})

const avgResponse = computed(() =>
  metrics.value ? averageLatency(metrics.value.latencyHistory) : null,
)

const uptimeStats = computed(() => {
  const w = metrics.value?.uptimeWindows
  return [
    { label: 'Uptime · 7 days', value: w ? formatUptime(w.days7) : '—' },
    { label: 'Uptime · 30 days', value: w ? formatUptime(w.days30) : '—' },
    { label: 'Uptime · 365 days', value: w ? formatUptime(w.days365) : '—' },
    { label: 'Avg response', value: formatLatency(avgResponse.value) },
  ]
})

const ranges: { value: MetricsRange; label: string }[] = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
  { value: '365d', label: '365d' },
]

function errMsg(e: unknown) {
  return e instanceof ApiError ? e.message : 'Something went wrong'
}

async function loadMetrics() {
  try {
    metrics.value = await api.serviceMetrics(id.value, range.value)
    notFound.value = false
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound.value = true
  } finally {
    loading.value = false
  }
}
async function loadIncidents() {
  try {
    incidents.value = (await api.listIncidents({ serviceId: id.value })).slice(0, 6)
  } catch {
    /* non-fatal */
  }
}

onMounted(async () => {
  if (!services.loaded) await services.fetchServices().catch(() => {})
  await Promise.all([loadMetrics(), loadIncidents()])
  timer = setInterval(() => {
    loadMetrics()
    loadIncidents()
  }, 15_000)
})
onUnmounted(() => timer && clearInterval(timer))
watch(range, loadMetrics)

// --- admin actions ---
const editOpen = ref(false)
const confirmOpen = ref(false)

function openService() {
  if (svc.value) window.open(svc.value.url, '_blank', 'noopener')
}
async function onEdit(input: ServiceInput) {
  try {
    await services.updateService(id.value, input)
    toast.success('Service updated')
    loadMetrics()
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function onDelete() {
  try {
    await services.removeService(id.value)
    toast.success('Service removed')
    router.push('/')
  } catch (e) {
    toast.error(errMsg(e))
  }
}

function incidentDuration(inc: Incident): string {
  const end = inc.resolvedAt ? new Date(inc.resolvedAt).getTime() : Date.now()
  const mins = Math.max(1, Math.round((end - new Date(inc.startedAt).getTime()) / 60000))
  if (mins < 60) return `${mins}m`
  const hrs = Math.floor(mins / 60)
  return hrs < 24 ? `${hrs}h ${mins % 60}m` : `${Math.floor(hrs / 24)}d ${hrs % 24}h`
}
function fmtDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div class="mx-auto max-w-[1200px] px-6 py-6 lg:px-8">
    <button
      class="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
      @click="router.push('/')"
    >
      <ArrowLeft class="size-4" />
      Dashboard
    </button>

    <div v-if="loading" class="flex items-center justify-center py-24 text-muted-foreground">
      <LoaderCircle class="size-5 animate-spin" />
    </div>

    <div v-else-if="notFound || !svc" class="py-24 text-center text-muted-foreground">
      This service no longer exists.
    </div>

    <template v-else>
      <!-- Header -->
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div class="flex min-w-0 items-center gap-4">
          <ServiceIcon :service="svc" class="size-14 shrink-0 shadow-elevation-low" />
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h2 class="truncate text-2xl font-semibold tracking-tight">{{ svc.name }}</h2>
              <span
                :class="[
                  'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium',
                  statusPill,
                ]"
              >
                <StatusDot :status="svc.status" :pulse="svc.status === 'online'" />
                {{ statusLabel(svc.status) }}
              </span>
            </div>
            <p class="mt-1 truncate text-sm text-muted-foreground">{{ prettyUrl(svc.url) }}</p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" @click="openService">
            <ArrowUpRight />
            Open
          </Button>
          <template v-if="auth.isAdmin">
            <Button variant="outline" size="sm" @click="editOpen = true">
              <Pencil />
              Edit
            </Button>
            <Button variant="outline" size="sm" @click="confirmOpen = true">
              <Trash2 />
              Delete
            </Button>
          </template>
        </div>
      </header>

      <!-- Stat grid -->
      <div class="mt-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div
          v-for="stat in uptimeStats"
          :key="stat.label"
          class="rounded-xl border border-border bg-card px-4 py-3"
        >
          <p class="text-xs text-muted-foreground">{{ stat.label }}</p>
          <p class="mt-1 text-xl font-semibold tabular-nums">{{ stat.value }}</p>
        </div>
      </div>
      <p class="mt-2 text-xs text-muted-foreground">
        Uptime windows are based on stored history (limited by the retention setting).
      </p>

      <!-- Chart -->
      <Card class="mt-6">
        <CardHeader class="flex-row items-center justify-between gap-3 space-y-0">
          <div class="flex items-center gap-2">
            <CardTitle class="text-sm">Response time</CardTitle>
            <button
              class="text-muted-foreground transition-colors hover:text-foreground"
              title="Refresh"
              @click="loadMetrics"
            >
              <RefreshCw class="size-3.5" />
            </button>
          </div>
          <Tabs v-model="range">
            <TabsList>
              <TabsTrigger v-for="r in ranges" :key="r.value" :value="r.value">
                {{ r.label }}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </CardHeader>
        <CardContent>
          <ResponseTimeChart :series="metrics?.series ?? []" />
          <p class="mt-1 text-xs text-muted-foreground">
            Last check {{ timeAgo(svc.lastCheck) }} · latest {{ formatLatency(svc.latencyMs) }}
          </p>
        </CardContent>
      </Card>

      <div class="mt-6 grid gap-6 lg:grid-cols-2">
        <SslCertCard :tls="metrics?.tls ?? null" :url="svc.url" />

        <!-- Recent incidents -->
        <Card>
          <CardHeader class="flex-row items-center justify-between gap-2 space-y-0">
            <CardTitle class="text-sm">Recent incidents</CardTitle>
            <RouterLink to="/incidents" class="text-xs text-primary hover:underline">
              View all
            </RouterLink>
          </CardHeader>
          <CardContent>
            <p v-if="incidents.length === 0" class="text-sm text-muted-foreground">
              No incidents recorded for this service.
            </p>
            <ul v-else class="divide-y divide-border">
              <li v-for="inc in incidents" :key="inc.id">
                <RouterLink
                  :to="`/incidents/${inc.id}`"
                  class="-mx-2 flex items-center gap-3 rounded-md px-2 py-2 transition-colors hover:bg-accent"
                >
                  <StatusDot :status="inc.status === 'ongoing' ? 'offline' : 'online'" :pulse="false" />
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-sm font-medium">
                      {{ inc.title || (inc.status === 'ongoing' ? 'Ongoing outage' : 'Outage') }}
                    </p>
                    <p class="text-xs text-muted-foreground">{{ fmtDateTime(inc.startedAt) }}</p>
                  </div>
                  <span class="shrink-0 text-xs tabular-nums text-muted-foreground">
                    {{ incidentDuration(inc) }}
                  </span>
                </RouterLink>
              </li>
            </ul>
          </CardContent>
        </Card>
      </div>
    </template>

    <ServiceFormDialog
      v-if="svc"
      v-model:open="editOpen"
      :service="services.getById(id) ?? null"
      @submit="onEdit"
    />
    <ConfirmDialog
      v-model:open="confirmOpen"
      :title="`Delete ${svc?.name ?? 'service'}?`"
      description="This removes the service and its history. This action cannot be undone."
      confirm-label="Delete"
      destructive
      @confirm="onDelete"
    />
  </div>
</template>
