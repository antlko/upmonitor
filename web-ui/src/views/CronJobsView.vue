<script setup lang="ts">
import { BellRing, Send, Mail, MessageSquare, Clock } from '@lucide/vue'
import PageHeader from '@/components/layout/PageHeader.vue'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'

// This page is intentionally inert — a preview of what's coming next.
const features = [
  {
    icon: BellRing,
    title: 'Notifications',
    description: 'Alert rules for status changes, latency spikes and recovery.',
  },
  {
    icon: Send,
    title: 'Telegram',
    description: 'Push incident alerts to a Telegram chat or channel.',
  },
  {
    icon: Mail,
    title: 'Email',
    description: 'Send digest and incident emails via SMTP.',
  },
  {
    icon: MessageSquare,
    title: 'Discord',
    description: 'Post status updates to a Discord webhook.',
  },
]
</script>

<template>
  <div class="mx-auto max-w-4xl px-6 py-6 lg:px-8">
    <PageHeader title="Cron Jobs" description="Scheduled notifications and automations." />

    <div
      class="mt-6 flex items-center gap-3 rounded-xl border border-dashed border-border bg-card/50 px-4 py-3"
    >
      <div class="flex size-9 items-center justify-center rounded-lg bg-muted">
        <Clock class="size-4 text-muted-foreground" />
      </div>
      <div>
        <p class="text-sm font-medium">Coming soon</p>
        <p class="text-xs text-muted-foreground">
          These integrations are on the roadmap and not yet configurable.
        </p>
      </div>
    </div>

    <div class="mt-6 grid gap-3 sm:grid-cols-2">
      <div
        v-for="f in features"
        :key="f.title"
        class="pointer-events-none relative flex select-none flex-col gap-3 rounded-xl border border-border bg-card p-5 opacity-60"
      >
        <div class="flex items-start justify-between">
          <div class="flex size-10 items-center justify-center rounded-lg bg-muted">
            <component :is="f.icon" class="size-5 text-muted-foreground" />
          </div>
          <Switch :disabled="true" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h3 class="text-sm font-semibold">{{ f.title }}</h3>
            <Badge variant="secondary" class="text-[10px]">Soon</Badge>
          </div>
          <p class="mt-1 text-sm text-muted-foreground">{{ f.description }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
