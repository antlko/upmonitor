<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Pencil, CheckCircle2, Trash2, LoaderCircle, Send } from '@lucide/vue'
import StatusDot from '@/components/dashboard/StatusDot.vue'
import IncidentFormDialog from '@/components/incidents/IncidentFormDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { toast } from '@/components/ui/sonner'
import { useIncidentsStore, type IncidentInput } from '@/stores/incidents'
import { useAuthStore } from '@/stores/auth'
import { api, ApiError } from '@/api'
import type { IncidentDetail } from '@/types'
import { initials } from '@/lib/format'

const route = useRoute()
const router = useRouter()
const store = useIncidentsStore()
const auth = useAuthStore()

const id = computed(() => Number(route.params.id))
const detail = ref<IncidentDetail | null>(null)
const loading = ref(true)
const editOpen = ref(false)
const confirmOpen = ref(false)
const commentBody = ref('')
const posting = ref(false)

function errMsg(e: unknown) {
  return e instanceof ApiError ? e.message : 'Something went wrong'
}

async function load() {
  loading.value = true
  try {
    detail.value = await store.getDetail(id.value)
  } catch {
    detail.value = null
  } finally {
    loading.value = false
  }
}
onMounted(load)

const durationText = computed(() => {
  const inc = detail.value
  if (!inc) return ''
  const end = inc.resolvedAt ? new Date(inc.resolvedAt).getTime() : Date.now()
  const mins = Math.max(1, Math.round((end - new Date(inc.startedAt).getTime()) / 60000))
  if (mins < 60) return `${mins} min`
  const hrs = Math.floor(mins / 60)
  return hrs < 24 ? `${hrs}h ${mins % 60}m` : `${Math.floor(hrs / 24)}d ${hrs % 24}h`
})

function fmtDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

async function onEdit(input: IncidentInput) {
  try {
    await store.update(id.value, input)
    toast.success('Incident updated')
    load()
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function onResolve() {
  try {
    await store.resolve(id.value)
    toast.success('Incident resolved')
    load()
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function onDelete() {
  try {
    await store.remove(id.value)
    toast.success('Incident deleted')
    router.push('/incidents')
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function postComment() {
  const body = commentBody.value.trim()
  if (!body || !detail.value) return
  posting.value = true
  try {
    const cm = await api.addIncidentComment(id.value, body)
    detail.value.comments.push(cm)
    commentBody.value = ''
  } catch (e) {
    toast.error(errMsg(e))
  } finally {
    posting.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-[820px] px-6 py-6 lg:px-8">
    <button
      class="mb-4 inline-flex cursor-pointer items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
      @click="router.push('/incidents')"
    >
      <ArrowLeft class="size-4" />
      Incidents
    </button>

    <div v-if="loading" class="flex justify-center py-24 text-muted-foreground">
      <LoaderCircle class="size-5 animate-spin" />
    </div>
    <div v-else-if="!detail" class="py-24 text-center text-muted-foreground">
      This incident no longer exists.
    </div>

    <template v-else>
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <StatusDot
              :status="detail.status === 'ongoing' ? 'offline' : 'online'"
              :pulse="detail.status === 'ongoing'"
            />
            <h2 class="truncate text-2xl font-semibold tracking-tight">
              {{ detail.title || (detail.status === 'ongoing' ? 'Ongoing outage' : 'Outage') }}
            </h2>
          </div>
          <p class="mt-1 text-sm text-muted-foreground">
            <RouterLink :to="`/services/${detail.serviceId}`" class="text-primary hover:underline">
              {{ detail.serviceName }}
            </RouterLink>
            · {{ detail.source === 'auto' ? 'Auto-detected' : 'Manually logged' }}
          </p>
        </div>
        <div v-if="auth.isAdmin" class="flex items-center gap-2">
          <Button variant="outline" size="sm" @click="editOpen = true">
            <Pencil />
            Edit
          </Button>
          <Button v-if="detail.status === 'ongoing'" variant="outline" size="sm" @click="onResolve">
            <CheckCircle2 />
            Resolve
          </Button>
          <Button variant="outline" size="sm" @click="confirmOpen = true">
            <Trash2 />
            Delete
          </Button>
        </div>
      </header>

      <!-- Timeline -->
      <div class="mt-6 grid grid-cols-3 gap-3">
        <div class="rounded-xl border border-border bg-card px-4 py-3">
          <p class="text-xs text-muted-foreground">Started</p>
          <p class="mt-1 text-sm font-semibold tabular-nums">{{ fmtDateTime(detail.startedAt) }}</p>
        </div>
        <div class="rounded-xl border border-border bg-card px-4 py-3">
          <p class="text-xs text-muted-foreground">Resolved</p>
          <p class="mt-1 text-sm font-semibold tabular-nums">
            {{ detail.resolvedAt ? fmtDateTime(detail.resolvedAt) : '—' }}
          </p>
        </div>
        <div class="rounded-xl border border-border bg-card px-4 py-3">
          <p class="text-xs text-muted-foreground">Duration</p>
          <p class="mt-1 text-sm font-semibold tabular-nums">{{ durationText }}</p>
        </div>
      </div>

      <!-- Comments -->
      <Card class="mt-6">
        <CardHeader>
          <CardTitle class="text-sm">Comments</CardTitle>
        </CardHeader>
        <CardContent>
          <p v-if="detail.comments.length === 0" class="text-sm text-muted-foreground">
            No comments yet.
          </p>
          <ul v-else class="space-y-4">
            <li v-for="cm in detail.comments" :key="cm.id" class="flex gap-3">
              <span
                class="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/15 text-xs font-medium text-primary"
              >
                {{ initials(cm.username || '?') }}
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex items-baseline gap-2">
                  <span class="text-sm font-medium">{{ cm.username || 'Unknown' }}</span>
                  <span class="text-xs text-muted-foreground tabular-nums">
                    {{ fmtDateTime(cm.createdAt) }}
                  </span>
                </div>
                <p class="mt-0.5 whitespace-pre-wrap text-sm">{{ cm.body }}</p>
              </div>
            </li>
          </ul>

          <form class="mt-4 flex items-end gap-2" @submit.prevent="postComment">
            <Textarea
              v-model="commentBody"
              placeholder="Add a comment…"
              class="min-h-10 flex-1"
              @keydown.enter.meta.prevent="postComment"
            />
            <Button type="submit" :disabled="posting || !commentBody.trim()">
              <Send />
              Post
            </Button>
          </form>
        </CardContent>
      </Card>
    </template>

    <IncidentFormDialog v-if="detail" v-model:open="editOpen" :incident="detail" @submit="onEdit" />
    <ConfirmDialog
      v-model:open="confirmOpen"
      title="Delete this incident?"
      description="This permanently removes the incident and its comments."
      confirm-label="Delete"
      destructive
      @confirm="onDelete"
    />
  </div>
</template>
