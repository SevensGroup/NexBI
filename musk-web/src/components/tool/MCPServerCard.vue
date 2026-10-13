<script setup lang="ts">
import type { MCPServer } from '@/types'
import MCPHealthBadge from './MCPHealthBadge.vue'

defineProps<{ server: MCPServer }>()
</script>

<template>
  <div class="rounded border border-line bg-panel p-3">
    <div class="flex items-center justify-between">
      <div class="font-medium">{{ server.name }}</div>
      <MCPHealthBadge :health="server.health" />
    </div>
    <p class="mt-1 text-xs text-mute">{{ server.description }}</p>
    <div class="mt-2 text-[11px] text-mute">Latency {{ server.latencyMs }}ms</div>
  </div>
</template>
