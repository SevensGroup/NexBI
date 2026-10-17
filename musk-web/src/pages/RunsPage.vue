<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { MOCK_RUNS } from '@/mocks/catalog'
import { formatDateTime, formatDuration, formatUsd } from '@/utils/format'
import PageState from '@/components/common/PageState.vue'
import { isMockMode } from '@/services/api/client'
import { useAgentRunStore } from '@/stores/agentRun'

const router = useRouter()
const run = useAgentRunStore()
const mock = isMockMode()
const rows = computed(() => (mock ? MOCK_RUNS : run.history))
</script>

<template>
  <div class="h-full overflow-auto">
    <div class="mx-auto flex max-w-5xl flex-col gap-8 px-6 py-8">
      <header>
        <h1 class="text-xl font-semibold tracking-tight">Runs</h1>
        <p class="mt-1 text-sm text-mute">
          {{ mock ? 'Replay sample executions.' : 'Live runs from this console. Submit a goal on Dashboard to create one.' }}
        </p>
      </header>
      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">History</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">Open a row for the timeline and the published board.</p>
        </div>
        <PageState :empty="!rows.length" empty-text="No runs yet. Submit a goal from Dashboard or the workspace.">
          <div class="overflow-hidden rounded-xl border border-line bg-panel">
            <table class="w-full text-left text-sm">
              <thead class="text-xs text-mute">
                <tr>
                  <th class="px-4 py-3 font-medium">ID</th>
                  <th class="px-4 py-3 font-medium">Goal</th>
                  <th class="px-4 py-3 font-medium">Status</th>
                  <th class="px-4 py-3 font-medium">Started</th>
                  <th v-if="mock" class="px-4 py-3 font-medium">Duration</th>
                  <th v-if="mock" class="px-4 py-3 font-medium">LLM</th>
                  <th v-if="mock" class="px-4 py-3 font-medium">Tokens</th>
                  <th v-if="mock" class="px-4 py-3 font-medium">Cost</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="r in rows"
                  :key="r.id"
                  class="cursor-pointer border-t border-line hover:bg-raised"
                  @click="router.push(`/runs/${r.id}`)"
                >
                  <td class="px-4 py-3.5 font-mono text-xs">{{ r.id }}</td>
                  <td class="max-w-lg truncate px-4 py-3.5">{{ r.goal }}</td>
                  <td class="px-4 py-3.5">{{ r.status }}</td>
                  <td class="px-4 py-3.5 text-mute">{{ formatDateTime(r.startedAt) }}</td>
                  <td v-if="mock" class="px-4 py-3.5">{{ formatDuration(r.durationMs) }}</td>
                  <td v-if="mock" class="px-4 py-3.5">{{ r.llmCalls }}</td>
                  <td v-if="mock" class="px-4 py-3.5">{{ r.tokens.toLocaleString() }}</td>
                  <td v-if="mock" class="px-4 py-3.5">{{ formatUsd(r.costUsd) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </PageState>
      </section>
    </div>
  </div>
</template>
