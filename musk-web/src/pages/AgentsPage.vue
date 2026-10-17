<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import AgentGraph from '@/components/agent/AgentGraph.vue'
import AgentCard from '@/components/agent/AgentCard.vue'
import AgentDetailDrawer from '@/components/agent/AgentDetailDrawer.vue'
import AgentStateBadge from '@/components/agent/AgentStateBadge.vue'
import { listAgents } from '@/services/api/agent'
import type { AgentInfo } from '@/types'
import { useAgentRunStore } from '@/stores/agentRun'
import { formatTime } from '@/utils/format'

const router = useRouter()
const run = useAgentRunStore()
const agents = ref<AgentInfo[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const drawer = ref(false)
const selected = ref<AgentInfo | null>(null)

const liveAgents = computed<AgentInfo[]>(() => {
  const status = run.state.status
  const task = run.state.plan?.tasks.find((t) => t.status === 'RUNNING')
  return agents.value.map((a) => {
    if (a.role === 'Coordinator' && run.state.events.length) {
      return { ...a, status, currentTask: task?.title, progress: Math.min(1, run.state.timeline.length / 20), confidence: run.latestDecision?.confidence ?? 0 }
    }
    const collab = run.state.collaborations.filter((c) => c.to === a.name || c.from === a.name).at(-1)
    if (collab) {
      const next: AgentInfo = {
        ...a,
        status: collab.status === 'completed' ? 'COMPLETED' : 'EXECUTING',
        currentTask: collab.task,
        progress: collab.status === 'completed' ? 1 : 0.6,
        confidence: 0.86,
      }
      return next
    }
    return a
  })
})

onMounted(async () => {
  try {
    agents.value = await listAgents()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load agents'
  } finally {
    loading.value = false
  }
})

function open(id: string): void {
  selected.value = liveAgents.value.find((a) => a.id === id) ?? null
  drawer.value = true
}

function goWorkspace(id: string): void {
  if (id === 'advertising-analyst' || id === 'analyst') router.push('/agents/advertising-analyst')
  else router.push(`/agents/${id}`)
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="flex items-end justify-between gap-4 px-6 pb-4 pt-6">
      <header>
        <h1 class="text-xl font-semibold tracking-tight">Agents</h1>
        <p class="mt-1 text-sm text-mute">Coordinator delegates to Analyst, Researcher, and Reviewer.</p>
      </header>
      <button class="h-9 shrink-0 rounded-xl bg-accent px-4 text-sm font-medium text-white" type="button" @click="router.push('/agents/advertising-analyst')">
        Advertising Analyst
      </button>
    </div>
    <div v-if="error" class="px-6 text-sm text-rose-300">{{ error }}</div>
    <div v-else class="grid min-h-0 flex-1 grid-cols-12 gap-4 px-6 pb-6">
      <div class="col-span-8 min-h-0">
        <AgentGraph :agents="liveAgents" @select="open" />
      </div>
      <div class="col-span-4 min-h-0 space-y-3 overflow-auto">
        <AgentCard v-for="a in liveAgents" :key="a.id" :agent="a" @click="goWorkspace" />
        <section class="overflow-hidden rounded-xl border border-line bg-panel">
          <div class="border-b border-line px-4 py-3">
            <h2 class="text-sm font-medium">Collaboration</h2>
            <p class="mt-0.5 text-xs text-mute">Delegations on the current run.</p>
          </div>
          <p v-if="!run.state.collaborations.length" class="px-4 py-4 text-sm text-mute">No delegations yet.</p>
          <div
            v-for="(c, i) in run.state.collaborations"
            :key="i"
            class="flex items-start justify-between gap-4 border-t border-line px-4 py-3 text-sm first:border-t-0"
          >
            <p class="min-w-0">{{ c.from }} → {{ c.to }} · {{ c.task }}</p>
            <p class="shrink-0 text-right text-xs text-mute">{{ c.status }}<br />{{ formatTime(c.timestamp) }}</p>
          </div>
        </section>
        <div class="flex items-center gap-2 px-1 text-xs text-mute">
          <AgentStateBadge :status="run.state.status" live />
          <span>Live run</span>
        </div>
      </div>
    </div>
    <AgentDetailDrawer v-model="drawer" :agent="selected" />
  </div>
</template>
