<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AgentInput from '@/components/agent/AgentInput.vue'
import AgentStateBadge from '@/components/agent/AgentStateBadge.vue'
import ReasoningPanel from '@/components/agent/ReasoningPanel.vue'
import BoardPanel from '@/components/board/BoardPanel.vue'
import { replayRun, startRun } from '@/composables/useAgentSession'
import { MOCK_RUNS } from '@/mocks/catalog'
import { formatUsd } from '@/utils/format'
import { isMockMode } from '@/services/api/client'
import { useAgentRunStore } from '@/stores/agentRun'
import { isLiveSampleCatalog, SAMPLE_CATALOG_HINT, runFailureBanner } from '@/utils/runDisplay'

const route = useRoute()
const router = useRouter()
const run = useAgentRunStore()
const id = computed(() => String(route.params.id || ''))
const sample = computed(() => isLiveSampleCatalog(id.value, isMockMode()))
const meta = computed(() => {
  if (isMockMode()) return MOCK_RUNS.find((r) => r.id === id.value)
  return run.history.find((r) => r.id === id.value)
})
const failMessage = computed(() => runFailureBanner(run.state.status, run.state.error))
const badgeStatus = computed(() => (run.state.runId ? run.state.status : 'IDLE'))
const busy = computed(() => {
  const status = run.state.status
  const terminal = status === 'COMPLETED' || status === 'FAILED'
  return Boolean(run.state.runId) && run.state.events.length > 0 && !terminal && status !== 'WAITING_APPROVAL'
})

watch(
  id,
  (next) => {
    if (next) void replayRun(next)
  },
  { immediate: true },
)

async function onSubmit(goal: string): Promise<void> {
  await startRun(goal)
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <header class="flex shrink-0 items-start justify-between gap-4 border-b border-line px-6 py-4">
      <div class="min-w-0">
        <h1 class="truncate text-xl font-semibold tracking-tight">{{ run.state.goal || 'Run' }}</h1>
        <p class="mt-1 text-sm text-mute">
          <template v-if="sample">Sample catalog · {{ id }}</template>
          <template v-else>
            <span class="font-mono text-xs">{{ id }}</span>
            <span v-if="meta && isMockMode()" class="ml-3">
              {{ meta.llmCalls }} LLM · {{ meta.tokens.toLocaleString() }} tok · {{ formatUsd(meta.costUsd) }}
            </span>
          </template>
        </p>
      </div>
      <AgentStateBadge v-if="!sample" :status="badgeStatus" :live="Boolean(run.state.runId)" />
    </header>
    <div v-if="sample" data-testid="sample-catalog-hint" class="border-b border-line px-6 py-4 text-sm text-mute">
      {{ SAMPLE_CATALOG_HINT }}
      <button class="ml-3 text-accent underline" type="button" @click="router.push('/dashboard')">Open Dashboard</button>
    </div>
    <div
      v-else-if="failMessage"
      data-testid="run-failure-banner"
      class="shrink-0 border-b border-rose-900/70 bg-rose-950/50 px-4 py-2 text-sm text-rose-200"
    >
      <span class="font-medium">FAILED</span>
      <span class="ml-2 whitespace-pre-wrap break-words">{{ failMessage }}</span>
    </div>
    <div v-if="!sample" class="grid min-h-0 flex-1 grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
      <div class="min-h-0 border-r border-line">
        <ReasoningPanel />
      </div>
      <div class="min-h-0">
        <BoardPanel :url="run.state.boardUrl" />
      </div>
    </div>
    <AgentInput v-if="!sample" :disabled="busy && run.state.status !== 'WAITING_APPROVAL'" @submit="onSubmit" />
  </div>
</template>
