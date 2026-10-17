<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ query: unknown; dsl: unknown; request: unknown }>()
</script>

<template>
  <div class="grid gap-3 md:grid-cols-3">
    <div>
      <div class="mb-2 text-sm font-medium">Semantic query</div>
      <JsonViewer :value="query" />
    </div>
    <div>
      <div class="mb-2 text-sm font-medium">Generated DSL</div>
      <JsonViewer :value="dsl" />
    </div>
    <div>
      <div class="mb-2 text-sm font-medium">MCP request</div>
      <JsonViewer :value="request" />
    </div>
  </div>
</template>
