<script setup lang="ts">
import type { MCPTool } from '@/types'

defineProps<{ tool: MCPTool }>()
</script>

<template>
  <div class="rounded-xl border border-line bg-raised p-3 text-sm">
    <div class="font-mono text-xs">{{ tool.name }}</div>
    <div class="text-xs text-mute">{{ tool.description }}</div>
  </div>
</template>
