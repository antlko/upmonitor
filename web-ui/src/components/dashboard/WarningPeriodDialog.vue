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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const props = defineProps<{ open: boolean; hours: number }>()
const emit = defineEmits<{ 'update:open': [boolean]; submit: [number] }>()

const hours = ref(props.hours)
watch(
  () => props.open,
  (o) => {
    if (o) hours.value = props.hours
  },
)

function submit() {
  const n = Math.round(Number(hours.value))
  if (!Number.isFinite(n) || n < 1) return
  emit('submit', n)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-sm">
      <DialogHeader>
        <DialogTitle>Warning look-back period</DialogTitle>
        <DialogDescription>
          How far back the dashboard's "Warning" tile looks when counting services that
          had a check recover on a retry.
        </DialogDescription>
      </DialogHeader>
      <div class="space-y-1.5">
        <Label for="warning-period-hours">Hours</Label>
        <Input id="warning-period-hours" v-model.number="hours" type="number" min="1" step="1" />
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="emit('update:open', false)">Cancel</Button>
        <Button @click="submit">Save</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
