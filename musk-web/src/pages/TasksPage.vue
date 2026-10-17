<script setup lang="ts">
import { computed, onMounted } from 'vue'
import TaskGraph from '@/components/task/TaskGraph.vue'
import PlanPanel from '@/components/task/PlanPanel.vue'
import TaskDetailDrawer from '@/components/task/TaskDetailDrawer.vue'
import ExecutionTimeline from '@/components/execution/ExecutionTimeline.vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { isMockMode } from '@/services/api/client'
import { ADS_PLAN } from '@/mocks/events'

const run = useAgentRunStore()
const plan = computed(() => run.state.plan)

onMounted(() => {
  if (isMockMode() && !run.state.plan) {
    run.ingest({
      id: 'seed-plan',
      runId: 'run-ads-01',
      step: 1,
      type: 'plan.created',
      timestamp: new Date().toISOString(),
      payload: { plan: ADS_PLAN },
    })
  }
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <header class="px-6 pb-4 pt-6">
      <h1 class="text-xl font-semibold tracking-tight">Tasks</h1>
      <p class="mt-1 text-sm text-mute">The current plan, the task graph, and the execution timeline.</p>
    </header>
    <div class="grid min-h-0 flex-1 grid-cols-12 gap-4 px-6 pb-6">
      <aside class="col-span-3 overflow-auto rounded-xl border border-line bg-panel p-4">
        <h2 class="text-sm font-medium">Plan</h2>
        <p class="mb-3 mt-0.5 text-xs text-mute">Steps the agent will run.</p>
        <PlanPanel :plan="plan" />
      </aside>
      <section class="col-span-6 min-h-0">
        <TaskGraph :plan="plan" />
      </section>
      <aside class="col-span-3 min-h-0 overflow-hidden rounded-xl border border-line bg-panel">
        <ExecutionTimeline />
      </aside>
    </div>
    <TaskDetailDrawer />
  </div>
</template>
