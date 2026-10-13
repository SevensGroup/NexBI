<script setup lang="ts">
import { computed } from 'vue'
import type { Approval } from '@/types'
import RiskBadge from '@/components/decision/RiskBadge.vue'

const props = defineProps<{ approval: Approval | null; modelValue: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [v: boolean]
  approve: [id: string]
  reject: [id: string]
}>()
const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})
</script>

<template>
  <el-drawer v-model="open" title="Approval" size="440px" append-to-body>
    <div v-if="approval" class="space-y-3 text-sm">
      <div class="flex items-center gap-2">
        <h3 class="font-medium">{{ approval.action }}</h3>
        <RiskBadge :risk="approval.risk" />
      </div>
      <p>{{ approval.description }}</p>
      <div>Current: {{ approval.currentValue }}</div>
      <div>Proposed: {{ approval.proposedValue }}</div>
      <div class="text-mute">{{ approval.reason }}</div>
      <div v-if="approval.status === 'PENDING'" class="flex gap-2">
        <button class="rounded border border-line px-3 py-1" type="button" @click="emit('reject', approval.id)">Reject</button>
        <button class="rounded bg-accent px-3 py-1 text-white" type="button" @click="emit('approve', approval.id)">Approve</button>
      </div>
    </div>
  </el-drawer>
</template>
