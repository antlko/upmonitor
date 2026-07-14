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
import type { Service, WidgetMode } from '@/types'
import type { ServiceInput } from '@/stores/services'

const props = defineProps<{ open: boolean; service?: Service | null }>()
const emit = defineEmits<{ 'update:open': [boolean]; submit: [ServiceInput] }>()

const isEdit = computed(() => !!props.service)

const name = ref('')
const url = ref('')
const interval = ref(30)
const mode = ref<WidgetMode>('name')

function reset() {
  name.value = props.service?.name ?? ''
  url.value = props.service?.url ?? ''
  interval.value = props.service?.check.interval ?? 30
  mode.value = props.service?.widget.mode ?? 'name'
}
watch(
  () => props.open,
  (o) => o && reset(),
)

const valid = computed(() => name.value.trim().length > 0 && /^https?:\/\/.+/.test(url.value.trim()))

function submit() {
  if (!valid.value) return
  emit('submit', {
    name: name.value.trim(),
    url: url.value.trim(),
    interval: Number(interval.value) || 30,
    mode: mode.value,
  })
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ isEdit ? 'Edit service' : 'Add service' }}</DialogTitle>
        <DialogDescription>
          {{ isEdit ? 'Update how this service is monitored and displayed.' : 'Monitor a new web service. Changes are written to config.yaml.' }}
        </DialogDescription>
      </DialogHeader>

      <form class="grid gap-4" @submit.prevent="submit">
        <div class="grid gap-2">
          <Label for="svc-name">Name</Label>
          <Input id="svc-name" v-model="name" placeholder="Grafana" autocomplete="off" />
        </div>
        <div class="grid gap-2">
          <Label for="svc-url">URL</Label>
          <Input id="svc-url" v-model="url" placeholder="https://grafana.home.lab" autocomplete="off" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="grid gap-2">
            <Label for="svc-interval">Check interval</Label>
            <div class="relative">
              <Input id="svc-interval" v-model="interval" type="number" min="5" class="pr-8" />
              <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">s</span>
            </div>
          </div>
          <div class="grid gap-2">
            <Label>Widget mode</Label>
            <Select v-model="mode">
              <SelectTrigger>
                <SelectValue placeholder="Select mode" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="icon">Icon only</SelectItem>
                <SelectItem value="name">Icon + name</SelectItem>
                <SelectItem value="dashboard">Mini dashboard</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </form>

      <DialogFooter>
        <Button variant="ghost" @click="emit('update:open', false)">Cancel</Button>
        <Button :disabled="!valid" @click="submit">{{ isEdit ? 'Save changes' : 'Add service' }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
