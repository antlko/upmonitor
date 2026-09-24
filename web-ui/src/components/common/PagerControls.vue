<script setup lang="ts">
import { ChevronLeft, ChevronRight, LoaderCircle } from '@lucide/vue'
import { Button } from '@/components/ui/button'

/**
 * A small Previous/Next pager shared by anything paginating server data in
 * fixed-size pages — the ping console (cursor-based, so `totalPages` is
 * omitted) and the incidents table (offset-based, so it's known upfront).
 */
withDefaults(
  defineProps<{
    page: number
    totalPages?: number
    hasPrev: boolean
    hasNext: boolean
    loading?: boolean
  }>(),
  { loading: false },
)
defineEmits<{ prev: []; next: [] }>()
</script>

<template>
  <div class="flex items-center justify-between gap-3 border-t border-border/60 px-3 py-2">
    <Button variant="ghost" size="sm" :disabled="!hasPrev || loading" @click="$emit('prev')">
      <ChevronLeft />
      Previous
    </Button>
    <span class="text-xs tabular-nums text-muted-foreground">
      Page {{ page }}<template v-if="totalPages"> of {{ totalPages }}</template>
    </span>
    <Button variant="ghost" size="sm" :disabled="!hasNext || loading" @click="$emit('next')">
      <LoaderCircle v-if="loading" class="animate-spin" />
      Next
      <ChevronRight />
    </Button>
  </div>
</template>
