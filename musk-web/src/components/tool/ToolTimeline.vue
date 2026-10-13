<script setup lang="ts">
import { computed } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { formatTime } from '@/utils/format'

const run = useAgentRunStore()
const rows = computed(() => {
  const out: Array<{ t: string; label: string }> = []
  for (const d of run.state.decisions) out.push({ t: d.timestamp, label: `Decision ${d.type}` })
  for (const tc of run.state.toolCalls) {
    out.push({ t: tc.timestamp, label: `Tool Started ${tc.toolName}` })
    if (tc.status === 'COMPLETED') out.push({ t: tc.timestamp, label: `Tool Completed ${tc.toolName}` })
  }
  for (const o of run.state.observations) out.push({ t: o.timestamp, label: `Observation ${o.summary}` })
  return out.sort((a, b) => a.t.localeCompare(b.t))
})
</script>

<template>
  <ol class="space-y-1 text-xs">
    <li v-for="(row, i) in rows" :key="i" class="flex gap-2">
      <span class="w-14 font-mono text-mute">{{ formatTime(row.t) }}</span>
      <span class="truncate">{{ row.label }}</span>
    </li>
  </ol>
</template>
