<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ result: unknown; latencyMs?: number; status?: string }>()
</script>

<template>
  <div>
    <div class="mb-2 flex items-baseline justify-between gap-3">
      <span class="text-sm font-medium">Result</span>
      <span class="font-mono text-xs text-mute">{{ status }} · {{ latencyMs ?? 0 }}ms</span>
    </div>
    <JsonViewer :value="result" />
  </div>
</template>
