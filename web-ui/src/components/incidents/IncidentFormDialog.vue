<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/ui/select'
import { useServicesStore } from '@/stores/services'
import type { Incident } from '@/types'
import type { IncidentInput } from '@/stores/incidents'

const props = defineProps<{ open: boolean; incident?: Incident | null }>()
const emit = defineEmits<{ 'update:open': [boolean]; submit: [IncidentInput] }>()

const services = useServicesStore()
const isEdit = computed(() => !!props.incident)

const serviceId = ref('')
const title = ref('')
const startedAt = ref('')
const resolvedAt = ref('')

function toLocalInput(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
function fromLocalInput(v: string): string | undefined {
  return v ? new Date(v).toISOString() : undefined
}

function reset() {
  serviceId.value = props.incident?.serviceId ?? services.services[0]?.id ?? ''
  title.value = props.incident?.title ?? ''
  startedAt.value = toLocalInput(props.incident?.startedAt)
  resolvedAt.value = toLocalInput(props.incident?.resolvedAt)
}
watch(
  () => props.open,
  (o) => o && reset(),
)

const valid = computed(() => isEdit.value || serviceId.value !== '')

function submit() {
  if (!valid.value) return
  const payload: IncidentInput = {
    title: title.value.trim(),
    startedAt: fromLocalInput(startedAt.value),
    resolvedAt: fromLocalInput(resolvedAt.value),
  }
  if (!isEdit.value) payload.serviceId = serviceId.value
  emit('submit', payload)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ isEdit ? 'Edit incident' : 'Create incident' }}</DialogTitle>
        <DialogDescription>
          {{
            isEdit
              ? 'Adjust this incident’s details or log when it ended.'
              : 'Manually record an incident (e.g. a planned outage the monitor can’t detect).'
          }}
        </DialogDescription>
      </DialogHeader>

      <form class="grid gap-4" @submit.prevent="submit">
        <div v-if="!isEdit" class="grid gap-2">
          <Label>Service</Label>
          <Select v-model="serviceId">
            <SelectTrigger>
              <SelectValue placeholder="Select a service" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="s in services.services" :key="s.id" :value="s.id">
                {{ s.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="grid gap-2">
          <Label for="inc-title">Title</Label>
          <Input id="inc-title" v-model="title" placeholder="Planned maintenance" autocomplete="off" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="grid gap-2">
            <Label for="inc-started">Started at</Label>
            <Input id="inc-started" v-model="startedAt" type="datetime-local" />
          </div>
          <div class="grid gap-2">
            <Label for="inc-resolved">Resolved at</Label>
            <Input id="inc-resolved" v-model="resolvedAt" type="datetime-local" />
          </div>
        </div>
        <p class="text-xs text-muted-foreground">
          Leave “Started at” empty to use the current time. Setting “Resolved at” marks the incident
          resolved.
        </p>
      </form>

      <DialogFooter>
        <Button variant="ghost" @click="emit('update:open', false)">Cancel</Button>
        <Button :disabled="!valid" @click="submit">
          {{ isEdit ? 'Save changes' : 'Create incident' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
