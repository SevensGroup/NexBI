<script setup lang="ts">
import { computed } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { useUiStore } from '@/stores/ui'
import DecisionCard from '@/components/decision/DecisionCard.vue'
import ReflectionCard from '@/components/reflection/ReflectionCard.vue'
import ToolExecutionCard from '@/components/tool/ToolExecutionCard.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'
import { timelineDetailCopy } from '@/utils/timeline'
import type { Decision } from '@/types/decision'
import type { Observation, ToolCall } from '@/types/tool'
import type { Reflection } from '@/types/reflection'

const run = useAgentRunStore()
const ui = useUiStore()

const open = computed({
  get: () => ui.timelineDrawerOpen && Boolean(run.selectedEvent),
  set: (v: boolean) => {
    ui.timelineDrawerOpen = v
  },
})

const event = computed(() => run.selectedEvent)
const copy = computed(() => timelineDetailCopy(event.value))
const decision = computed(() => (event.value?.payload.decision as Decision | undefined) ?? null)
const reflection = computed(() => (event.value?.payload.reflection as Reflection | undefined) ?? null)
const observation = computed(() => (event.value?.payload.observation as Observation | undefined) ?? null)
const tool = computed(() => {
  const ev = event.value
  if (!ev) return null
  if (ev.payload.toolCall) return ev.payload.toolCall as ToolCall
  const id = ev.payload.toolCallId
  if (typeof id === 'string') return run.state.toolCalls.find((t) => t.id === id) ?? null
  return null
})
const errorText = computed(() => {
  if (event.value?.type === 'agent.failed' || event.value?.type === 'tool.failed') {
    return String(event.value.payload.error ?? run.state.error ?? '')
  }
  return ''
})
</script>

<template>
  <el-drawer v-model="open" title="Event detail" size="480px" append-to-body>
    <div v-if="event" data-testid="timeline-detail" class="space-y-3 text-sm">
      <div class="flex items-center justify-between gap-2 text-[11px] text-mute">
        <span class="font-mono text-accent">{{ event.type }}</span>
        <span>{{ event.timestamp }}</span>
      </div>
      <div v-if="errorText" class="rounded border border-rose-900 bg-rose-950/50 p-2 text-rose-200">
        {{ errorText }}
      </div>
      <DecisionCard v-if="decision" :decision="decision" />
      <ReflectionCard v-else-if="reflection" :reflection="reflection" />
      <ToolExecutionCard v-else-if="tool" :tool="tool" />
      <p v-else-if="observation" class="whitespace-pre-wrap leading-relaxed">
        {{ observation.summary }}
        <span v-if="observation.reason" class="mt-2 block text-mute">{{ observation.reason }}</span>
      </p>
      <pre v-else class="whitespace-pre-wrap break-words font-sans text-sm leading-relaxed">{{ copy }}</pre>
      <div>
        <div class="mb-1 text-[11px] uppercase text-mute">Full payload</div>
        <JsonViewer :value="event.payload" max-height="320px" />
      </div>
    </div>
  </el-drawer>
</template>
