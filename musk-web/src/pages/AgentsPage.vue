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
  <div class="flex h-full min-h-0 flex-col p-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <h1 class="text-lg font-medium">Agent Center</h1>
        <p class="text-xs text-mute">Coordinator delegates to Analyst, Researcher, Reviewer</p>
      </div>
      <button class="rounded bg-accent px-3 py-1.5 text-xs text-white" type="button" @click="router.push('/agents/advertising-analyst')">
        Advertising Analyst
      </button>
    </div>
    <div v-if="error" class="text-sm text-rose-400">{{ error }}</div>
    <div v-else class="grid min-h-0 flex-1 grid-cols-12 gap-3">
      <div class="col-span-8 min-h-0">
        <AgentGraph :agents="liveAgents" @select="open" />
      </div>
      <div class="col-span-4 min-h-0 space-y-2 overflow-auto">
        <AgentCard v-for="a in liveAgents" :key="a.id" :agent="a" @click="goWorkspace" />
        <div class="rounded border border-line bg-panel p-3">
          <div class="mb-2 text-[11px] uppercase text-mute">Collaboration Timeline</div>
          <div v-if="!run.state.collaborations.length" class="text-xs text-mute">No delegations yet.</div>
          <ol class="space-y-1 text-xs">
            <li v-for="(c, i) in run.state.collaborations" :key="i" class="flex justify-between gap-2">
              <span>{{ c.from }} → {{ c.to }} · {{ c.task }} · {{ c.status }}</span>
              <span class="text-mute">{{ formatTime(c.timestamp) }}</span>
            </li>
          </ol>
        </div>
        <div class="flex items-center gap-2 text-xs text-mute">
          <AgentStateBadge :status="run.state.status" live />
          <span>live run</span>
        </div>
      </div>
    </div>
    <AgentDetailDrawer v-model="drawer" :agent="selected" />
  </div>
</template>
