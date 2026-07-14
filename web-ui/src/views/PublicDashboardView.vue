<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Lock, LoaderCircle } from '@lucide/vue'
import BrandMark from '@/components/common/BrandMark.vue'
import ServiceCard from '@/components/dashboard/ServiceCard.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { api, ApiError } from '@/api'
import type { Service } from '@/types'

const services = ref<Service[]>([])
const loaded = ref(false)
const disabled = ref(false)
let timer: ReturnType<typeof setInterval> | undefined

async function load() {
  try {
    services.value = await api.publicServices()
    disabled.value = false
  } catch (e) {
    if (e instanceof ApiError && e.status === 403) disabled.value = true
  } finally {
    loaded.value = true
  }
}

onMounted(() => {
  load()
  timer = setInterval(load, 15_000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})

function cardClass(mode: string) {
  return mode === 'dashboard' ? 'h-72 sm:col-span-2' : 'h-40'
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <header
      class="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-border bg-background/80 px-5 backdrop-blur-xl"
    >
      <BrandMark class="size-7" />
      <span class="font-semibold tracking-tight">upmonitor</span>
      <span class="rounded-full border border-border px-2 py-0.5 text-xs text-muted-foreground">
        Status
      </span>
      <div class="ml-auto">
        <ThemeToggle />
      </div>
    </header>

    <main class="mx-auto max-w-[1400px] px-6 py-8">
      <div v-if="!loaded" class="flex justify-center py-24 text-muted-foreground">
        <LoaderCircle class="size-5 animate-spin" />
      </div>
      <div v-else-if="disabled" class="flex flex-col items-center gap-3 py-24 text-center">
        <Lock class="size-6 text-muted-foreground" />
        <p class="text-sm text-muted-foreground">The public dashboard is disabled.</p>
      </div>
      <div v-else class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        <div v-for="s in services" :key="s.id" :class="cardClass(s.widget.mode)">
          <ServiceCard :service="s" readonly />
        </div>
      </div>
    </main>
  </div>
</template>
