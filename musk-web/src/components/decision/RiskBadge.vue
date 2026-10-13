<script setup lang="ts">
import { computed } from 'vue'
import type { RiskLevel } from '@/types'
import { RISK_CLASS } from '@/utils/status'

const props = defineProps<{ risk: RiskLevel | string }>()
const cls = computed(() => RISK_CLASS[(props.risk as RiskLevel) || 'LOW'] || 'risk-medium')
</script>

<template>
  <span class="rounded border border-line px-1.5 py-0.5 text-[11px] font-medium uppercase" :class="cls">{{ risk }}</span>
</template>
