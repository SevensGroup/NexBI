<script setup lang="ts">
import type { Approval } from '@/types'
import ApprovalCard from './ApprovalCard.vue'
import PageState from '@/components/common/PageState.vue'

defineProps<{ items: Approval[]; loading?: boolean; error?: string | null }>()
const emit = defineEmits<{
  approve: [id: string]
  reject: [id: string]
  open: [id: string]
}>()
</script>

<template>
  <PageState :loading="loading" :error="error" :empty="!items.length" empty-text="No approvals.">
    <div class="grid gap-3 md:grid-cols-2">
      <ApprovalCard
        v-for="a in items"
        :key="a.id"
        :approval="a"
        @approve="emit('approve', $event)"
        @reject="emit('reject', $event)"
        @open="emit('open', $event)"
      />
    </div>
  </PageState>
</template>
