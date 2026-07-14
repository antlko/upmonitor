<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{ icon?: Component; title: string; description?: string }>()
</script>

<template>
  <div class="flex flex-col items-center justify-center px-6 py-20 text-center">
    <div
      class="mb-5 flex size-14 items-center justify-center rounded-2xl border border-border bg-card shadow-elevation-low"
    >
      <component :is="icon" v-if="icon" class="size-6 text-muted-foreground" />
    </div>
    <h3 class="text-base font-semibold tracking-tight">{{ title }}</h3>
    <p v-if="description" class="mt-1.5 max-w-sm text-sm text-muted-foreground">
      {{ description }}
    </p>
    <div v-if="$slots.default" class="mt-6">
      <slot />
    </div>
  </div>
</template>
