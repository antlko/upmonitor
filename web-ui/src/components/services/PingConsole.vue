<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { LoaderCircle } from '@lucide/vue'
import PagerControls from '@/components/common/PagerControls.vue'
import { api } from '@/api'
import { formatLatency } from '@/lib/format'
import type { CheckRow } from '@/types'

const props = defineProps<{ serviceId: string }>()

const rows = ref<CheckRow[]>([])
const nextBefore = ref<number | null>(null)
const loading = ref(true)
const loadingMore = ref(false)

// How many raw rows to fetch per request from the server (a run of successes
// squashes to one line, so this needs to be generous for a page of LINES to
// usually be satisfied by a single fetch).
const FETCH_BATCH = 100
// Lines shown per page. Successful checks are the boring majority and collapse
// to one line each, so a page can span a lot more than 10 raw checks.
const LINES_PER_PAGE = 10
// Safety cap on how many batches a single "next page" click will fetch, so a
// service that has been up for its entire history can't trigger an unbounded
// fetch loop — the user can just click Next again.
const MAX_FETCH_ROUNDS = 20

const page = ref(1)

async function load() {
  loading.value = true
  page.value = 1
  try {
    const res = await api.serviceChecks(props.serviceId, { limit: FETCH_BATCH })
    rows.value = res.checks
    nextBefore.value = res.nextBefore
  } catch {
    /* non-fatal: the console is supplementary */
  } finally {
    loading.value = false
  }
}

async function fetchMoreRaw(): Promise<boolean> {
  if (nextBefore.value == null) return false
  const res = await api.serviceChecks(props.serviceId, { limit: FETCH_BATCH, before: nextBefore.value })
  rows.value = [...rows.value, ...res.checks]
  nextBefore.value = res.nextBefore
  return true
}

/**
 * Refresh only the newest rows, keeping anything already paged in. Rows are
 * immutable once written, so merging by id is enough — no reconciliation.
 */
async function refresh() {
  try {
    const res = await api.serviceChecks(props.serviceId, { limit: FETCH_BATCH })
    const known = new Set(rows.value.map((r) => r.id))
    const fresh = res.checks.filter((r) => !known.has(r.id))
    if (fresh.length) rows.value = [...fresh, ...rows.value]
  } catch {
    /* non-fatal */
  }
}
defineExpose({ refresh })

watch(() => props.serviceId, load, { immediate: true })

type Line =
  | { kind: 'ok'; key: string; count: number; from: string; to: string; avg: number | null }
  | { kind: 'event'; key: string; row: CheckRow }

/**
 * Successful checks are the boring majority, so consecutive ones collapse into
 * a single summary line and anything that failed stays expanded. The squashing
 * runs over the accumulated rows rather than per page, so a run split by a page
 * boundary merges once the next page arrives.
 */
const lines = computed<Line[]>(() => {
  const out: Line[] = []
  let run: CheckRow[] = []

  const flush = () => {
    if (run.length === 0) return
    const latencies = run.map((r) => r.latencyMs).filter((v): v is number => v != null)
    const avg = latencies.length
      ? latencies.reduce((sum, v) => sum + v, 0) / latencies.length
      : null
    // Rows are newest-first, so the run's last entry is the older edge.
    out.push({
      kind: 'ok',
      key: `ok-${run[0]!.id}`,
      count: run.length,
      from: run[run.length - 1]!.ts,
      to: run[0]!.ts,
      avg,
    })
    run = []
  }

  for (const row of rows.value) {
    if (row.status === 'online') {
      run.push(row)
      continue
    }
    flush()
    out.push({ kind: 'event', key: `ev-${row.id}`, row })
  }
  flush()
  return out
})

const pageLines = computed(() => {
  const start = (page.value - 1) * LINES_PER_PAGE
  return lines.value.slice(start, start + LINES_PER_PAGE)
})

const hasPrev = computed(() => page.value > 1)
// More to show either because it's already buffered, or because the server
// might still have older history to fetch.
const hasNext = computed(() => page.value * LINES_PER_PAGE < lines.value.length || nextBefore.value != null)

async function prevPage() {
  if (hasPrev.value) page.value -= 1
}

async function nextPage() {
  if (loadingMore.value) return
  const needed = (page.value + 1) * LINES_PER_PAGE
  if (lines.value.length < needed && nextBefore.value != null) {
    loadingMore.value = true
    try {
      for (let i = 0; i < MAX_FETCH_ROUNDS && lines.value.length < needed && nextBefore.value != null; i++) {
        if (!(await fetchMoreRaw())) break
      }
    } catch {
      /* non-fatal: show whatever page is available */
    } finally {
      loadingMore.value = false
    }
  }
  if (lines.value.length > page.value * LINES_PER_PAGE) page.value += 1
}

function fmtTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
function fmtShort(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { hour: '2-digit', minute: '2-digit' })
}
function detail(row: CheckRow): string {
  const bits: string[] = []
  if (row.statusCode != null) bits.push(`HTTP ${row.statusCode}`)
  if (row.latencyMs != null) bits.push(formatLatency(row.latencyMs))
  if (row.attempts > 1) bits.push(`attempt ${row.attempts}`)
  return bits.join(' · ')
}
</script>

<template>
  <div class="rounded-lg border border-border bg-muted/30">
    <div v-if="loading" class="flex items-center justify-center py-10 text-muted-foreground">
      <LoaderCircle class="size-4 animate-spin" />
    </div>
    <p v-else-if="lines.length === 0" class="px-4 py-8 text-center text-sm text-muted-foreground">
      No checks recorded yet.
    </p>
    <ul v-else class="divide-y divide-border/60 font-mono text-xs">
      <li
        v-for="line in pageLines"
        :key="line.key"
        class="flex items-start gap-2.5 px-3 py-1.5"
        :class="
          line.kind === 'event' && line.row.status === 'offline'
            ? 'bg-offline/5'
            : line.kind === 'event'
              ? 'bg-warning/5'
              : ''
        "
      >
        <template v-if="line.kind === 'ok'">
          <span class="shrink-0 text-online">✓</span>
          <span class="min-w-0 flex-1 text-muted-foreground">
            {{ line.count }} successful {{ line.count === 1 ? 'check' : 'checks' }}
            <span class="tabular-nums">· {{ fmtShort(line.from) }} – {{ fmtShort(line.to) }}</span>
            <span v-if="line.avg != null" class="tabular-nums"> · avg {{ formatLatency(line.avg) }}</span>
          </span>
        </template>
        <template v-else>
          <span
            class="shrink-0"
            :class="line.row.status === 'offline' ? 'text-offline' : 'text-warning'"
          >
            {{ line.row.status === 'offline' ? '✕' : '!' }}
          </span>
          <span class="min-w-0 flex-1">
            <span class="tabular-nums text-muted-foreground">{{ fmtTime(line.row.ts) }}</span>
            <span
              class="ml-2 font-semibold uppercase"
              :class="line.row.status === 'offline' ? 'text-offline' : 'text-warning'"
            >
              {{ line.row.status === 'offline' ? 'down' : 'warn' }}
            </span>
            <span v-if="detail(line.row)" class="ml-2 tabular-nums text-muted-foreground">
              {{ detail(line.row) }}
            </span>
            <span v-if="line.row.error" class="ml-2 break-all text-muted-foreground">
              — {{ line.row.error }}
            </span>
          </span>
        </template>
      </li>
    </ul>

    <PagerControls
      v-if="lines.length > 0"
      :page="page"
      :has-prev="hasPrev"
      :has-next="hasNext"
      :loading="loadingMore"
      @prev="prevPage"
      @next="nextPage"
    />
  </div>
</template>
