<script setup lang="ts">
import { computed } from 'vue'
import { useConnectionStore } from '@/stores/connection'

const conn = useConnectionStore()
const label = computed(() => conn.status)
const color = computed(() => {
  if (conn.status === 'Connected') return 'bg-emerald-400'
  if (conn.status === 'Reconnecting') return 'bg-amber-400 pulse-soft'
  return 'bg-zinc-500'
})
</script>

<template>
  <div data-testid="backend-connection" class="flex items-center gap-2 text-xs text-mute" title="musk-back GET /health (REST). Not SSE.">
    <span class="status-dot" :class="color" />
    <span>{{ label }}</span>
  </div>
</template>
