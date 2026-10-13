<script setup lang="ts">
import { computed } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { useUiStore, type ContextTab } from '@/stores/ui'
import PlanPanel from '@/components/task/PlanPanel.vue'
import EvidenceList from '@/components/evidence/EvidenceList.vue'
import DecisionCard from '@/components/decision/DecisionCard.vue'
import FinalReport from '@/components/report/FinalReport.vue'
import MemoryCard from '@/components/memory/MemoryCard.vue'
import SectionHeader from '@/components/common/SectionHeader.vue'

const run = useAgentRunStore()
const ui = useUiStore()

const tabs: Array<{ id: ContextTab; label: string }> = [
  { id: 'plan', label: 'Plan' },
  { id: 'decision', label: 'Decision' },
  { id: 'evidence', label: 'Evidence' },
  { id: 'memory', label: 'Memory' },
  { id: 'report', label: 'Report' },
]

const used = computed(() => run.state.memories.filter((m) => m.used))
const recent = computed(() => [...run.state.memories].slice(-5).reverse())

function onEvidence(id: string): void {
  run.selectEvidence(id)
  const ev = run.state.events.find((e) => {
    const p = e.payload
    return JSON.stringify(p).includes(id)
  })
  if (ev) run.selectEvent(ev.id)
  ui.contextTab = 'evidence'
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="flex gap-1 border-b border-line px-2 py-1">
      <button
        v-for="t in tabs"
        :key="t.id"
        type="button"
        class="rounded px-2 py-1 text-xs"
        :class="ui.contextTab === t.id ? 'bg-raised text-ink' : 'text-mute'"
        @click="ui.contextTab = t.id"
      >
        {{ t.label }}
      </button>
    </div>
    <div class="min-h-0 flex-1 overflow-auto p-3">
      <PlanPanel v-if="ui.contextTab === 'plan'" :plan="run.state.plan" />
      <div v-else-if="ui.contextTab === 'decision'" class="space-y-2">
        <DecisionCard v-if="run.latestDecision" :decision="run.latestDecision" />
        <div v-else class="text-xs text-mute">No decision yet.</div>
      </div>
      <EvidenceList v-else-if="ui.contextTab === 'evidence'" :items="run.state.evidence" @open="onEvidence" />
      <div v-else-if="ui.contextTab === 'memory'" class="space-y-3">
        <SectionHeader title="Used Memories" />
        <MemoryCard v-for="m in used" :key="m.id" :item="m" />
        <SectionHeader title="Recent Memory" />
        <MemoryCard v-for="m in recent" :key="'r-' + m.id" :item="m" />
        <div v-if="!run.state.memories.length" class="text-xs text-mute">No memory retrieved.</div>
      </div>
      <FinalReport v-else-if="ui.contextTab === 'report' && run.state.report" :report="run.state.report" @evidence="onEvidence" />
      <div v-else class="text-xs text-mute">No report yet.</div>
    </div>
  </div>
</template>
