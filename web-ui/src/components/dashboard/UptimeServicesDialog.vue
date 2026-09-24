<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import type { Service } from '@/types'

const props = defineProps<{ open: boolean; services: Service[]; excludedIds: string[] }>()
const emit = defineEmits<{ 'update:open': [boolean]; submit: [string[]] }>()

// Tracked as "included", the inverse of the stored excluded list — a switch
// per row reads more naturally as "counted" than as "excluded".
const included = ref<Set<string>>(new Set())
watch(
  () => props.open,
  (o) => {
    if (!o) return
    const excluded = new Set(props.excludedIds)
    included.value = new Set(props.services.filter((s) => !excluded.has(s.id)).map((s) => s.id))
  },
)

function toggle(id: string, on: boolean) {
  const next = new Set(included.value)
  if (on) next.add(id)
  else next.delete(id)
  included.value = next
}

function submit() {
  const excluded = props.services.filter((s) => !included.value.has(s.id)).map((s) => s.id)
  emit('submit', excluded)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-sm">
      <DialogHeader>
        <DialogTitle>Services counted in Avg uptime</DialogTitle>
        <DialogDescription>
          Choose which services count toward the dashboard's "Avg uptime" tile.
        </DialogDescription>
      </DialogHeader>
      <ul class="max-h-72 space-y-1 overflow-y-auto">
        <li
          v-for="svc in services"
          :key="svc.id"
          class="flex items-center justify-between gap-3 rounded-md px-1.5 py-1.5"
        >
          <span class="min-w-0 truncate text-sm">{{ svc.name }}</span>
          <Switch
            :model-value="included.has(svc.id)"
            @update:model-value="(v: boolean) => toggle(svc.id, v)"
          />
        </li>
        <li v-if="services.length === 0" class="px-1.5 py-1.5 text-sm text-muted-foreground">
          No services yet.
        </li>
      </ul>
      <DialogFooter>
        <Button variant="ghost" @click="emit('update:open', false)">Cancel</Button>
        <Button @click="submit">Save</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
