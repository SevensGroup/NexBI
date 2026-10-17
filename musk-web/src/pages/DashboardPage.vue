<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import MetricCard from '@/components/common/MetricCard.vue'
import PageState from '@/components/common/PageState.vue'
import { DEFAULT_GOAL, MOCK_DASHBOARD, MOCK_RUNS } from '@/mocks/catalog'
import { formatDateTime, formatDuration, formatPct, formatUsd } from '@/utils/format'
import { isMockMode } from '@/services/api/client'
import { startRun } from '@/composables/useAgentSession'
import { useAgentRunStore } from '@/stores/agentRun'

const router = useRouter()
const run = useAgentRunStore()
const loading = ref(true)
const error = ref<string | null>(null)
const starting = ref(false)
const suggestions = [
  DEFAULT_GOAL,
  'Compare spend, revenue, and ROAS by channel',
  'Find campaigns with high spend and low return',
]
const goal = ref(DEFAULT_GOAL)
const stats = ref(MOCK_DASHBOARD)
const mock = isMockMode()

const liveHint = computed(() => (mock ? 'Mock replay' : 'Backend connected'))

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
  <div class="h-full overflow-auto">
    <div class="mx-auto flex max-w-5xl flex-col gap-8 px-6 py-8">
      <header>
        <h1 class="text-xl font-semibold tracking-tight">Dashboard</h1>
        <p class="mt-1 text-sm text-mute">Start a run from one sentence. {{ liveHint }}.</p>
      </header>

      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">Goal</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">The agent queries the data, then writes a board.</p>
        </div>
        <form class="overflow-hidden rounded-xl border border-line bg-panel p-4" @submit.prevent="startFromDashboard">
          <label class="text-sm" for="dashboard-goal">What should we look at?</label>
          <textarea
            id="dashboard-goal"
            v-model="goal"
            rows="3"
            class="mt-3 w-full resize-none rounded-xl border border-line bg-canvas px-4 py-3 text-sm leading-6 text-ink outline-none focus:border-accent"
            data-testid="dashboard-goal"
          />
          <div class="mt-3 flex flex-wrap gap-2">
            <button
              v-for="item in suggestions"
              :key="item"
              class="rounded-full border border-line bg-raised px-3 py-1 text-xs text-mute hover:text-ink"
              type="button"
              @click="goal = item"
            >
              {{ item }}
            </button>
          </div>
          <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
            <p class="max-w-xl text-xs leading-5 text-mute">
              The default goal is “Build me an advertising data board”. The finished board opens at /bi/&lt;code&gt; in this app.
            </p>
            <button
              data-testid="dashboard-run"
              class="h-10 rounded-xl bg-accent px-5 text-sm font-medium text-white disabled:opacity-40"
              type="submit"
              :disabled="starting"
            >
              {{ starting ? 'Running…' : 'Run' }}
            </button>
          </div>
        </form>
      </section>

      <p v-if="run.state.runId" class="font-mono text-xs text-mute">
        Last run {{ run.state.runId }} · {{ run.state.status }}
      </p>

      <PageState :loading="loading" :error="error">
        <section v-if="mock">
          <div class="mb-3">
            <h2 class="text-sm font-medium">Today</h2>
            <p class="mt-0.5 text-xs leading-5 text-mute">Sample counters from the mock catalog.</p>
          </div>
          <div class="grid gap-3 md:grid-cols-4 lg:grid-cols-7">
            <MetricCard label="Active Agents" :value="stats.activeAgents" />
            <MetricCard label="Completed Today" :value="stats.completedToday" />
            <MetricCard label="Success Rate" :value="formatPct(stats.successRate, 0)" />
            <MetricCard label="Avg Runtime" :value="formatDuration(stats.averageRuntimeMs)" />
            <MetricCard label="LLM Calls" :value="stats.llmCalls" />
            <MetricCard label="Tokens" :value="stats.tokens.toLocaleString()" />
            <MetricCard label="Cost" :value="formatUsd(stats.costUsd)" />
          </div>
        </section>

        <section>
          <div class="mb-3">
            <h2 class="text-sm font-medium">Runs</h2>
            <p class="mt-0.5 text-xs leading-5 text-mute">Open a row for the decisions, the task plan, and the published board.</p>
          </div>
          <div class="overflow-hidden rounded-xl border border-line bg-panel">
            <table v-if="mock || run.history.length" class="w-full text-left text-sm">
              <thead class="text-xs text-mute">
                <tr>
                  <th class="px-4 py-3 font-medium">Run</th>
                  <th class="px-4 py-3 font-medium">Goal</th>
                  <th class="px-4 py-3 font-medium">Status</th>
                  <th class="px-4 py-3 font-medium">Started</th>
                  <th v-if="mock" class="px-4 py-3 font-medium">Steps</th>
                  <th v-if="mock" class="px-4 py-3 font-medium">Cost</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="r in mock ? MOCK_RUNS : run.history"
                  :key="r.id"
                  class="cursor-pointer border-t border-line hover:bg-raised"
                  @click="router.push(`/runs/${r.id}`)"
                >
                  <td class="px-4 py-3.5 font-mono text-xs">{{ r.id }}</td>
                  <td class="max-w-md truncate px-4 py-3.5">{{ r.goal }}</td>
                  <td class="px-4 py-3.5">{{ r.status }}</td>
                  <td class="px-4 py-3.5 text-mute">{{ formatDateTime(r.startedAt) }}</td>
                  <td v-if="mock" class="px-4 py-3.5">{{ r.steps }}</td>
                  <td v-if="mock" class="px-4 py-3.5">{{ formatUsd(r.costUsd) }}</td>
                </tr>
              </tbody>
            </table>
            <p v-else class="px-4 py-6 text-sm leading-6 text-mute">Submit a goal to start. The run detail page opens when it is created.</p>
          </div>
        </section>
      </PageState>
    </div>
  </div>
</template>
