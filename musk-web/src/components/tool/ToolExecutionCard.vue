<script setup lang="ts">
import { ref } from 'vue'
import type { ToolCall } from '@/types'
import ToolStatusBadge from './ToolStatusBadge.vue'
import ToolArguments from './ToolArguments.vue'
import ToolResult from './ToolResult.vue'
import RiskBadge from '@/components/decision/RiskBadge.vue'
import { formatDuration } from '@/utils/format'
import { useAgentRunStore } from '@/stores/agentRun'

defineProps<{ tool: ToolCall }>()
const showArgs = ref(false)
const showResult = ref(false)
const run = useAgentRunStore()
</script>

<template>
  <article class="rounded-xl border border-line bg-panel p-4 text-sm">
    <div class="flex items-center justify-between gap-2">
      <span class="font-mono text-xs">{{ tool.toolName }}</span>
      <ToolStatusBadge :status="tool.status" />
    </div>
    <p class="mt-1 text-xs text-mute">{{ tool.description }}</p>
    <div class="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-mute">
      <span>Duration {{ formatDuration(tool.duration) }}</span>
      <RiskBadge :risk="tool.risk" />
    </div>
    <div class="mt-2 flex gap-2">
      <button class="text-[11px] text-accent" type="button" @click="showArgs = !showArgs">View Arguments</button>
      <button class="text-[11px] text-accent" type="button" @click="showResult = !showResult">View Result</button>
      <button v-if="tool.status === 'FAILED'" class="text-[11px] text-amber-300" type="button" @click="run.retryTool(tool.id)">
        Retry
      </button>
    </div>
    <ToolArguments v-if="showArgs" class="mt-2" :value="tool.arguments" />
    <ToolResult v-if="showResult" class="mt-2" :result="tool.result" :error="tool.error" />
  </article>
</template>
