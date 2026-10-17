<script setup lang="ts">
import type { AgentInfo } from '@/types'
import AgentStateBadge from './AgentStateBadge.vue'
import { formatPct } from '@/utils/format'

defineProps<{ agent: AgentInfo }>()
const emit = defineEmits<{ click: [id: string] }>()
</script>

<template>
  <button
    type="button"
    class="w-full rounded-xl border border-line bg-panel p-4 text-left hover:border-accent/50"
    @click="emit('click', agent.id)"
  >
    <div class="flex items-center justify-between gap-2">
      <div class="font-medium">{{ agent.name }}</div>
      <AgentStateBadge :status="agent.status === 'IDLE' ? 'IDLE' : agent.status" />
    </div>
    <div class="mt-1 text-xs text-mute">{{ agent.role }} · {{ agent.description }}</div>
    <div v-if="agent.currentTask" class="mt-2 truncate text-xs">Task: {{ agent.currentTask }}</div>
    <div class="mt-2 flex items-center gap-3 text-[11px] text-mute">
      <span>Progress {{ Math.round(agent.progress * 100) }}%</span>
      <span>Conf {{ formatPct(agent.confidence, 0) }}</span>
    </div>
  </button>
</template>
