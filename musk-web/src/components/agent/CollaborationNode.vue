<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import type { AgentInfo } from '@/types'
import AgentStateBadge from './AgentStateBadge.vue'

defineProps<{ data: { agent: AgentInfo } }>()
</script>

<template>
  <div class="w-[220px] rounded border border-line bg-panel px-3 py-2 shadow">
    <Handle type="target" :position="Position.Top" />
    <div class="flex items-center justify-between">
      <strong class="text-sm">{{ data.agent.name }}</strong>
      <AgentStateBadge :status="data.agent.status" live />
    </div>
    <div class="mt-1 truncate text-[11px] text-mute">{{ data.agent.currentTask || 'No current task' }}</div>
    <div class="mt-1 flex justify-between text-[11px] text-mute">
      <span>{{ Math.round(data.agent.progress * 100) }}%</span>
      <span>{{ Math.round(data.agent.confidence * 100) }}% conf</span>
    </div>
    <Handle type="source" :position="Position.Bottom" />
  </div>
</template>
