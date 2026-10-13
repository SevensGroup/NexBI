<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ result: unknown; latencyMs?: number; status?: string }>()
</script>

<template>
  <div>
    <div class="mb-1 flex justify-between text-[11px] uppercase text-mute">
      <span>Elasticsearch Result</span>
      <span>{{ status }} · {{ latencyMs ?? 0 }}ms</span>
    </div>
    <JsonViewer :value="result" />
  </div>
</template>
