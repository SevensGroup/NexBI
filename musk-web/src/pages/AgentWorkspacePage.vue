<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import WorkspaceShell from '@/components/workspace/WorkspaceShell.vue'
import AgentCard from '@/components/agent/AgentCard.vue'
import PlanPanel from '@/components/task/PlanPanel.vue'
import { MOCK_AGENTS } from '@/mocks/catalog'
import { useAgentRunStore } from '@/stores/agentRun'

const route = useRoute()
const run = useAgentRunStore()
const id = computed(() => String(route.params.id || 'coordinator'))
const agent = computed(() => MOCK_AGENTS.find((a) => a.id === id.value) ?? MOCK_AGENTS[0])
</script>

<template>
  <WorkspaceShell :title="`${agent.name} Workspace`" :agent-id="agent.id">
    <template #left>
      <AgentCard :agent="{ ...agent, status: run.state.status, currentTask: run.state.plan?.tasks.find((t) => t.status === 'RUNNING')?.title, progress: 0.4, confidence: run.latestDecision?.confidence ?? 0 }" />
      <div class="mt-3">
        <PlanPanel :plan="run.state.plan" />
      </div>
    </template>
  </WorkspaceShell>
</template>
