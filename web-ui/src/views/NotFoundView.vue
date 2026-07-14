<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { ArrowLeft } from '@lucide/vue'
import BrandMark from '@/components/common/BrandMark.vue'
import { Button } from '@/components/ui/button'
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center gap-6 bg-background px-6 text-center">
    <BrandMark class="size-11" />
    <div>
      <p class="text-5xl font-semibold tracking-tight tabular-nums">404</p>
      <p class="mt-2 text-sm text-muted-foreground">This page could not be found.</p>
    </div>
    <Button as-child variant="outline">
      <RouterLink to="/">
        <ArrowLeft />
        Back to dashboard
      </RouterLink>
    </Button>
  </div>
</template>
