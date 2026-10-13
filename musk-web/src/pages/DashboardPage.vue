<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import MetricCard from '@/components/common/MetricCard.vue'
import PageState from '@/components/common/PageState.vue'
import { DEFAULT_GOAL, MOCK_DASHBOARD, MOCK_RUNS } from '@/mocks/catalog'
import { formatDuration, formatPct, formatUsd } from '@/utils/format'
import { isMockMode } from '@/services/api/client'
import { startRun } from '@/composables/useAgentSession'
import { useAgentRunStore } from '@/stores/agentRun'

const router = useRouter()
const run = useAgentRunStore()
const loading = ref(true)
const error = ref<string | null>(null)
const starting = ref(false)
const goal = ref(DEFAULT_GOAL)
const stats = ref(MOCK_DASHBOARD)
const mock = isMockMode()

const liveHint = computed(() =>
  mock ? 'Mock replay' : 'Live musk-back · POST /api/v1/agent/run { message }',
)

onMounted(() => {
  loading.value = false
})

async function startFromDashboard(): Promise<void> {
  const message = goal.value.trim() || DEFAULT_GOAL
  starting.value = true
  error.value = null
  try {
    await router.push('/agents/advertising-analyst')
    await startRun(message, 'advertising-analyst')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to start agent run'
  } finally {
    starting.value = false
  }
}
</script>

<template>
  <div class="h-full overflow-auto p-6">
    <h1 class="text-lg font-medium">Dashboard</h1>
    <p class="mt-1 text-sm text-mute">Autonomous Agent Operating Console · {{ liveHint }}</p>
    <form class="mt-4 flex max-w-3xl items-end gap-2" @submit.prevent="startFromDashboard">
      <label class="flex-1 text-xs text-mute">
        Goal
        <textarea
          v-model="goal"
          rows="2"
          class="mt-1 w-full resize-none rounded border border-line bg-canvas px-3 py-2 text-sm text-ink"
          data-testid="dashboard-goal"
        />
      </label>
      <button
        data-testid="dashboard-run"
        class="h-10 rounded bg-accent px-4 text-sm text-white disabled:opacity-40"
        type="submit"
        :disabled="starting"
      >
        {{ starting ? 'Running…' : 'Run' }}
      </button>
    </form>
    <p v-if="run.state.runId" class="mt-2 font-mono text-[11px] text-mute">
      last run {{ run.state.runId }} · {{ run.state.status }}
    </p>
    <PageState :loading="loading" :error="error">
      <div v-if="mock" class="mt-6 grid gap-3 md:grid-cols-4 lg:grid-cols-7">
        <MetricCard label="Active Agents" :value="stats.activeAgents" />
        <MetricCard label="Completed Today" :value="stats.completedToday" />
        <MetricCard label="Success Rate" :value="formatPct(stats.successRate, 0)" />
        <MetricCard label="Avg Runtime" :value="formatDuration(stats.averageRuntimeMs)" />
        <MetricCard label="LLM Calls" :value="stats.llmCalls" />
        <MetricCard label="Tokens" :value="stats.tokens.toLocaleString()" />
        <MetricCard label="Cost" :value="formatUsd(stats.costUsd)" />
      </div>
      <div v-else class="mt-6 text-sm text-mute">
        Live run metrics come from GET /api/v1/runs/:id (tokens / cost on the persisted run). musk-back has no run-list endpoint yet.
      </div>
      <h2 class="mt-8 text-sm uppercase tracking-wide text-mute">Recent Runs</h2>
      <table v-if="mock || run.history.length" class="mt-2 w-full text-left text-sm">
        <thead class="text-xs text-mute">
          <tr>
            <th class="py-2">Run</th>
            <th>Goal</th>
            <th>Status</th>
            <th v-if="mock">Steps</th>
            <th v-if="mock">Cost</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in mock ? MOCK_RUNS : run.history"
            :key="r.id"
            class="cursor-pointer border-t border-line hover:bg-raised"
            @click="router.push(`/runs/${r.id}`)"
          >
            <td class="py-2 font-mono text-xs">{{ r.id }}</td>
            <td class="max-w-md truncate">{{ r.goal }}</td>
            <td>{{ r.status }}</td>
            <td v-if="mock">{{ r.steps }}</td>
            <td v-if="mock">{{ formatUsd(r.costUsd) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="mt-2 text-sm text-mute">Submit a goal to start a live run. The console will open /runs/&lt;uuid&gt; from musk-back, not a sample id.</p>
    </PageState>
  </div>
</template>
