<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ query: unknown }>()
</script>

<template>
  <div>
    <div class="mb-1 text-[11px] uppercase text-mute">Semantic Query</div>
    <JsonViewer :value="query" />
  </div>
</template>
