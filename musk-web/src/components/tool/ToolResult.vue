<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ result?: unknown; error?: string }>()
</script>

<template>
  <div>
    <div class="mb-1 text-[11px] uppercase text-mute">{{ error ? 'Error' : 'Result' }}</div>
    <div v-if="error" class="rounded border border-rose-900 bg-rose-950/40 p-2 text-xs text-rose-300">{{ error }}</div>
    <JsonViewer v-else :value="result ?? null" />
  </div>
</template>
