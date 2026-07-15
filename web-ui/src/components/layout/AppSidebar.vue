<script setup lang="ts">
import { computed } from 'vue'
import { LayoutDashboard, Boxes, TriangleAlert, Bell, Settings, PanelLeft } from '@lucide/vue'
import BrandMark from '@/components/common/BrandMark.vue'
import SidebarNavItem from './SidebarNavItem.vue'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { useUiStore } from '@/stores/ui'
import { useServicesStore } from '@/stores/services'
import { useAuthStore } from '@/stores/auth'
import { cn } from '@/lib/utils'

const ui = useUiStore()
const services = useServicesStore()
const auth = useAuthStore()

interface NavItem {
  to: string
  label: string
  icon: typeof LayoutDashboard
  badge?: string
}

// Incidents are visible to everyone (comments come from any user); admin-only
// pages (Resources, Integrations, Settings) are gated.
const nav = computed<NavItem[]>(() => {
  const items: NavItem[] = [{ to: '/', label: 'Dashboard', icon: LayoutDashboard }]
  if (auth.isAdmin) items.push({ to: '/resources', label: 'Resources', icon: Boxes })
  items.push({ to: '/incidents', label: 'Incidents', icon: TriangleAlert })
  if (auth.isAdmin) {
    items.push(
      { to: '/integrations', label: 'Integrations', icon: Bell },
      { to: '/settings', label: 'Settings', icon: Settings },
    )
  }
  return items
})

const collapsed = computed(() => ui.sidebarCollapsed)
</script>

<template>
  <aside
    :class="
      cn(
        'relative z-20 flex h-full flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-300 ease-[cubic-bezier(0.32,0.72,0,1)]',
        collapsed ? 'w-[68px]' : 'w-60',
      )
    "
  >
    <!-- Brand + collapse toggle -->
    <div class="flex h-14 items-center gap-2.5 px-3.5">
      <RouterLink to="/" class="flex items-center gap-2.5 overflow-hidden outline-none">
        <BrandMark class="size-8 shrink-0" />
        <Transition name="brand">
          <span v-if="!collapsed" class="text-[15px] font-semibold tracking-tight whitespace-nowrap">
            upmonitor
          </span>
        </Transition>
      </RouterLink>
      <Tooltip :delay-duration="0">
        <TooltipTrigger as-child>
          <button
            :class="
              cn(
                'ml-auto flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground outline-none transition-colors hover:bg-sidebar-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/40',
                collapsed && 'absolute -right-3 top-4 border border-border bg-card shadow-elevation-low hover:bg-card',
              )
            "
            aria-label="Toggle sidebar"
            @click="ui.toggleSidebar()"
          >
            <PanelLeft class="size-4 transition-transform" :class="collapsed && 'rotate-180'" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="right">{{ collapsed ? 'Expand' : 'Collapse' }}</TooltipContent>
      </Tooltip>
    </div>

    <!-- Navigation -->
    <nav class="flex flex-1 flex-col gap-1 px-2.5 pt-3">
      <p
        v-if="!collapsed"
        class="px-2.5 pb-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/70"
      >
        Menu
      </p>
      <SidebarNavItem
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        :label="item.label"
        :icon="item.icon"
        :badge="item.badge"
        :collapsed="collapsed"
      />
    </nav>

    <!-- Status summary footer -->
    <div class="border-t border-sidebar-border p-2.5">
      <div
        :class="
          cn(
            'flex items-center gap-2 rounded-lg bg-sidebar-accent/50 px-3 py-2.5',
            collapsed && 'flex-col gap-1.5 px-0 py-2',
          )
        "
      >
        <span class="flex items-center gap-1.5" title="Online">
          <span class="size-2 rounded-full bg-online" />
          <span v-if="!collapsed" class="text-xs font-medium tabular-nums">{{ services.onlineCount }}</span>
        </span>
        <span class="flex items-center gap-1.5" title="Offline">
          <span class="size-2 rounded-full bg-offline" />
          <span v-if="!collapsed" class="text-xs font-medium tabular-nums">{{ services.offlineCount }}</span>
        </span>
        <span class="flex items-center gap-1.5" title="Unknown">
          <span class="size-2 rounded-full bg-unknown" />
          <span v-if="!collapsed" class="text-xs font-medium tabular-nums">{{ services.unknownCount }}</span>
        </span>
        <span v-if="!collapsed" class="ml-auto text-[11px] text-muted-foreground">monitored</span>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.brand-enter-active,
.brand-leave-active {
  transition:
    opacity 0.2s ease,
    transform 0.2s ease;
}
.brand-enter-from,
.brand-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}
</style>
