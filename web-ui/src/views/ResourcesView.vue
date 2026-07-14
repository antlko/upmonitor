<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Plus, Pencil, Trash2, ArrowUpRight, Boxes } from '@lucide/vue'
import PageHeader from '@/components/layout/PageHeader.vue'
import ServiceIcon from '@/components/dashboard/ServiceIcon.vue'
import StatusDot from '@/components/dashboard/StatusDot.vue'
import ServiceFormDialog from '@/components/services/ServiceFormDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { toast } from '@/components/ui/sonner'
import { useServicesStore, type ServiceInput } from '@/stores/services'
import { ApiError } from '@/api'
import type { Service } from '@/types'
import { prettyUrl, statusLabel } from '@/lib/format'

const services = useServicesStore()

onMounted(() => {
  if (!services.loaded) services.fetchServices()
})

function errMsg(e: unknown) {
  return e instanceof ApiError ? e.message : 'Something went wrong'
}

const formOpen = ref(false)
const formService = ref<Service | null>(null)
const confirmOpen = ref(false)
const confirmService = ref<Service | null>(null)

const modeLabel: Record<string, string> = { icon: 'Icon', name: 'Name', dashboard: 'Dashboard' }

function openAdd() {
  formService.value = null
  formOpen.value = true
}
function openEdit(s: Service) {
  formService.value = s
  formOpen.value = true
}
function openDelete(s: Service) {
  confirmService.value = s
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
</script>

<template>
  <div class="mx-auto max-w-5xl px-6 py-6 lg:px-8">
    <PageHeader title="Resources" description="Manage the services you monitor. Saved to config.yaml.">
      <template #actions>
        <Button v-if="services.hasServices" @click="openAdd">
          <Plus />
          Add service
        </Button>
      </template>
    </PageHeader>

    <div v-if="services.hasServices" class="mt-6 overflow-hidden rounded-xl border border-border bg-card">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-border text-left text-xs text-muted-foreground">
            <th class="px-4 py-3 font-medium">Service</th>
            <th class="hidden px-4 py-3 font-medium sm:table-cell">Interval</th>
            <th class="hidden px-4 py-3 font-medium md:table-cell">Mode</th>
            <th class="px-4 py-3 font-medium">Status</th>
            <th class="px-4 py-3 text-right font-medium">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="s in services.services"
            :key="s.id"
            class="border-b border-border/60 transition-colors last:border-0 hover:bg-accent/40"
          >
            <td class="px-4 py-3">
              <div class="flex items-center gap-3">
                <ServiceIcon :service="s" class="size-8 shrink-0" />
                <div class="min-w-0">
                  <p class="truncate font-medium">{{ s.name }}</p>
                  <p class="truncate text-xs text-muted-foreground">{{ prettyUrl(s.url) }}</p>
                </div>
              </div>
            </td>
            <td class="hidden px-4 py-3 tabular-nums text-muted-foreground sm:table-cell">
              {{ s.check.interval }}s
            </td>
            <td class="hidden px-4 py-3 md:table-cell">
              <Badge variant="secondary">{{ modeLabel[s.widget.mode] }}</Badge>
            </td>
            <td class="px-4 py-3">
              <span class="inline-flex items-center gap-1.5">
                <StatusDot :status="s.status" :pulse="false" />
                <span class="text-xs">{{ statusLabel(s.status) }}</span>
              </span>
            </td>
            <td class="px-4 py-3">
              <div class="flex items-center justify-end gap-1">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon-sm" as-child>
                      <a :href="s.url" target="_blank" rel="noopener" aria-label="Open service">
                        <ArrowUpRight />
                      </a>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Open</TooltipContent>
                </Tooltip>
                <Button variant="ghost" size="icon-sm" aria-label="Edit" @click="openEdit(s)">
                  <Pencil />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="text-muted-foreground hover:text-offline"
                  aria-label="Delete"
                  @click="openDelete(s)"
                >
                  <Trash2 />
                </Button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="mt-6 rounded-xl border border-border bg-card">
      <EmptyState
        :icon="Boxes"
        title="No services yet"
        description="Add your first service to start monitoring it."
      >
        <Button @click="openAdd">
          <Plus />
          Add first service
        </Button>
      </EmptyState>
    </div>

    <ServiceFormDialog v-model:open="formOpen" :service="formService" @submit="onSubmit" />
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
