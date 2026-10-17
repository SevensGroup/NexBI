<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ query: unknown }>()
</script>

<template>
  <div>
    <div class="mb-2 text-sm font-medium">Semantic query</div>
    <JsonViewer :value="query" />
  </div>
</template>
