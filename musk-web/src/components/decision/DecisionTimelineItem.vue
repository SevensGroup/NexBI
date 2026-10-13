<script setup lang="ts">
import type { Decision } from '@/types'
import RiskBadge from './RiskBadge.vue'
import { formatTime } from '@/utils/format'

defineProps<{ decision: Decision; active?: boolean }>()
const emit = defineEmits<{ select: [id: string] }>()
</script>

<template>
  <button
    type="button"
    class="w-full rounded border px-2 py-1.5 text-left text-xs"
    :class="active ? 'border-accent bg-raised' : 'border-transparent hover:bg-raised'"
    @click="emit('select', decision.id)"
  >
    <div class="flex items-center justify-between gap-2">
      <span class="font-mono text-accent">{{ decision.type }}</span>
      <span class="text-mute">{{ formatTime(decision.timestamp) }}</span>
    </div>
    <div class="mt-0.5 truncate">{{ decision.summary }}</div>
    <div class="mt-1 flex items-center gap-2">
      <RiskBadge :risk="decision.risk" />
      <span class="text-mute">{{ Math.round(decision.confidence * 100) }}%</span>
    </div>
  </button>
</template>
