<script setup lang="ts">
import { computed, markRaw } from 'vue'
import { VueFlow, type Edge, type Node, type NodeMouseEvent, type NodeTypesObject } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import type { Plan } from '@/types'
import TaskNode from './TaskNode.vue'
import GraphToolbar from './GraphToolbar.vue'
import { useAgentRunStore } from '@/stores/agentRun'

const props = defineProps<{ plan: Plan | null }>()
const run = useAgentRunStore()
const nodeTypes = { task: markRaw(TaskNode) } as unknown as NodeTypesObject

const LAYOUT: Record<string, { x: number; y: number }> = {
  't-goal': { x: 20, y: 140 },
  't-data': { x: 260, y: 140 },
  't-trend': { x: 500, y: 80 },
  't-camp': { x: 500, y: 220 },
  't-creative': { x: 740, y: 140 },
  't-cause': { x: 980, y: 140 },
  't-review': { x: 1220, y: 140 },
  't-final': { x: 1460, y: 140 },
}

const nodes = computed<Node[]>(() => {
  const tasks = props.plan?.tasks ?? []
  return tasks.map((task, i) => ({
    id: task.id,
    type: 'task',
    position: LAYOUT[task.id] ?? { x: i * 220, y: 120 },
    data: { task, active: run.state.selectedTaskId === task.id },
  }))
})

const edges = computed<Edge[]>(() => {
  const tasks = props.plan?.tasks ?? []
  const out: Edge[] = []
  for (const task of tasks) {
    for (const dep of task.dependencies) {
      out.push({
        id: `${dep}-${task.id}`,
        source: dep,
        target: task.id,
        animated: task.status === 'RUNNING',
      })
    }
  }
  return out
})

function onNodeClick({ node }: NodeMouseEvent): void {
  run.selectTask(node.id)
  const tl = run.state.timeline.find((t) => t.subtitle?.includes(node.id) || t.taskId === node.id)
  if (tl) run.selectEvent(tl.eventId)
}
</script>

<template>
  <div class="relative flex h-full min-h-[280px] flex-col overflow-hidden rounded-xl border border-line">
    <VueFlow
      id="task-dag"
      class="flex-1"
      :nodes="nodes"
      :edges="edges"
      :node-types="nodeTypes"
      fit-view-on-init
      @node-click="onNodeClick"
    >
      <Background pattern-color="#2a303b" :gap="16" />
      <Controls />
      <MiniMap />
      <GraphToolbar />
    </VueFlow>
  </div>
</template>
