<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RefreshCw, Sparkles } from '@lucide/vue'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { generateIconDataUrl, generateIconSvg, ICON_STYLES, type IconStyle } from '@/lib/icon-generator'
import type { Service } from '@/types'
import { cn } from '@/lib/utils'

const props = defineProps<{ open: boolean; service?: Service | null }>()
const emit = defineEmits<{ 'update:open': [boolean]; apply: [string] }>()

const style = ref<IconStyle>('gradient')
const seeds = ref<string[]>([])
const selected = ref(0)

const name = computed(() => props.service?.name ?? 'Service')
const baseId = computed(() => props.service?.id ?? 'service')

function randomSeed() {
  return `${baseId.value}-${Math.random().toString(36).slice(2, 8)}`
}
function shuffle() {
  seeds.value = [seeds.value[0] ?? baseId.value, randomSeed(), randomSeed(), randomSeed()]
  selected.value = 0
}
watch(
  () => props.open,
  (o) => {
    if (!o) return
    style.value = 'gradient'
    seeds.value = [baseId.value, randomSeed(), randomSeed(), randomSeed()]
    selected.value = 0
  },
)

const preview = computed(() =>
  generateIconDataUrl(seeds.value[selected.value] ?? baseId.value, style.value, name.value),
)
const thumbs = computed(() =>
  seeds.value.map((s) => ({ seed: s, url: generateIconDataUrl(s, style.value, name.value) })),
)

function apply() {
  // Emit the raw SVG so the caller can rasterize + upload it as WebP.
  const svg = generateIconSvg(seeds.value[selected.value] ?? baseId.value, style.value, name.value)
  emit('apply', svg)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <Sparkles class="size-4 text-primary" />
          Generate icon
        </DialogTitle>
        <DialogDescription>
          A unique icon generated on-device from “{{ name }}”. No external services.
        </DialogDescription>
      </DialogHeader>

      <div class="flex flex-col items-center gap-5">
        <!-- Preview -->
        <div class="rounded-2xl border border-border bg-muted/30 p-4">
          <img
            :src="preview"
            :alt="`${name} generated icon`"
            class="size-24 rounded-2xl shadow-elevation-medium"
          />
        </div>

        <!-- Style segmented control -->
        <div class="inline-flex rounded-lg bg-muted p-1">
          <button
            v-for="opt in ICON_STYLES"
            :key="opt.value"
            type="button"
            :class="
              cn(
                'rounded-md px-3 py-1.5 text-sm font-medium transition-all',
                style === opt.value
                  ? 'bg-card text-foreground shadow-elevation-low'
                  : 'text-muted-foreground hover:text-foreground',
              )
            "
            @click="style = opt.value"
          >
            {{ opt.label }}
          </button>
        </div>

        <!-- Variations -->
        <div class="flex items-center gap-3">
          <button
            v-for="(t, i) in thumbs"
            :key="t.seed"
            type="button"
            :class="
              cn(
                'rounded-xl p-0.5 outline-none transition-all',
                selected === i
                  ? 'ring-2 ring-primary ring-offset-2 ring-offset-card'
                  : 'opacity-70 hover:opacity-100',
              )
            "
            @click="selected = i"
          >
            <img :src="t.url" alt="Icon variation" class="size-11 rounded-lg" />
          </button>
        </div>
      </div>

      <DialogFooter class="sm:justify-between">
        <Button variant="outline" @click="shuffle">
          <RefreshCw />
          Shuffle
        </Button>
        <div class="flex gap-2">
          <Button variant="ghost" @click="emit('update:open', false)">Cancel</Button>
          <Button @click="apply">Apply icon</Button>
        </div>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
