<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Plus, Boxes, LoaderCircle } from '@lucide/vue'
import ServiceGrid from '@/components/dashboard/ServiceGrid.vue'
import ServiceFormDialog from '@/components/services/ServiceFormDialog.vue'
import IconGeneratorDialog from '@/components/services/IconGeneratorDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { Button } from '@/components/ui/button'
import { toast } from '@/components/ui/sonner'
import { useServicesStore, type ServiceInput } from '@/stores/services'
import { useAuthStore } from '@/stores/auth'
import { useServicesPolling } from '@/composables/useServicesPolling'
import { optimizeToWebP, svgToWebP } from '@/lib/image'
import { ApiError } from '@/api'
import type { ChartType, Service, WidgetMode } from '@/types'
import { formatUptime } from '@/lib/format'
import { cn } from '@/lib/utils'

const services = useServicesStore()
const auth = useAuthStore()
useServicesPolling()

const formOpen = ref(false)
const formService = ref<Service | null>(null)
const iconOpen = ref(false)
const iconService = ref<Service | null>(null)
const confirmOpen = ref(false)
const confirmService = ref<Service | null>(null)

const fileInput = ref<HTMLInputElement>()
const imageTargetId = ref<string | null>(null)
const hoveredServiceId = ref<string | null>(null)

const showEmpty = computed(() => services.loaded && !services.hasServices)
const stats = computed(() => [
  { label: 'Services', value: String(services.services.length) },
  { label: 'Online', value: String(services.onlineCount), dot: 'bg-online' },
  { label: 'Offline', value: String(services.offlineCount), dot: 'bg-offline' },
  { label: 'Avg uptime', value: formatUptime(services.avgUptime) },
])

function errMsg(e: unknown) {
  return e instanceof ApiError ? e.message : 'Something went wrong'
}

function openAdd() {
  formService.value = null
  formOpen.value = true
}
function openEdit(id: string) {
  formService.value = services.getById(id) ?? null
  formOpen.value = true
}
function openGenerate(id: string) {
  iconService.value = services.getById(id) ?? null
  iconOpen.value = true
}
function openReplace(id: string) {
  imageTargetId.value = id
  fileInput.value?.click()
}
function openDelete(id: string) {
  confirmService.value = services.getById(id) ?? null
  confirmOpen.value = true
}

async function onSubmit(input: ServiceInput) {
  try {
    if (formService.value) {
      await services.updateService(formService.value.id, input)
      toast.success('Service updated')
    } else {
      await services.addService(input)
      toast.success('Service added')
    }
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function onApplyIcon(svg: string) {
  if (!iconService.value) return
  try {
    const blob = await svgToWebP(svg)
    await services.uploadIcon(iconService.value.id, blob)
    toast.success('Icon applied')
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function onConfirmDelete() {
  if (!confirmService.value) return
  const name = confirmService.value.name
  try {
    await services.removeService(confirmService.value.id)
    toast.success(`${name} removed`)
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function uploadImageFor(id: string, file: File) {
  try {
    const blob = await optimizeToWebP(file)
    await services.uploadIcon(id, blob)
    toast.success('Image updated')
  } catch (err) {
    toast.error(errMsg(err))
  }
}
async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (file && imageTargetId.value) await uploadImageFor(imageTargetId.value, file)
  input.value = ''
}
async function onSetWidgetMode(id: string, mode: WidgetMode) {
  try {
    await services.setWidgetMode(id, mode)
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function onSetChartType(id: string, type: ChartType) {
  try {
    await services.setChartType(id, type)
  } catch (e) {
    toast.error(errMsg(e))
  }
}
function onHover(id: string, entering: boolean) {
  if (entering) hoveredServiceId.value = id
  else if (hoveredServiceId.value === id) hoveredServiceId.value = null
}

// Ctrl+V an image over the hovered card to replace its icon.
function onPaste(e: ClipboardEvent) {
  const el = document.activeElement as HTMLElement | null
  if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)) return
  const id = hoveredServiceId.value
  if (!id) return
  for (const item of e.clipboardData?.items ?? []) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) {
        e.preventDefault()
        uploadImageFor(id, file)
      }
      return
    }
  }
}
onMounted(() => window.addEventListener('paste', onPaste))
onUnmounted(() => window.removeEventListener('paste', onPaste))
</script>

<template>
  <div class="mx-auto max-w-[1600px] px-6 py-6 lg:px-8">
    <header class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h2 class="text-2xl font-semibold tracking-tight">Dashboard</h2>
        <p class="mt-1 text-sm text-muted-foreground">
          Live status of your services · drag to rearrange, resize from the corner.
        </p>
      </div>
      <Button v-if="services.hasServices && auth.isAdmin" @click="openAdd">
        <Plus />
        Add service
      </Button>
    </header>

    <!-- Initial loading -->
    <div v-if="!services.loaded" class="flex items-center justify-center py-24 text-muted-foreground">
      <LoaderCircle class="size-5 animate-spin" />
    </div>

    <template v-else>
      <div v-if="services.hasServices" class="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div
          v-for="stat in stats"
          :key="stat.label"
          class="rounded-xl border border-border bg-card px-4 py-3"
        >
          <div class="flex items-center gap-1.5">
            <span v-if="stat.dot" :class="cn('size-2 rounded-full', stat.dot)" />
            <p class="text-xs text-muted-foreground">{{ stat.label }}</p>
          </div>
          <p class="mt-1 text-xl font-semibold tabular-nums">{{ stat.value }}</p>
        </div>
      </div>

      <div class="mt-4">
        <ServiceGrid
          v-if="services.hasServices"
          class="-mx-2"
          :readonly="!auth.isAdmin"
          @edit="openEdit"
          @replace-image="openReplace"
          @generate-icon="openGenerate"
          @remove="openDelete"
          @set-widget-mode="onSetWidgetMode"
          @set-chart-type="onSetChartType"
          @drop-image="uploadImageFor"
          @hover="onHover"
        />
        <EmptyState
          v-else-if="showEmpty"
          :icon="Boxes"
          title="No services yet"
          :description="
            auth.isAdmin
              ? 'Add your first service to start monitoring its status, response time and uptime.'
              : 'No services have been added yet.'
          "
        >
          <Button v-if="auth.isAdmin" @click="openAdd">
            <Plus />
            Add first service
          </Button>
        </EmptyState>
      </div>
    </template>

    <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onFileChange" />

    <ServiceFormDialog v-model:open="formOpen" :service="formService" @submit="onSubmit" />
    <IconGeneratorDialog v-model:open="iconOpen" :service="iconService" @apply="onApplyIcon" />
    <ConfirmDialog
      v-model:open="confirmOpen"
      :title="`Delete ${confirmService?.name ?? 'service'}?`"
      description="This removes the service and its history. This action cannot be undone."
      confirm-label="Delete"
      destructive
      @confirm="onConfirmDelete"
    />
  </div>
</template>
