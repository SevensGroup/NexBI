<script setup lang="ts">
import type { MCPHealth } from '@/types'

const map: Record<MCPHealth, string> = {
  healthy: 'text-emerald-400',
  degraded: 'text-amber-400',
  down: 'text-rose-400',
}

defineProps<{ health: MCPHealth }>()
</script>

<template>
  <span class="text-xs uppercase" :class="map[health]">{{ health }}</span>
</template>
