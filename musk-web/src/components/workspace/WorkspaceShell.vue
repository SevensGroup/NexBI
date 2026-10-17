<script setup lang="ts">
import { computed } from 'vue'
import AgentInput from '@/components/agent/AgentInput.vue'
import AgentStateBadge from '@/components/agent/AgentStateBadge.vue'
import ExecutionTimeline from '@/components/execution/ExecutionTimeline.vue'
import ContextPanel from '@/components/workspace/ContextPanel.vue'
import PlanPanel from '@/components/task/PlanPanel.vue'
import TaskDetailDrawer from '@/components/task/TaskDetailDrawer.vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { startRun } from '@/composables/useAgentSession'

const props = withDefaults(defineProps<{ title?: string; agentId?: string }>(), {
  title: 'Agent Workspace',
  agentId: '',
})

const run = useAgentRunStore()
const badgeStatus = computed(() => (run.state.runId ? run.state.status : 'IDLE'))
const busy = computed(() => {
  const s = run.state.status
  const terminal = s === 'COMPLETED' || s === 'FAILED'
  return Boolean(run.state.runId) && run.state.events.length > 0 && !terminal && s !== 'WAITING_APPROVAL'
})

async function onSubmit(goal: string): Promise<void> {
  await startRun(goal, props.agentId)
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="flex items-center justify-between gap-4 border-b border-line px-6 py-4">
      <div class="min-w-0">
        <h1 class="truncate text-xl font-semibold tracking-tight">{{ title }}</h1>
        <p class="mt-1 truncate text-sm text-mute">{{ run.state.goal || 'No active goal' }}</p>
        <p v-if="run.state.runId" class="mt-1 truncate font-mono text-xs text-mute">{{ run.state.runId }}</p>
      </div>
      <AgentStateBadge :status="badgeStatus" :live="Boolean(run.state.runId)" />
    </div>
    <div class="grid min-h-0 flex-1 grid-cols-12">
      <aside class="col-span-3 min-h-0 overflow-auto border-r border-line p-3">
        <h2 class="mb-3 text-sm font-medium">Agents and tasks</h2>
        <slot name="left">
          <PlanPanel :plan="run.state.plan" />
        </slot>
      </aside>
      <section class="col-span-5 min-h-0 overflow-hidden border-r border-line">
        <ExecutionTimeline />
      </section>
      <aside class="col-span-4 min-h-0 overflow-hidden">
        <ContextPanel />
      </aside>
    </div>
    <AgentInput :disabled="busy && run.state.status !== 'WAITING_APPROVAL'" @submit="onSubmit" />
    <TaskDetailDrawer />
  </div>
</template>
