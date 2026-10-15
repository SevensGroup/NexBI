<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import WorkspaceShell from '@/components/workspace/WorkspaceShell.vue'
import { replayRun } from '@/composables/useAgentSession'
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

watch(
  id,
  (next) => {
    if (next) void replayRun(next)
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="border-b border-line px-4 py-2 text-xs text-mute">
      <template v-if="sample">Sample catalog · {{ id }}</template>
      <template v-else>
        Run {{ id }}
        <span v-if="meta && isMockMode()" class="ml-3">
          {{ meta.llmCalls }} LLM · {{ meta.tokens.toLocaleString() }} tok · {{ formatUsd(meta.costUsd) }}
        </span>
        <span v-else-if="run.state.runId && run.state.status" class="ml-3">{{ run.state.status }}</span>
      </template>
    </div>
    <div v-if="sample" data-testid="sample-catalog-hint" class="border-b border-line bg-raised/40 px-4 py-3 text-sm text-mute">
      {{ SAMPLE_CATALOG_HINT }}
      <button class="ml-3 text-accent underline" type="button" @click="router.push('/dashboard')">Open Dashboard</button>
    </div>
    <div
      v-else-if="run.state.boardUrl"
      class="shrink-0 border-b border-line bg-raised/60 px-4 py-3 text-sm"
    >
      <span class="text-mute">看板地址</span>
      <a class="ml-3 text-accent underline" :href="run.state.boardUrl" target="_blank" rel="noreferrer">{{ run.state.boardUrl }}</a>
    </div>
    <div
      v-else-if="failMessage"
      data-testid="run-failure-banner"
      class="shrink-0 border-b border-rose-900/70 bg-rose-950/50 px-4 py-2 text-sm text-rose-200"
    >
      <span class="font-medium">FAILED</span>
      <span class="ml-2 whitespace-pre-wrap break-words">{{ failMessage }}</span>
    </div>
    <WorkspaceShell title="Run" />
  </div>
</template>
