<script setup lang="ts">
import { computed, markRaw } from 'vue'
import { VueFlow, type Edge, type Node, type NodeMouseEvent, type NodeTypesObject } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import type { AgentInfo } from '@/types'
import CollaborationNode from './CollaborationNode.vue'

const props = defineProps<{ agents: AgentInfo[] }>()
const emit = defineEmits<{ select: [id: string] }>()

const nodeTypes = { collab: markRaw(CollaborationNode) } as unknown as NodeTypesObject

const nodes = computed<Node[]>(() => {
  const coord = props.agents.find((a) => a.role === 'Coordinator') ?? props.agents[0]
  const others = props.agents.filter((a) => a.id !== coord?.id)
  const list: Node[] = []
  if (coord) {
    list.push({
      id: coord.id,
      type: 'collab',
      position: { x: 280, y: 40 },
      data: { agent: coord },
    })
  }
  others.forEach((a, i) => {
    list.push({
      id: a.id,
      type: 'collab',
      position: { x: 40 + i * 280, y: 240 },
      data: { agent: a },
    })
  })
  return list
})

const edges = computed<Edge[]>(() => {
  const coord = props.agents.find((a) => a.role === 'Coordinator') ?? props.agents[0]
  if (!coord) return []
  return props.agents
    .filter((a) => a.id !== coord.id)
    .map((a) => ({
      id: `${coord.id}-${a.id}`,
      source: coord.id,
      target: a.id,
      label: 'delegate',
      animated: a.status !== 'IDLE' && a.status !== 'COMPLETED',
    }))
})

function onNodeClick({ node }: NodeMouseEvent): void {
  emit('select', node.id)
}
</script>

<template>
  <div class="h-full min-h-[360px] rounded border border-line">
    <VueFlow :nodes="nodes" :edges="edges" :node-types="nodeTypes" fit-view-on-init @node-click="onNodeClick">
      <Background pattern-color="#2a303b" :gap="18" />
      <Controls />
      <MiniMap pannable zoomable />
    </VueFlow>
  </div>
</template>
