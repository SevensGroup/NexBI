<script setup lang="ts">
import { computed } from 'vue'
import type { SemanticDimension, SemanticMetric } from '@/types'

const metrics = defineModel<string[]>('metrics', { default: () => [] })
const dimensions = defineModel<string[]>('dimensions', { default: () => [] })
const channel = defineModel<string>('channel', { default: 'Meta' })
const start = defineModel<string>('start', { default: '2026-08-24' })
const end = defineModel<string>('end', { default: '2026-08-26' })

defineProps<{ metricOptions: SemanticMetric[]; dimensionOptions: SemanticDimension[] }>()

const query = computed(() => ({
  metrics: metrics.value,
  dimensions: dimensions.value,
  filters: channel.value ? [{ dimension: 'channel', op: 'eq', value: channel.value }] : [],
  timeRange: { start: start.value, end: end.value, label: 'custom' },
}))

defineExpose({ query })
</script>

<template>
  <div class="space-y-3 text-sm">
    <div>
      <div class="mb-1 text-xs text-mute">Metrics</div>
      <div class="flex flex-wrap gap-2">
        <label v-for="m in metricOptions" :key="m.key" class="flex items-center gap-1 text-xs">
          <input v-model="metrics" type="checkbox" :value="m.key" />
          {{ m.name }}
        </label>
      </div>
    </div>
    <div>
      <div class="mb-1 text-xs text-mute">Dimensions</div>
      <div class="flex flex-wrap gap-2">
        <label v-for="d in dimensionOptions" :key="d.key" class="flex items-center gap-1 text-xs">
          <input v-model="dimensions" type="checkbox" :value="d.key" />
          {{ d.name }}
        </label>
      </div>
    </div>
    <div class="flex flex-wrap gap-3">
      <label class="text-xs">Filter channel
        <select v-model="channel" class="ml-1 rounded border border-line bg-canvas px-2 py-1">
          <option>Meta</option>
          <option>Google</option>
          <option>TikTok</option>
        </select>
      </label>
      <label class="text-xs">Start <input v-model="start" class="ml-1 rounded border border-line bg-canvas px-2 py-1" type="date" /></label>
      <label class="text-xs">End <input v-model="end" class="ml-1 rounded border border-line bg-canvas px-2 py-1" type="date" /></label>
    </div>
  </div>
</template>
