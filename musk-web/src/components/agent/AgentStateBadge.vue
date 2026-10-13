<script setup lang="ts">
import { computed } from 'vue'
import type { AgentStatus } from '@/types'
import { STATUS_CLASS, STATUS_LABEL } from '@/utils/status'

const props = defineProps<{ status: AgentStatus | 'IDLE'; live?: boolean }>()
const cls = computed(() => (props.status === 'IDLE' ? 'text-mute' : STATUS_CLASS[props.status]))
const label = computed(() => (props.status === 'IDLE' ? 'Idle' : STATUS_LABEL[props.status]))
</script>

<template>
  <span class="inline-flex items-center gap-1.5 text-xs" :class="cls">
    <span class="status-dot" :class="[live && status !== 'IDLE' && status !== 'COMPLETED' && status !== 'FAILED' ? 'pulse-soft' : '', `badge-${status}`]" />
    {{ label }}
  </span>
</template>
