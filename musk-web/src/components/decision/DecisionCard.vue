<script setup lang="ts">
import type { Decision } from '@/types'
import ConfidenceBar from './ConfidenceBar.vue'
import RiskBadge from './RiskBadge.vue'
import ReasoningSummary from './ReasoningSummary.vue'
import { formatTime } from '@/utils/format'

defineProps<{ decision: Decision }>()
</script>

<template>
  <article class="rounded border border-line bg-panel p-3 text-sm">
    <div class="flex items-center justify-between gap-2">
      <span class="font-mono text-[11px] text-accent">{{ decision.type }}</span>
      <span class="text-[11px] text-mute">{{ formatTime(decision.timestamp) }} · step {{ decision.step }}</span>
    </div>
    <div class="mt-2 text-xs uppercase tracking-wide text-mute">Goal / Objective</div>
    <div>{{ decision.objective || '—' }}</div>
    <div class="mt-2 text-xs uppercase tracking-wide text-mute">Decision</div>
    <div>{{ decision.summary }}</div>
    <div class="mt-2 text-xs uppercase tracking-wide text-mute">Reasoning summary</div>
    <ReasoningSummary :summary="decision.reason" />
    <div class="mt-3 grid grid-cols-2 gap-3">
      <div>
        <div class="mb-1 text-[11px] text-mute">Confidence</div>
        <ConfidenceBar :value="decision.confidence" />
      </div>
      <div>
        <div class="mb-1 text-[11px] text-mute">Risk</div>
        <RiskBadge :risk="decision.risk" />
      </div>
    </div>
    <div class="mt-2 text-xs text-mute">Next action: <span class="text-ink">{{ decision.nextAction }}</span></div>
    <div v-if="decision.agent" class="mt-1 text-xs text-mute">Agent: <span class="text-ink">{{ decision.agent }}</span></div>
    <pre
      v-if="decision.arguments && Object.keys(decision.arguments).length"
      class="mt-2 overflow-auto whitespace-pre-wrap break-words rounded bg-canvas p-2 font-mono text-[11px] text-mute"
    >{{ JSON.stringify(decision.arguments, null, 2) }}</pre>
  </article>
</template>
