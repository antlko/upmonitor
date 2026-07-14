<script setup lang="ts">
import { ref, watch } from 'vue'
import { GridLayout, GridItem } from 'grid-layout-plus'
import ServiceCard from './ServiceCard.vue'
import { useServicesStore } from '@/stores/services'

interface GridItemData {
  i: string
  x: number
  y: number
  w: number
  h: number
}

const services = useServicesStore()
const props = withDefaults(defineProps<{ readonly?: boolean }>(), { readonly: false })
const emit = defineEmits<{
  edit: [id: string]
  replaceImage: [id: string]
  generateIcon: [id: string]
  remove: [id: string]
}>()

const layout = ref<GridItemData[]>([])

function buildLayout() {
  layout.value = services.services.map((s) => ({
    i: s.id,
    x: s.layout.x,
    y: s.layout.y,
    w: s.layout.w,
    h: s.layout.h,
  }))
}
buildLayout()

// Rebuild only when the set of services changes (add/remove); dragging keeps its own positions.
watch(
  () => services.services.map((s) => s.id).join(','),
  () => buildLayout(),
)

function onLayoutUpdated(newLayout: GridItemData[]) {
  if (props.readonly) return
  services.saveLayout(newLayout).catch(() => {})
}
</script>

<template>
  <GridLayout
    v-model:layout="layout"
    :col-num="12"
    :row-height="68"
    :margin="[16, 16]"
    :is-draggable="!props.readonly"
    :is-resizable="!props.readonly"
    :responsive="true"
    :cols="{ lg: 12, md: 12, sm: 8, xs: 4, xxs: 2 }"
    :breakpoints="{ lg: 1200, md: 900, sm: 640, xs: 480, xxs: 0 }"
    :vertical-compact="true"
    @layout-updated="onLayoutUpdated"
  >
    <GridItem
      v-for="item in layout"
      :key="item.i"
      :i="item.i"
      :x="item.x"
      :y="item.y"
      :w="item.w"
      :h="item.h"
      :min-w="1"
      :min-h="2"
    >
      <ServiceCard
        v-if="services.getById(item.i)"
        :service="services.getById(item.i)!"
        :readonly="props.readonly"
        @edit="emit('edit', item.i)"
        @replace-image="emit('replaceImage', item.i)"
        @generate-icon="emit('generateIcon', item.i)"
        @remove="emit('remove', item.i)"
      />
    </GridItem>
  </GridLayout>
</template>

<style scoped>
/* Drag placeholder — a soft accent ghost instead of the default red. */
:deep(.vgl-item--placeholder) {
  background: var(--color-primary);
  opacity: 0.1;
  border-radius: 0.75rem;
  transition: all 0.15s ease;
}

/* Smooth position/size transitions while not actively dragging. */
:deep(.vgl-item:not(.vgl-item--dragging)) {
  transition:
    transform 0.2s ease,
    width 0.2s ease,
    height 0.2s ease;
}

/* Resize handle — subtle, appears on hover. */
:deep(.vgl-item__resizer) {
  width: 18px;
  height: 18px;
  right: 3px;
  bottom: 3px;
  opacity: 0;
  transition: opacity 0.15s ease;
}
:deep(.vgl-item:hover .vgl-item__resizer) {
  opacity: 0.5;
}
:deep(.vgl-item__resizer::after) {
  border-color: var(--color-muted-foreground);
  width: 8px;
  height: 8px;
}
</style>
