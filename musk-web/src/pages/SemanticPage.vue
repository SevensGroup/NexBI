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

const tab = ref('metrics')
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
  <div class="h-full overflow-auto p-6">
    <h1 class="text-lg font-medium">Semantic Layer</h1>
    <p class="text-sm text-mute">Business semantics first. Elasticsearch fields are compiled output, not the agent interface.</p>
    <div class="mt-4 flex gap-1 border-b border-line">
      <button v-for="t in ['metrics', 'dimensions', 'entities', 'datasources', 'builder']" :key="t" class="px-3 py-2 text-xs" :class="tab === t ? 'border-b border-accent text-ink' : 'text-mute'" type="button" @click="tab = t">
        {{ t }}
      </button>
    </div>
    <div class="mt-4">
      <MetricExplorer v-if="tab === 'metrics'" :metrics="metrics" />
      <DimensionExplorer v-else-if="tab === 'dimensions'" :dimensions="dimensions" />
      <div v-else-if="tab === 'entities'" class="grid gap-2 md:grid-cols-3">
        <div v-for="e in MOCK_ENTITIES" :key="e.key" class="rounded border border-line bg-panel p-3 text-sm">
          <div class="font-medium">{{ e.name }}</div>
          <div class="text-xs text-mute">{{ e.description }}</div>
          <div class="mt-1 font-mono text-[11px]">{{ e.keyField }}</div>
        </div>
      </div>
      <div v-else-if="tab === 'datasources'" class="grid gap-2 md:grid-cols-2">
        <div v-for="d in MOCK_DATASOURCES" :key="d.key" class="rounded border border-line bg-panel p-3 text-sm">
          <div class="font-medium">{{ d.name }}</div>
          <div class="text-xs text-mute">{{ d.description }}</div>
          <div class="mt-1 font-mono text-[11px]">index {{ d.index }} · date {{ d.dateField }}</div>
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
</template>
