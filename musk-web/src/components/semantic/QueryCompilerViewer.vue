<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ dsl: unknown; result?: unknown }>()
</script>

<template>
  <div class="grid gap-3 md:grid-cols-2">
    <div>
      <div class="mb-1 text-[11px] uppercase text-mute">Query Compiler → Elasticsearch DSL</div>
      <JsonViewer :value="dsl" />
    </div>
    <div>
      <div class="mb-1 text-[11px] uppercase text-mute">Result</div>
      <JsonViewer :value="result ?? { status: 'ready', note: 'Business semantics compiled; ES fields are an implementation detail.' }" />
    </div>
  </div>
</template>
