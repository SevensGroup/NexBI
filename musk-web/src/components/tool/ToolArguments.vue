<script setup lang="ts">
import JsonViewer from '@/components/common/JsonViewer.vue'

defineProps<{ value: Record<string, unknown> }>()
</script>

<template>
  <div>
    <div class="mb-1 text-[11px] uppercase text-mute">Arguments</div>
    <JsonViewer :value="value" />
  </div>
</template>
