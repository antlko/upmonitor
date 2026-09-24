<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, LoaderCircle } from '@lucide/vue'
import PageHeader from '@/components/layout/PageHeader.vue'
import StatusDot from '@/components/dashboard/StatusDot.vue'
import IncidentFormDialog from '@/components/incidents/IncidentFormDialog.vue'
import PagerControls from '@/components/common/PagerControls.vue'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { toast } from '@/components/ui/sonner'
import { useIncidentsStore, INCIDENTS_PAGE_SIZE, type IncidentInput } from '@/stores/incidents'
import { useServicesStore } from '@/stores/services'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/api'
import type { Incident } from '@/types'

const router = useRouter()
const store = useIncidentsStore()
const services = useServicesStore()
const auth = useAuthStore()

const filter = ref<'all' | 'ongoing' | 'resolved'>('all')
const createOpen = ref(false)

function errMsg(e: unknown) {
  return e instanceof ApiError ? e.message : 'Something went wrong'
}

function load(page = 1) {
  store
    .fetchIncidents({ ...(filter.value === 'all' ? {} : { status: filter.value }), page })
    .catch((e) => {
      toast.error(errMsg(e))
    })
}

onMounted(() => {
  if (!services.loaded) services.fetchServices().catch(() => {})
  load()
})
// A filter change starts back at page 1 — the old page number may not exist
// under the new filter.
watch(filter, () => load(1))

const rows = computed(() => store.incidents)
const totalPages = computed(() => Math.max(1, Math.ceil(store.total / INCIDENTS_PAGE_SIZE)))

async function onCreate(input: IncidentInput) {
  try {
    await store.create(input)
    toast.success('Incident created')
    load()
  } catch (e) {
    toast.error(errMsg(e))
  }
}

function fmtDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
function duration(startedAt: string, resolvedAt: string | null): string {
  const end = resolvedAt ? new Date(resolvedAt).getTime() : Date.now()
  const mins = Math.max(1, Math.round((end - new Date(startedAt).getTime()) / 60000))
  if (mins < 60) return `${mins}m`
  const hrs = Math.floor(mins / 60)
  return hrs < 24 ? `${hrs}h ${mins % 60}m` : `${Math.floor(hrs / 24)}d ${hrs % 24}h`
}
function rowTitle(inc: Incident): string {
  if (inc.title) return inc.title
  if (inc.severity === 'warning') return 'Recovered on retry'
  return inc.status === 'ongoing' ? 'Ongoing outage' : 'Outage'
}
</script>

<template>
  <div class="mx-auto max-w-[1200px] px-6 py-6 lg:px-8">
    <PageHeader title="Incidents" description="Outages detected automatically or logged manually.">
      <template #actions>
        <Button v-if="auth.isAdmin" @click="createOpen = true">
          <Plus />
          New incident
        </Button>
      </template>
    </PageHeader>

    <div class="mt-6 flex items-center justify-between gap-3">
      <Tabs v-model="filter">
        <TabsList>
          <TabsTrigger value="all">All</TabsTrigger>
          <TabsTrigger value="ongoing">Ongoing</TabsTrigger>
          <TabsTrigger value="resolved">Resolved</TabsTrigger>
        </TabsList>
      </Tabs>
    </div>

    <div class="mt-4 rounded-xl border border-border bg-card">
      <div v-if="store.loading && !store.loaded" class="flex justify-center py-16 text-muted-foreground">
        <LoaderCircle class="size-5 animate-spin" />
      </div>
      <div v-else-if="rows.length === 0" class="py-16 text-center text-sm text-muted-foreground">
        No incidents to show.
      </div>
      <Table v-else>
        <TableHeader>
          <TableRow>
            <TableHead>Service</TableHead>
            <TableHead>Incident</TableHead>
            <TableHead>Started</TableHead>
            <TableHead>Duration</TableHead>
            <TableHead>Status</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow
            v-for="inc in rows"
            :key="inc.id"
            class="cursor-pointer"
            @click="router.push(`/incidents/${inc.id}`)"
          >
            <TableCell class="font-medium">{{ inc.serviceName }}</TableCell>
            <TableCell class="text-muted-foreground">
              {{ rowTitle(inc) }}
              <span
                v-if="inc.source === 'manual'"
                class="ml-1.5 rounded bg-muted px-1.5 py-0.5 text-[10px] uppercase text-muted-foreground"
              >
                manual
              </span>
            </TableCell>
            <TableCell class="tabular-nums text-muted-foreground">{{ fmtDateTime(inc.startedAt) }}</TableCell>
            <TableCell class="tabular-nums">
              {{ inc.severity === 'warning' ? '—' : duration(inc.startedAt, inc.resolvedAt) }}
            </TableCell>
            <TableCell>
              <span v-if="inc.severity === 'warning'" class="inline-flex items-center gap-1.5">
                <StatusDot status="warning" :pulse="false" />
                <span class="text-xs">Warning</span>
              </span>
              <span v-else class="inline-flex items-center gap-1.5">
                <StatusDot
                  :status="inc.status === 'ongoing' ? 'offline' : 'online'"
                  :pulse="inc.status === 'ongoing'"
                />
                <span class="text-xs capitalize">{{ inc.status }}</span>
              </span>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <PagerControls
        v-if="rows.length > 0"
        :page="store.page"
        :total-pages="totalPages"
        :has-prev="store.page > 1"
        :has-next="store.page < totalPages"
        :loading="store.loading"
        @prev="load(store.page - 1)"
        @next="load(store.page + 1)"
      />
    </div>

    <IncidentFormDialog v-model:open="createOpen" :incident="null" @submit="onCreate" />
  </div>
</template>
