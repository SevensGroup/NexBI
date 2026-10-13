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
  <div class="h-full overflow-auto p-6">
    <h1 class="text-lg font-medium">Runs</h1>
    <p class="text-sm text-mute">
      {{ mock ? 'Replay sample executions' : 'Live musk-back runs. Submit a goal from Dashboard to create a UUID run.' }}
    </p>
    <PageState :empty="!rows.length" empty-text="No runs yet. Submit a goal from Dashboard or the workspace.">
      <table class="mt-4 w-full text-left text-sm">
        <thead class="text-xs text-mute">
          <tr>
            <th class="py-2">ID</th>
            <th>Goal</th>
            <th>Status</th>
            <th>Started</th>
            <th v-if="mock">Duration</th>
            <th v-if="mock">LLM</th>
            <th v-if="mock">Tokens</th>
            <th v-if="mock">Cost</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in rows"
            :key="r.id"
            class="cursor-pointer border-t border-line hover:bg-raised"
            @click="router.push(`/runs/${r.id}`)"
          >
            <td class="py-2 font-mono text-xs">{{ r.id }}</td>
            <td class="max-w-lg truncate">{{ r.goal }}</td>
            <td>{{ r.status }}</td>
            <td>{{ formatDateTime(r.startedAt) }}</td>
            <td v-if="mock">{{ formatDuration(r.durationMs) }}</td>
            <td v-if="mock">{{ r.llmCalls }}</td>
            <td v-if="mock">{{ r.tokens.toLocaleString() }}</td>
            <td v-if="mock">{{ formatUsd(r.costUsd) }}</td>
          </tr>
        </tbody>
      </table>
    </PageState>
  </div>
</template>
