<script setup lang="ts">
import { computed } from 'vue'
import { MoreHorizontal, Pencil, ImageUp, Sparkles, Trash2, ArrowUpRight } from '@lucide/vue'
import type { Service } from '@/types'
import ServiceIcon from './ServiceIcon.vue'
import StatusDot from './StatusDot.vue'
import SparklineChart from './SparklineChart.vue'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'
import { statusLabel, formatLatency, formatUptime, timeAgo, prettyUrl } from '@/lib/format'

const props = withDefaults(defineProps<{ service: Service; readonly?: boolean }>(), {
  readonly: false,
})
const emit = defineEmits<{ edit: []; replaceImage: []; generateIcon: []; remove: [] }>()

const s = computed(() => props.service)

const borderClass = computed(
  () =>
    ({
      online: 'border-online/30 hover:border-online/50',
      offline: 'border-offline/40 hover:border-offline/60',
      unknown: 'border-border hover:border-muted-foreground/25',
    })[s.value.status],
)
const statusPill = computed(
  () =>
    ({
      online: 'bg-online/10 text-online',
      offline: 'bg-offline/10 text-offline',
      unknown: 'bg-unknown/15 text-muted-foreground',
    })[s.value.status],
)
const sparkColor = computed(
  () =>
    ({
      online: 'var(--color-online)',
      offline: 'var(--color-offline)',
      unknown: 'var(--color-unknown)',
    })[s.value.status],
)

function openService() {
  window.open(s.value.url, '_blank', 'noopener')
}
</script>

<template>
  <div
    :class="
      cn(
        'group/card relative flex h-full flex-col overflow-hidden rounded-xl border bg-card transition-all duration-200 hover:shadow-elevation-medium',
        borderClass,
      )
    "
  >
    <!-- Actions menu (shared across modes) -->
    <div v-if="!readonly" class="absolute right-1.5 top-1.5 z-10">
      <DropdownMenu>
        <DropdownMenuTrigger
          class="no-drag flex size-7 items-center justify-center rounded-md text-muted-foreground opacity-0 outline-none transition-all hover:bg-accent hover:text-foreground focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring/40 group-hover/card:opacity-100 data-[state=open]:bg-accent data-[state=open]:opacity-100"
          aria-label="Service actions"
          @pointerdown.stop
        >
          <MoreHorizontal class="size-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-48">
          <DropdownMenuItem @select="openService">
            <ArrowUpRight />
            Open service
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @select="emit('edit')">
            <Pencil />
            Edit
          </DropdownMenuItem>
          <DropdownMenuItem @select="emit('replaceImage')">
            <ImageUp />
            Replace image
          </DropdownMenuItem>
          <DropdownMenuItem @select="emit('generateIcon')">
            <Sparkles />
            Generate icon
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" @select="emit('remove')">
            <Trash2 />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>

    <!-- Mode: icon only -->
    <div
      v-if="s.widget.mode === 'icon'"
      class="flex h-full flex-col items-center justify-center gap-2 p-3"
    >
      <div class="relative">
        <ServiceIcon :service="s" class="size-14 shadow-elevation-low" />
        <span
          class="absolute -bottom-1 -right-1 flex items-center justify-center rounded-full border-2 border-card bg-card p-0.5"
        >
          <StatusDot :status="s.status" size="md" :pulse="s.status === 'online'" />
        </span>
      </div>
    </div>

    <!-- Mode: icon + name -->
    <div v-else-if="s.widget.mode === 'name'" class="flex h-full items-center gap-3 p-3.5">
      <ServiceIcon :service="s" class="size-10 shrink-0" />
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-1.5">
          <StatusDot :status="s.status" />
          <span class="truncate text-sm font-semibold">{{ s.name }}</span>
        </div>
        <p class="mt-0.5 truncate text-xs text-muted-foreground">{{ prettyUrl(s.url) }}</p>
      </div>
    </div>

    <!-- Mode: icon + name + mini dashboard -->
    <div v-else class="flex h-full flex-col p-4">
      <header class="flex items-start gap-3">
        <ServiceIcon :service="s" class="size-10 shrink-0" />
        <div class="min-w-0 flex-1 pr-6">
          <span class="block truncate text-sm font-semibold leading-tight">{{ s.name }}</span>
          <p class="mt-0.5 truncate text-xs text-muted-foreground">{{ prettyUrl(s.url) }}</p>
        </div>
      </header>

      <div class="mt-3">
        <span
          :class="
            cn('inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium', statusPill)
          "
        >
          <StatusDot :status="s.status" :pulse="s.status === 'online'" />
          {{ statusLabel(s.status) }}
        </span>
      </div>

      <div class="mt-3.5 grid grid-cols-2 gap-x-4 gap-y-3">
        <div>
          <p class="text-[11px] uppercase tracking-wide text-muted-foreground/70">Response</p>
          <p class="mt-0.5 text-sm font-semibold tabular-nums">{{ formatLatency(s.latencyMs) }}</p>
        </div>
        <div>
          <p class="text-[11px] uppercase tracking-wide text-muted-foreground/70">Uptime</p>
          <p class="mt-0.5 text-sm font-semibold tabular-nums">
            {{ s.status === 'unknown' ? '—' : formatUptime(s.uptime) }}
          </p>
        </div>
      </div>

      <div class="mt-auto pt-4">
        <div v-if="s.latencyHistory.length" class="-mx-1">
          <SparklineChart :values="s.latencyHistory" :color="sparkColor" class="w-full" :height="36" />
        </div>
        <div class="mt-1.5 flex items-center justify-between text-[11px] text-muted-foreground">
          <span>{{ s.latencyHistory.length ? 'Response time · 24 checks' : 'Awaiting data' }}</span>
          <span>{{ timeAgo(s.lastCheck) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
