<script setup lang="ts">
import type { Approval } from '@/types'
import RiskBadge from '@/components/decision/RiskBadge.vue'
import ConfidenceBar from '@/components/decision/ConfidenceBar.vue'

defineProps<{ approval: Approval }>()
const emit = defineEmits<{
  approve: [id: string]
  reject: [id: string]
  open: [id: string]
}>()
</script>

<template>
  <article class="rounded-xl border border-line bg-panel p-4 text-sm">
    <div class="flex items-center justify-between gap-2">
      <button class="text-left font-medium hover:underline" type="button" @click="emit('open', approval.id)">
        {{ approval.action }}
      </button>
      <RiskBadge :risk="approval.risk" />
    </div>
    <p class="mt-1 text-xs text-mute">{{ approval.description }}</p>
    <div class="mt-2 grid grid-cols-2 gap-2 text-xs">
      <div>
        <div class="text-mute">Current</div>
        <div>{{ approval.currentValue || '—' }}</div>
      </div>
      <div>
        <div class="text-mute">Proposed</div>
        <div>{{ approval.proposedValue || '—' }}</div>
      </div>
    </div>
    <div class="mt-2 text-xs">Reason: {{ approval.reason }}</div>
    <div class="mt-2">
      <ConfidenceBar :value="approval.confidence" />
    </div>
    <div v-if="approval.status === 'PENDING'" class="mt-3 flex gap-2">
      <button class="rounded-full border border-line px-3 py-1 text-xs" type="button" @click="emit('reject', approval.id)">Reject</button>
      <button class="rounded-full bg-accent px-3 py-1 text-xs text-white" type="button" @click="emit('approve', approval.id)">Approve</button>
    </div>
    <div v-else class="mt-2 text-[11px] uppercase text-mute">{{ approval.status }}</div>
  </article>
</template>
