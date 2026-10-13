<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import WorkspaceShell from '@/components/workspace/WorkspaceShell.vue'
import TaskGraph from '@/components/task/TaskGraph.vue'
import DecisionCard from '@/components/decision/DecisionCard.vue'
import ReflectionCard from '@/components/reflection/ReflectionCard.vue'
import ToolExecutionCard from '@/components/tool/ToolExecutionCard.vue'
import FinalReport from '@/components/report/FinalReport.vue'
import { CHART_SERIES } from '@/mocks/events'
import { useAgentRunStore } from '@/stores/agentRun'
import { useUiStore } from '@/stores/ui'
import { startRun } from '@/composables/useAgentSession'
import { isMockMode } from '@/services/api/client'
import { DEFAULT_GOAL } from '@/mocks/catalog'

use([CanvasRenderer, LineChart, BarChart, GridComponent, TooltipComponent, LegendComponent])

const run = useAgentRunStore()
const ui = useUiStore()

const dark = {
  backgroundColor: 'transparent',
  textStyle: { color: '#8b929e', fontSize: 11 },
}

const trend = computed(() => ({
  ...dark,
  tooltip: { trigger: 'axis' },
  legend: { data: ['ROAS', 'Spend', 'Revenue'], textStyle: { color: '#8b929e' } },
  grid: { left: 40, right: 16, top: 28, bottom: 24 },
  xAxis: { type: 'category', data: CHART_SERIES.dates, axisLine: { lineStyle: { color: '#2a303b' } } },
  yAxis: { type: 'value', splitLine: { lineStyle: { color: '#2a303b' } } },
  series: [
    { name: 'ROAS', type: 'line', data: CHART_SERIES.roas, smooth: true },
    { name: 'Spend', type: 'line', data: CHART_SERIES.spend, smooth: true },
    { name: 'Revenue', type: 'line', data: CHART_SERIES.revenue, smooth: true },
  ],
}))

const campaign = computed(() => ({
  ...dark,
  tooltip: { trigger: 'axis' },
  grid: { left: 110, right: 16, top: 8, bottom: 24 },
  xAxis: { type: 'value', splitLine: { lineStyle: { color: '#2a303b' } } },
  yAxis: { type: 'category', data: CHART_SERIES.campaigns.map((c) => c.name) },
  series: [{ type: 'bar', data: CHART_SERIES.campaigns.map((c) => c.roas), name: 'ROAS' }],
}))

const creative = computed(() => ({
  ...dark,
  tooltip: { trigger: 'axis' },
  grid: { left: 80, right: 16, top: 8, bottom: 24 },
  xAxis: { type: 'value', splitLine: { lineStyle: { color: '#2a303b' } } },
  yAxis: { type: 'category', data: CHART_SERIES.creatives.map((c) => c.name) },
  series: [{ type: 'bar', data: CHART_SERIES.creatives.map((c) => c.ctr), name: 'CTR %' }],
}))

function onEvidence(id: string): void {
  run.selectEvidence(id)
  const tool = run.state.toolCalls.find((t) => JSON.stringify(t.result || {}).includes(id) || t.id.includes(id))
  const q = run.state.queries.find((s) => JSON.stringify(s).includes(id))
  ui.contextTab = q ? 'evidence' : tool ? 'evidence' : 'evidence'
  const ev = run.state.events.find((e) => JSON.stringify(e.payload).includes(id))
  if (ev) run.selectEvent(ev.id)
}

onMounted(async () => {
  if (isMockMode() && !run.state.events.length) await startRun(DEFAULT_GOAL, 'advertising-analyst')
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <WorkspaceShell title="Advertising Autonomous Analyst" agent-id="advertising-analyst">
      <template #left>
        <div class="space-y-3">
          <div class="text-[11px] uppercase text-mute">Pipeline</div>
          <ol class="space-y-1 text-[11px] text-mute">
            <li>Goal → Understanding → Planning</li>
            <li>Decision → Semantic Query → MCP</li>
            <li>Observation → Campaign / Creative</li>
            <li>Reflection → Reviewer → Report</li>
          </ol>
        </div>
      </template>
    </WorkspaceShell>
    <div class="grid max-h-[42%] grid-cols-12 gap-2 overflow-auto border-t border-line p-3">
      <div class="col-span-5 h-40 rounded border border-line bg-panel p-2">
        <VChart class="h-full" :option="trend" autoresize />
      </div>
      <div class="col-span-4 h-40 rounded border border-line bg-panel p-2">
        <VChart class="h-full" :option="campaign" autoresize />
      </div>
      <div class="col-span-3 h-40 rounded border border-line bg-panel p-2">
        <VChart class="h-full" :option="creative" autoresize />
      </div>
      <div class="col-span-4 min-h-0 overflow-auto">
        <TaskGraph :plan="run.state.plan" />
      </div>
      <div class="col-span-4 space-y-2 overflow-auto">
        <DecisionCard v-if="run.latestDecision" :decision="run.latestDecision" />
        <ReflectionCard v-if="run.latestReflection" :reflection="run.latestReflection" />
        <ToolExecutionCard v-if="run.state.toolCalls.at(-1)" :tool="run.state.toolCalls.at(-1)!" />
      </div>
      <div class="col-span-4 overflow-auto rounded border border-line bg-panel p-3">
        <FinalReport v-if="run.state.report" :report="run.state.report" @evidence="onEvidence" />
        <div v-else class="text-xs text-mute">Report appears after reflection + reviewer.</div>
      </div>
    </div>
  </div>
</template>
