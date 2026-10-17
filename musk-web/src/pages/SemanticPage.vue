<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import MetricExplorer from '@/components/semantic/MetricExplorer.vue'
import DimensionExplorer from '@/components/semantic/DimensionExplorer.vue'
import SemanticQueryBuilder from '@/components/semantic/SemanticQueryBuilder.vue'
import SemanticQueryPreview from '@/components/semantic/SemanticQueryPreview.vue'
import QueryCompilerViewer from '@/components/semantic/QueryCompilerViewer.vue'
import { MOCK_DATASOURCES, MOCK_ENTITIES, MOCK_METRICS, MOCK_DIMENSIONS } from '@/mocks/catalog'
import { listDimensions, listMetrics } from '@/services/api/agent'
import type { SemanticDimension, SemanticMetric } from '@/types'

defineProps<{ embedded?: boolean }>()

const tab = ref('metrics')
const tabs = [
  { id: 'metrics', label: 'Metrics' },
  { id: 'dimensions', label: 'Dimensions' },
  { id: 'entities', label: 'Entities' },
  { id: 'datasources', label: 'Data sources' },
  { id: 'builder', label: 'Builder' },
]
const metrics = ref<SemanticMetric[]>(MOCK_METRICS)
const dimensions = ref<SemanticDimension[]>(MOCK_DIMENSIONS)
const selectedMetrics = ref<string[]>(['roas', 'spend', 'revenue'])
const selectedDims = ref<string[]>(['campaign'])
const channel = ref('Meta')
const start = ref('2026-08-24')
const end = ref('2026-08-26')

const query = computed(() => ({
  metrics: selectedMetrics.value,
  dimensions: selectedDims.value,
  filters: [{ dimension: 'channel', op: 'eq', value: channel.value }],
  timeRange: { start: start.value, end: end.value, label: 'custom' },
}))

const dsl = computed(() => ({
  size: 0,
  query: {
    bool: {
      filter: [
        { term: { channel: channel.value } },
        { range: { date: { gte: start.value, lt: end.value } } },
      ],
    },
  },
  aggs: {
    by_dim: {
      terms: { field: selectedDims.value[0] === 'campaign' ? 'campaign_id' : selectedDims.value[0] || 'date' },
      aggs: Object.fromEntries(selectedMetrics.value.filter((m) => ['spend', 'revenue'].includes(m)).map((m) => [m, { sum: { field: m } }])),
    },
  },
}))

const result = computed(() => ({
  took: 37,
  aggregations: {
    by_dim: {
      buckets: [
        { key: 'cmp_us_prosp', spend: { value: 18400 }, revenue: { value: 13064 } },
        { key: 'cmp_us_rtg', spend: { value: 9600 }, revenue: { value: 29952 } },
      ],
    },
  },
}))

onMounted(async () => {
  try {
    metrics.value = await listMetrics()
    dimensions.value = await listDimensions()
  } catch {
    metrics.value = MOCK_METRICS
    dimensions.value = MOCK_DIMENSIONS
  }
})
</script>

<template>
  <div class="h-full overflow-auto">
    <div class="mx-auto flex max-w-5xl flex-col gap-8 px-6 py-8">
    <header v-if="!embedded">
      <h1 class="text-xl font-semibold tracking-tight">Semantic</h1>
      <p class="mt-1 text-sm text-mute">Business names first. Compiled fields stay behind the query.</p>
    </header>
    <div class="flex flex-wrap gap-2">
      <button
        v-for="t in tabs"
        :key="t.id"
        class="rounded-full border px-3 py-1 text-xs"
        :class="tab === t.id ? 'border-line bg-raised text-ink' : 'border-transparent text-mute hover:text-ink'"
        type="button"
        @click="tab = t.id"
      >
        {{ t.label }}
      </button>
    </div>
    <div>
      <MetricExplorer v-if="tab === 'metrics'" :metrics="metrics" />
      <DimensionExplorer v-else-if="tab === 'dimensions'" :dimensions="dimensions" />
      <div v-else-if="tab === 'entities'" class="grid gap-3 md:grid-cols-3">
        <div v-for="e in MOCK_ENTITIES" :key="e.key" class="rounded-xl border border-line bg-panel p-4 text-sm">
          <div class="font-medium">{{ e.name }}</div>
          <div class="mt-0.5 text-xs text-mute">{{ e.description }}</div>
          <div class="mt-2 font-mono text-xs text-mute">{{ e.keyField }}</div>
        </div>
      </div>
      <div v-else-if="tab === 'datasources'" class="grid gap-3 md:grid-cols-2">
        <div v-for="d in MOCK_DATASOURCES" :key="d.key" class="rounded-xl border border-line bg-panel p-4 text-sm">
          <div class="font-medium">{{ d.name }}</div>
          <div class="mt-0.5 text-xs text-mute">{{ d.description }}</div>
          <div class="mt-2 font-mono text-xs text-mute">index {{ d.index }} · date {{ d.dateField }}</div>
        </div>
      </div>
      <div v-else class="space-y-4">
        <SemanticQueryBuilder
          v-model:metrics="selectedMetrics"
          v-model:dimensions="selectedDims"
          v-model:channel="channel"
          v-model:start="start"
          v-model:end="end"
          :metric-options="metrics"
          :dimension-options="dimensions"
        />
        <SemanticQueryPreview :query="query" />
        <QueryCompilerViewer :dsl="dsl" :result="result" />
      </div>
    </div>
    </div>
  </div>
</template>
