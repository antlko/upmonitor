<script setup lang="ts">
import { computed, type Component } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

const props = defineProps<{
  to: string
  label: string
  icon: Component
  collapsed: boolean
  badge?: string
}>()

const route = useRoute()
// Exact match for the dashboard root, prefix match for the rest.
const active = computed(() =>
  props.to === '/' ? route.path === '/' : route.path.startsWith(props.to),
)
</script>

<template>
  <Tooltip :delay-duration="0">
    <TooltipTrigger as-child>
      <RouterLink
        :to="to"
        :class="
          cn(
            'group relative flex h-9 items-center gap-3 rounded-lg text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring/40',
            collapsed ? 'justify-center px-0' : 'px-3',
            active
              ? 'bg-sidebar-accent text-sidebar-accent-foreground'
              : 'text-sidebar-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground',
          )
        "
      >
        <span
          v-if="active"
          class="absolute -left-2 top-1/2 h-5 w-1 -translate-y-1/2 rounded-full bg-primary"
        />
        <component
          :is="icon"
          class="size-[18px] shrink-0 transition-transform group-active:scale-90"
        />
        <span v-if="!collapsed" class="truncate">{{ label }}</span>
        <span
          v-if="badge && !collapsed"
          class="ml-auto rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground"
        >
          {{ badge }}
        </span>
      </RouterLink>
    </TooltipTrigger>
    <TooltipContent v-if="collapsed" side="right" class="font-medium">
      {{ label }}
      <span v-if="badge" class="ml-1.5 text-muted-foreground">· {{ badge }}</span>
    </TooltipContent>
  </Tooltip>
</template>
