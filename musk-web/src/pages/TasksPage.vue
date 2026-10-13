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
  <div class="grid h-full min-h-0 grid-cols-12">
    <aside class="col-span-3 overflow-auto border-r border-line p-3">
      <h1 class="mb-3 text-sm font-medium">Tasks</h1>
      <PlanPanel :plan="plan" />
    </aside>
    <section class="col-span-6 min-h-0 p-3">
      <TaskGraph :plan="plan" />
    </section>
    <aside class="col-span-3 min-h-0 overflow-hidden border-l border-line">
      <ExecutionTimeline />
    </aside>
    <TaskDetailDrawer />
  </div>
</template>
